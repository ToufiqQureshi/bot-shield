package policy

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
)

// Validation errors are returned as-is (not wrapped in a request-facing
// generic message) because pkg/rules' Create/Update handler is expected
// to surface them to the dashboard operator who wrote the rule.
var (
	ErrNoConditions             = errors.New("policy: rule must have at least one condition")
	ErrTooManyConditions        = errors.New("policy: rule has too many conditions")
	ErrUnknownField             = errors.New("policy: unknown condition field")
	ErrUnknownOperator          = errors.New("policy: unknown condition operator")
	ErrOperatorNotValidForField = errors.New("policy: operator is not valid for this field")
	ErrUnknownAction            = errors.New("policy: unknown action")
	ErrEmptyValue               = errors.New("policy: condition value must not be empty")
	ErrValueTooLong             = errors.New("policy: condition value is too long")
	ErrBadRegex                 = errors.New("policy: MATCHES value is not a valid regular expression")
	ErrBadNumber                = errors.New("policy: Threat Score condition requires a numeric value")
	ErrBadCIDR                  = errors.New("policy: IP Range value is not a valid CIDR")
	ErrUAOnlyAllow              = errors.New("policy: a rule cannot allow traffic based only on User-Agent — it would skip scoring")
	ErrUAOnlyBlock              = errors.New("policy: a rule cannot block traffic based only on User-Agent")
	ErrDeceiveNotStricter       = errors.New("policy: a DECEIVE rule needs a Threat Score floor above the account's block threshold")
	ErrInvalidRuleValue         = errors.New("policy: invalid condition value")
)

const (
	maxRuleConditions    = 16
	maxConditionValueLen = 512
)

// allowedOperators is the closed set of operators that mean something
// for each field, matching exactly what Condition.matches actually does
// in policy.go (FieldScore always numeric-compares, FieldCIDR always
// CIDR-contains regardless of operator). Anything outside this set would
// be accepted by Evaluate as a safe non-match, but ValidateRule rejects
// it instead of letting the dashboard save a rule that can never fire.
var allowedOperators = map[Field][]Operator{
	FieldJA4:           {OpEquals, OpContains, OpMatches},
	FieldIP:            {OpEquals, OpContains, OpMatches},
	FieldUserAgent:     {OpEquals, OpContains, OpMatches},
	FieldPath:          {OpEquals, OpContains, OpMatches},
	FieldMethod:        {OpEquals},
	FieldScore:         {OpEquals, OpGT, OpLT, OpGTE, OpLTE},
	FieldCIDR:          {OpEquals},
	FieldVerifiedAgent: {OpEquals},
	FieldSignal:        {OpEquals},
	FieldRequestClass:  {OpEquals},
	FieldAllowlisted:   {OpEquals},
}

func validSignal(name string) bool {
	return slices.Contains(signals.FeatureNames(), name)
}

// ValidateRule is the single place a candidate rule is approved, called
// both by the dashboard-facing Create/Update handler in pkg/rules and by
// any future migration path — see docs/BACKEND_IMPLEMENTATION_PLAN.md
// Phase 1 guardrails. blockThreshold is the account's configured block
// score (pkg/settings Protection.BlockThreshold); pass 0 when validating
// a rule that carries no DECEIVE action, since it is only consulted for
// that check.
func ValidateRule(r Rule, blockThreshold int) error {
	if len(r.Conditions) == 0 {
		return ErrNoConditions
	}
	if len(r.Conditions) > maxRuleConditions {
		return ErrTooManyConditions
	}
	if !knownActions[r.Action] {
		return fmt.Errorf("%w: %q", ErrUnknownAction, r.Action)
	}
	for _, c := range r.Conditions {
		if err := validateCondition(c); err != nil {
			return err
		}
	}

	// A single condition on User-Agent alone must never grant PASS: a
	// claimed browser string is the cheapest thing to forge, and an
	// allow rule built only from it would let a scraper skip every other
	// check just by copying a real browser's header (CLAUDE.md Section
	// 17). Multi-condition rules are fine — User-Agent plus something
	// server-verified (JA4, IP range) is no longer a bare claim.
	if r.Action == ActionAllow && len(r.Conditions) == 1 && r.Conditions[0].Field == FieldUserAgent {
		return ErrUAOnlyAllow
	}
	if r.Action == ActionBlock && len(r.Conditions) == 1 && r.Conditions[0].Field == FieldUserAgent {
		return ErrUAOnlyBlock
	}

	if r.Action == ActionDeceive {
		if !deceiveFloorAboveBlock(r.Conditions, blockThreshold) {
			return ErrDeceiveNotStricter
		}
	}

	return nil
}

func validateCondition(c Condition) error {
	allowed, ok := allowedOperators[c.Field]
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownField, c.Field)
	}
	if !knownOperators[c.Operator] {
		return fmt.Errorf("%w: %q", ErrUnknownOperator, c.Operator)
	}
	if !slices.Contains(allowed, c.Operator) {
		return fmt.Errorf("%w: %q on %q", ErrOperatorNotValidForField, c.Operator, c.Field)
	}
	if strings.TrimSpace(c.Value) == "" {
		return ErrEmptyValue
	}
	if len(c.Value) > maxConditionValueLen {
		return ErrValueTooLong
	}
	if c.Operator == OpMatches {
		if _, err := regexp.Compile(c.Value); err != nil {
			return fmt.Errorf("%w: %w", ErrBadRegex, err)
		}
	}
	if c.Field == FieldScore {
		if _, err := strconv.Atoi(strings.TrimSpace(c.Value)); err != nil {
			return ErrBadNumber
		}
	}
	if c.Field == FieldCIDR {
		if _, _, err := net.ParseCIDR(c.Value); err != nil {
			return fmt.Errorf("%w: %w", ErrBadCIDR, err)
		}
	}
	if c.Field == FieldRequestClass && !validClass(c.Value) {
		return fmt.Errorf("%w: invalid request class", ErrInvalidRuleValue)
	}
	if c.Field == FieldAllowlisted && c.Value != "true" {
		return fmt.Errorf("%w: allowlist value must be true", ErrInvalidRuleValue)
	}
	if c.Field == FieldVerifiedAgent && c.Value != "search" && c.Value != "monitor" {
		return fmt.Errorf("%w: unknown verified agent", ErrInvalidRuleValue)
	}
	if c.Field == FieldSignal && !validSignal(c.Value) {
		return fmt.Errorf("%w: unknown signal", ErrInvalidRuleValue)
	}
	if c.Field == FieldMethod {
		switch c.Value {
		case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS":
		default:
			return fmt.Errorf("%w: invalid request method", ErrInvalidRuleValue)
		}
	}
	if c.Field == FieldPath && c.Operator == OpEquals {
		normalized, ok := NormalizePath(c.Value)
		if !ok || normalized != c.Value {
			return fmt.Errorf("%w: path must be canonical", ErrInvalidRuleValue)
		}
	}
	return nil
}

// deceiveFloorAboveBlock reports whether the rule has a Threat Score
// lower-bound condition (GT/GTE) whose value is strictly greater than
// blockThreshold. Showing a visitor fabricated data is a worse outcome
// than an honest block for anyone not already past the block bar, so a
// DECEIVE rule must require more evidence than a BLOCK already would
// (CLAUDE.md Phase 1 guardrails).
func deceiveFloorAboveBlock(conditions []Condition, blockThreshold int) bool {
	for _, c := range conditions {
		if c.Field != FieldScore {
			continue
		}
		if c.Operator != OpGT && c.Operator != OpGTE {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(c.Value))
		if err != nil {
			continue
		}
		if n > blockThreshold {
			return true
		}
	}
	return false
}
