package policy

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Validation errors are returned as-is (not wrapped in a request-facing
// generic message) because pkg/rules' Create/Update handler is expected
// to surface them to the dashboard operator who wrote the rule.
var (
	ErrNoConditions       = errors.New("policy: rule must have at least one condition")
	ErrUnknownField       = errors.New("policy: unknown condition field")
	ErrUnknownOperator    = errors.New("policy: unknown condition operator")
	ErrUnknownAction      = errors.New("policy: unknown action")
	ErrEmptyValue         = errors.New("policy: condition value must not be empty")
	ErrBadRegex           = errors.New("policy: MATCHES value is not a valid regular expression")
	ErrBadNumber          = errors.New("policy: numeric operator requires a numeric value")
	ErrUAOnlyAllow        = errors.New("policy: a rule cannot allow traffic based only on User-Agent — it would skip scoring")
	ErrDeceiveNotStricter = errors.New("policy: a DECEIVE rule needs a Threat Score floor above the account's block threshold")
)

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

	if r.Action == ActionDeceive {
		if !deceiveFloorAboveBlock(r.Conditions, blockThreshold) {
			return ErrDeceiveNotStricter
		}
	}

	return nil
}

func validateCondition(c Condition) error {
	if !knownFields[c.Field] {
		return fmt.Errorf("%w: %q", ErrUnknownField, c.Field)
	}
	if !knownOperators[c.Operator] {
		return fmt.Errorf("%w: %q", ErrUnknownOperator, c.Operator)
	}
	if strings.TrimSpace(c.Value) == "" {
		return ErrEmptyValue
	}
	if c.Operator == OpMatches {
		if _, err := regexp.Compile(c.Value); err != nil {
			return fmt.Errorf("%w: %v", ErrBadRegex, err)
		}
	}
	if isNumericOperator(c.Operator) {
		if _, err := strconv.Atoi(strings.TrimSpace(c.Value)); err != nil {
			return ErrBadNumber
		}
		if c.Field != FieldScore {
			// A numeric comparison only means something against Threat
			// Score today; reject it early rather than silently saving a
			// rule that Evaluate will always report as not matching.
			return fmt.Errorf("%w: operator %q is only valid for %q", ErrUnknownOperator, c.Operator, FieldScore)
		}
	}
	return nil
}

func isNumericOperator(op Operator) bool {
	switch op {
	case OpGT, OpLT, OpGTE, OpLTE:
		return true
	default:
		return false
	}
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
