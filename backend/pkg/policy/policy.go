// Package policy turns an account's dashboard-authored mitigation rules
// into a match against one request. It is pure and storage-agnostic: no
// database, no HTTP, no tenant lookup. Evaluate never errors and never
// panics, because it is meant to run in the request path — an
// unrecognised field, operator, or a rule with no conditions all resolve
// to "does not match" rather than an error, so a malformed or
// not-yet-supported rule can never itself become a source of blocked
// traffic (CLAUDE.md Section 2: unknown is neutral, not malicious).
//
// This package only decides what a policy would do. Nothing here
// enforces anything — wiring its answer into core.Guard's actual
// allow/challenge/block decision is a separate, later change (see
// docs/BACKEND_IMPLEMENTATION_PLAN.md Phase 1).
package policy

import (
	"net"
	"regexp"
	"strconv"
	"strings"
)

// Field is one fact about a request a condition can be written against.
// This is a closed set on purpose: a dashboard rule referencing a field
// outside this list is not an error, it simply never matches (see
// Condition.matches), so adding a new field here is the only way it
// starts taking effect.
type Field string

const (
	FieldJA4       Field = "JA4 Fingerprint"
	FieldScore     Field = "Threat Score"
	FieldIP        Field = "IP Address"
	FieldUserAgent Field = "User-Agent"
	FieldPath      Field = "Request Path"
	FieldMethod    Field = "Request Method"
	FieldCIDR      Field = "IP Range"
)

// knownFields is every field ValidateRule accepts. The dashboard's rule
// builder (dashboard/src/pages/MitigationRules.tsx) also lists ASN, Geo
// and TLS Version, but nothing in RequestFacts computes those yet, so
// they are deliberately left out here — ValidateRule rejects a rule
// that uses one rather than silently saving a rule the dashboard would
// then claim is active but that can never fire.
var knownFields = map[Field]bool{
	FieldJA4:       true,
	FieldScore:     true,
	FieldIP:        true,
	FieldUserAgent: true,
	FieldPath:      true,
	FieldMethod:    true,
	FieldCIDR:      true,
}

// notYetSupportedFields are recognised by the product (the dashboard
// offers them) but have no data source in RequestFacts yet. Evaluate
// treats a condition against one of these as defense-in-depth: it must
// never match rather than panic or match everything, in case a rule
// saved before ValidateRule rejected them is still stored somewhere.
var notYetSupportedFields = map[Field]bool{
	"ASN":         true,
	"Geo":         true,
	"TLS Version": true,
}

// Operator is the comparison a condition applies between the request's
// field value and the rule's configured value. Values match the
// dashboard's wire format exactly (dashboard/src/pages/MitigationRules.tsx
// `operators`) so a rule round-trips without translation.
type Operator string

const (
	OpEquals   Operator = "EQUALS"
	OpContains Operator = "CONTAINS"
	OpMatches  Operator = "MATCHES"
	OpGT       Operator = ">"
	OpLT       Operator = "<"
	OpGTE      Operator = ">="
	OpLTE      Operator = "<="
)

var knownOperators = map[Operator]bool{
	OpEquals: true, OpContains: true, OpMatches: true,
	OpGT: true, OpLT: true, OpGTE: true, OpLTE: true,
}

// Action is what a matched rule recommends. Nothing in this package
// carries it out — see the package doc comment.
type Action string

const (
	ActionAllow     Action = "PASS"
	ActionChallenge Action = "CHALLENGE"
	ActionBlock     Action = "BLOCK"
	ActionDeceive   Action = "DECEIVE"
	ActionLog       Action = "LOG"
)

var knownActions = map[Action]bool{
	ActionAllow: true, ActionChallenge: true, ActionBlock: true,
	ActionDeceive: true, ActionLog: true,
}

// Condition is one clause of a rule, e.g. {Request Path, CONTAINS, /admin}.
type Condition struct {
	Field    Field
	Operator Operator
	Value    string
}

// Rule is one account-authored mitigation rule, already validated and
// ordered. Conditions are ANDed together — a rule matches only when
// every condition matches.
type Rule struct {
	ID         string
	Name       string
	Conditions []Condition
	Action     Action
	Enabled    bool
}

// Policy is one account's ordered rule set. The zero value (nil Rules)
// is the safe default for a tenant that has never configured one: it
// evaluates to no match, every time, for every request.
type Policy struct {
	OwnerUserID string
	Version     int
	Rules       []Rule
}

// Facts is the subset of a request Evaluate can see. It intentionally
// mirrors signals.RequestFacts plus the two fields (Method, Score) that
// package doesn't carry — kept separate so this package never imports
// pkg/signals and pkg/signals never has to import this package.
type Facts struct {
	IP     string
	JA4    string
	UA     string
	Path   string
	Method string
	// Score is the combined signal score for this request (see
	// signals.Evaluate). Callers compute it first and pass it in, since
	// only pkg/signals owns how it's derived.
	Score int
}

// MatchResult is what a policy would do about one request. Matched is
// false when no rule fired, in which case RuleID/RuleName/Action are
// zero values and the caller's existing decision is unaffected.
type MatchResult struct {
	Matched  bool
	RuleID   string
	RuleName string
	Action   Action
}

// Evaluate walks the policy's rules in order and returns the first one
// that matches. An empty or nil policy, or one whose rules all fail to
// match, returns a zero MatchResult — the same outcome as having no
// policy at all, which is what keeps this safe to call for every tenant
// unconditionally.
func Evaluate(p *Policy, f Facts) MatchResult {
	if p == nil {
		return MatchResult{}
	}
	for _, r := range p.Rules {
		if !r.Enabled {
			continue
		}
		if ruleMatches(r, f) {
			return MatchResult{Matched: true, RuleID: r.ID, RuleName: r.Name, Action: r.Action}
		}
	}
	return MatchResult{}
}

// ruleMatches reports whether every condition in r matches f. A rule
// with zero conditions never matches — an unscoped rule that fires on
// every request is a footgun (an empty BLOCK rule would block all
// traffic), not a useful "match everything" shorthand, so it is treated
// the same as a disabled rule.
func ruleMatches(r Rule, f Facts) bool {
	if len(r.Conditions) == 0 {
		return false
	}
	for _, c := range r.Conditions {
		if !c.matches(f) {
			return false
		}
	}
	return true
}

func (c Condition) matches(f Facts) bool {
	if notYetSupportedFields[c.Field] {
		return false
	}
	switch c.Field {
	case FieldJA4:
		return stringMatch(c.Operator, f.JA4, c.Value)
	case FieldIP:
		return stringMatch(c.Operator, f.IP, c.Value)
	case FieldCIDR:
		return cidrMatch(f.IP, c.Value)
	case FieldUserAgent:
		return stringMatch(c.Operator, f.UA, c.Value)
	case FieldPath:
		return stringMatch(c.Operator, f.Path, c.Value)
	case FieldMethod:
		return stringMatch(c.Operator, f.Method, c.Value)
	case FieldScore:
		return numberMatch(c.Operator, f.Score, c.Value)
	default:
		return false
	}
}

func stringMatch(op Operator, actual, want string) bool {
	switch op {
	case OpEquals:
		return actual == want
	case OpContains:
		return want != "" && strings.Contains(actual, want)
	case OpMatches:
		re, err := regexp.Compile(want)
		if err != nil {
			// A malformed pattern is a validation failure, not a runtime
			// one: Validate rejects it before a rule can be saved. If one
			// somehow reaches here anyway, it must not match rather than
			// panic on live traffic.
			return false
		}
		return re.MatchString(actual)
	default:
		// Numeric operators against a string field are meaningless.
		return false
	}
}

func numberMatch(op Operator, actual int, wantStr string) bool {
	want, err := strconv.Atoi(strings.TrimSpace(wantStr))
	if err != nil {
		return false
	}
	switch op {
	case OpEquals:
		return actual == want
	case OpGT:
		return actual > want
	case OpLT:
		return actual < want
	case OpGTE:
		return actual >= want
	case OpLTE:
		return actual <= want
	default:
		return false
	}
}

func cidrMatch(ipStr, cidrStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	_, network, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return false
	}
	return network.Contains(ip)
}
