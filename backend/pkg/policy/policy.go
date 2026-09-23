// Package policy turns an account's dashboard-authored mitigation rules
// into a match against one request. It is pure and storage-agnostic: no
// database, no HTTP, no tenant lookup. Evaluate never errors and never
// panics, because it is meant to run in the request path — an
// unrecognised field, operator, or a rule with no conditions all resolve
// to "does not match" rather than an error, so a malformed or
// not-yet-supported rule can never itself become a source of blocked
// traffic (CLAUDE.md Section 2: unknown is neutral, not malicious).
//
// This package only matches conditions. core.Guard applies a selected
// action only for an activated tenant revision in enforcement mode.
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
	FieldJA4           Field = "JA4 Fingerprint"
	FieldScore         Field = "Threat Score"
	FieldIP            Field = "IP Address"
	FieldUserAgent     Field = "User-Agent"
	FieldPath          Field = "Request Path"
	FieldMethod        Field = "Request Method"
	FieldCIDR          Field = "IP Range"
	FieldVerifiedAgent Field = "Verified Agent"
	FieldSignal        Field = "Signal"
	FieldRequestClass  Field = "Request Class"
	FieldAllowlisted   Field = "Tenant Allowlist"
)

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
	ActionRateLimit Action = "RATE_LIMIT"
)

var knownActions = map[Action]bool{
	ActionAllow: true, ActionChallenge: true, ActionBlock: true,
	ActionDeceive: true, ActionLog: true,
	ActionRateLimit: true,
}

// Condition is one clause of a rule, e.g. {Request Path, CONTAINS, /admin}.
type Condition struct {
	Field    Field    `json:"field"`
	Operator Operator `json:"operator"`
	Value    string   `json:"value"`
	compiled *regexp.Regexp
}

// Rule is one operator-authored mitigation rule, already validated and
// ordered. Conditions are ANDed together — a rule matches only when
// every condition matches.
type Rule struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Conditions []Condition `json:"conditions"`
	Action     Action      `json:"action"`
	Enabled    bool        `json:"enabled"`
}

// Policy is one tenant's ordered rule set. The zero value (nil Rules)
// is the safe default for a tenant that has never configured one: it
// evaluates to no match, every time, for every request.
type Policy struct {
	TenantID       string
	OwnerUserID    string
	Version        int
	Mode           string // shadow or enforce; empty is shadow
	Rules          []Rule
	Allowlist      []*net.IPNet
	ChallengeTheme string
	BlockMessage   string
}

func (p *Policy) Allowlisted(ip string) bool {
	if p == nil {
		return false
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, cidr := range p.Allowlist {
		if cidr.Contains(parsed) {
			return true
		}
	}
	return false
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
	Score         int
	VerifiedAgent string
	Signals       []string
	Class         string
	Allowlisted   bool
}

// MatchResult is what a policy would do about one request. Matched is
// false when no rule fired, in which case RuleID/RuleName/Action are
// zero values and the caller's existing decision is unaffected.
type MatchResult struct {
	Matched  bool   `json:"matched"`
	RuleID   string `json:"ruleId,omitempty"`
	RuleName string `json:"ruleName,omitempty"`
	Action   Action `json:"action,omitempty"`
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
		if !r.Enabled || !knownActions[r.Action] {
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
	if len(r.Conditions) == 0 || len(r.Conditions) > maxRuleConditions {
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
	if len(c.Value) > maxConditionValueLen {
		return false
	}
	if notYetSupportedFields[c.Field] {
		return false
	}
	switch c.Field {
	case FieldJA4:
		return c.stringMatch(f.JA4)
	case FieldIP:
		return c.stringMatch(f.IP)
	case FieldCIDR:
		if c.Operator != OpEquals {
			return false
		}
		return cidrMatch(f.IP, c.Value)
	case FieldUserAgent:
		return c.stringMatch(f.UA)
	case FieldPath:
		return c.stringMatch(f.Path)
	case FieldMethod:
		return c.stringMatch(f.Method)
	case FieldScore:
		return numberMatch(c.Operator, f.Score, c.Value)
	case FieldVerifiedAgent:
		return c.Operator == OpEquals && f.VerifiedAgent != "" && strings.EqualFold(f.VerifiedAgent, c.Value)
	case FieldSignal:
		if c.Operator != OpEquals {
			return false
		}
		for _, signal := range f.Signals {
			if signal == c.Value {
				return true
			}
		}
		return false
	case FieldRequestClass:
		return c.Operator == OpEquals && f.Class == c.Value
	case FieldAllowlisted:
		return c.Operator == OpEquals && f.Allowlisted && c.Value == "true"
	default:
		return false
	}
}

func (c Condition) stringMatch(actual string) bool {
	if c.Operator == OpMatches && c.compiled != nil {
		return c.compiled.MatchString(actual)
	}
	return stringMatch(c.Operator, actual, c.Value)
}

// Compile copies a validated policy and compiles regular expressions once.
// The result is immutable and safe to share across concurrent requests.
func Compile(p *Policy) (*Policy, error) {
	if p == nil {
		return nil, nil
	}
	copyPolicy := *p
	copyPolicy.Rules = make([]Rule, len(p.Rules))
	for i, rule := range p.Rules {
		copyPolicy.Rules[i] = rule
		copyPolicy.Rules[i].Conditions = make([]Condition, len(rule.Conditions))
		copy(copyPolicy.Rules[i].Conditions, rule.Conditions)
		for j := range copyPolicy.Rules[i].Conditions {
			c := &copyPolicy.Rules[i].Conditions[j]
			if c.Operator == OpMatches {
				re, err := regexp.Compile(c.Value)
				if err != nil {
					return nil, err
				}
				c.compiled = re
			}
		}
	}
	return &copyPolicy, nil
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
