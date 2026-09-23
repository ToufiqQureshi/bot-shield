package policy

import (
	"strings"
	"testing"
)

func TestEvaluate_NilPolicyIsNoOp(t *testing.T) {
	got := Evaluate(nil, Facts{Path: "/admin", Score: 999})
	if got.Matched {
		t.Fatalf("nil policy matched: %+v", got)
	}
}

func TestEvaluate_EmptyRulesIsNoOp(t *testing.T) {
	p := &Policy{Rules: nil}
	got := Evaluate(p, Facts{Path: "/admin", Score: 999})
	if got.Matched {
		t.Fatalf("empty policy matched: %+v", got)
	}
}

// A rule with zero conditions must behave exactly like no policy at
// all — it must never become an implicit match-everything rule.
func TestEvaluate_ZeroConditionRuleNeverMatches(t *testing.T) {
	p := &Policy{Rules: []Rule{
		{ID: "r1", Name: "empty", Conditions: nil, Action: ActionBlock, Enabled: true},
	}}
	got := Evaluate(p, Facts{Path: "/anything"})
	if got.Matched {
		t.Fatalf("zero-condition rule matched: %+v", got)
	}
}

func TestEvaluate_DisabledRuleNeverMatches(t *testing.T) {
	p := &Policy{Rules: []Rule{
		{ID: "r1", Name: "disabled", Enabled: false, Action: ActionBlock,
			Conditions: []Condition{{Field: FieldPath, Operator: OpEquals, Value: "/admin"}}},
	}}
	got := Evaluate(p, Facts{Path: "/admin"})
	if got.Matched {
		t.Fatalf("disabled rule matched: %+v", got)
	}
}

func TestEvaluate_FirstMatchWins(t *testing.T) {
	p := &Policy{Rules: []Rule{
		{ID: "r1", Name: "first", Enabled: true, Action: ActionChallenge,
			Conditions: []Condition{{Field: FieldPath, Operator: OpEquals, Value: "/login"}}},
		{ID: "r2", Name: "second", Enabled: true, Action: ActionBlock,
			Conditions: []Condition{{Field: FieldPath, Operator: OpEquals, Value: "/login"}}},
	}}
	got := Evaluate(p, Facts{Path: "/login"})
	if !got.Matched || got.RuleID != "r1" || got.Action != ActionChallenge {
		t.Fatalf("expected r1 to win, got %+v", got)
	}
}

func TestEvaluate_AllConditionsMustMatchAND(t *testing.T) {
	p := &Policy{Rules: []Rule{
		{ID: "r1", Name: "and-rule", Enabled: true, Action: ActionBlock,
			Conditions: []Condition{
				{Field: FieldPath, Operator: OpEquals, Value: "/login"},
				{Field: FieldMethod, Operator: OpEquals, Value: "POST"},
			}},
	}}
	if got := Evaluate(p, Facts{Path: "/login", Method: "GET"}); got.Matched {
		t.Fatalf("partial match should not fire: %+v", got)
	}
	if got := Evaluate(p, Facts{Path: "/login", Method: "POST"}); !got.Matched {
		t.Fatalf("full match should fire")
	}
}

func TestEvaluate_UnknownFieldNeverMatches(t *testing.T) {
	p := &Policy{Rules: []Rule{
		{ID: "r1", Enabled: true, Action: ActionBlock,
			Conditions: []Condition{{Field: Field("nonsense"), Operator: OpEquals, Value: "x"}}},
	}}
	got := Evaluate(p, Facts{})
	if got.Matched {
		t.Fatalf("unknown field matched: %+v", got)
	}
}

func TestEvaluate_UnsupportedButKnownFieldNeverMatches(t *testing.T) {
	p := &Policy{Rules: []Rule{
		{ID: "r1", Enabled: true, Action: ActionBlock,
			Conditions: []Condition{{Field: "Geo", Operator: OpEquals, Value: "RU"}}},
	}}
	got := Evaluate(p, Facts{})
	if got.Matched {
		t.Fatalf("unsupported field matched: %+v", got)
	}
}

func TestEvaluate_UnknownOperatorNeverMatches(t *testing.T) {
	p := &Policy{Rules: []Rule{
		{ID: "r1", Enabled: true, Action: ActionBlock,
			Conditions: []Condition{{Field: FieldPath, Operator: Operator("WAT"), Value: "/x"}}},
	}}
	got := Evaluate(p, Facts{Path: "/x"})
	if got.Matched {
		t.Fatalf("unknown operator matched: %+v", got)
	}
}

func TestEvaluate_CIDRWithUnknownOperatorNeverMatches(t *testing.T) {
	p := &Policy{Rules: []Rule{{ID: "r1", Enabled: true, Action: ActionBlock,
		Conditions: []Condition{{Field: FieldCIDR, Operator: "INVALID", Value: "203.0.113.0/24"}}}}}
	if got := Evaluate(p, Facts{IP: "203.0.113.5"}); got.Matched {
		t.Fatalf("invalid CIDR operator matched: %+v", got)
	}
}

func TestEvaluate_UnknownActionNeverMatches(t *testing.T) {
	p := &Policy{Rules: []Rule{{ID: "r1", Enabled: true, Action: "INVALID",
		Conditions: []Condition{{Field: FieldPath, Operator: OpEquals, Value: "/"}}}}}
	if got := Evaluate(p, Facts{Path: "/"}); got.Matched {
		t.Fatalf("unknown action matched: %+v", got)
	}
}

func TestEvaluate_UnvalidatedOversizedRuleNeverMatches(t *testing.T) {
	p := &Policy{Rules: []Rule{{ID: "r1", Enabled: true, Action: ActionBlock,
		Conditions: []Condition{{Field: FieldPath, Operator: OpMatches, Value: strings.Repeat("a", maxConditionValueLen+1)}}}}}
	if got := Evaluate(p, Facts{Path: strings.Repeat("a", maxConditionValueLen+1)}); got.Matched {
		t.Fatalf("oversized regex matched: %+v", got)
	}
}

func TestEvaluate_MalformedRegexNeverMatchesOrPanics(t *testing.T) {
	p := &Policy{Rules: []Rule{
		{ID: "r1", Enabled: true, Action: ActionBlock,
			Conditions: []Condition{{Field: FieldPath, Operator: OpMatches, Value: "(unterminated"}}},
	}}
	got := Evaluate(p, Facts{Path: "/anything"})
	if got.Matched {
		t.Fatalf("malformed regex matched: %+v", got)
	}
}

func TestEvaluate_ScoreBoundary(t *testing.T) {
	p := &Policy{Rules: []Rule{
		{ID: "r1", Enabled: true, Action: ActionChallenge,
			Conditions: []Condition{{Field: FieldScore, Operator: OpGTE, Value: "50"}}},
	}}
	if got := Evaluate(p, Facts{Score: 49}); got.Matched {
		t.Fatalf("49 should not match >= 50")
	}
	if got := Evaluate(p, Facts{Score: 50}); !got.Matched {
		t.Fatalf("50 should match >= 50")
	}
}

func TestEvaluate_ScoreNonNumericValueNeverMatches(t *testing.T) {
	p := &Policy{Rules: []Rule{
		{ID: "r1", Enabled: true, Action: ActionChallenge,
			Conditions: []Condition{{Field: FieldScore, Operator: OpGT, Value: "not-a-number"}}},
	}}
	got := Evaluate(p, Facts{Score: 100})
	if got.Matched {
		t.Fatalf("non-numeric score value matched: %+v", got)
	}
}

func TestEvaluate_CIDRMatch(t *testing.T) {
	p := &Policy{Rules: []Rule{
		{ID: "r1", Enabled: true, Action: ActionBlock,
			Conditions: []Condition{{Field: FieldCIDR, Operator: OpEquals, Value: "203.0.113.0/24"}}},
	}}
	if got := Evaluate(p, Facts{IP: "203.0.113.5"}); !got.Matched {
		t.Fatalf("in-range IP should match")
	}
	if got := Evaluate(p, Facts{IP: "198.51.100.5"}); got.Matched {
		t.Fatalf("out-of-range IP should not match")
	}
	if got := Evaluate(p, Facts{IP: "not-an-ip"}); got.Matched {
		t.Fatalf("malformed IP should not match")
	}
}

func TestEvaluate_ContainsRejectsEmptyValue(t *testing.T) {
	p := &Policy{Rules: []Rule{
		{ID: "r1", Enabled: true, Action: ActionBlock,
			Conditions: []Condition{{Field: FieldPath, Operator: OpContains, Value: ""}}},
	}}
	got := Evaluate(p, Facts{Path: "/anything"})
	if got.Matched {
		t.Fatalf("empty CONTAINS value matched everything: %+v", got)
	}
}

// Mutation check (CLAUDE.md Section 12): remove the zero-condition
// guard in ruleMatches and this test must fail.
func TestMutation_ZeroConditionGuardMatters(t *testing.T) {
	r := Rule{ID: "r1", Enabled: true, Action: ActionBlock, Conditions: nil}
	if ruleMatches(r, Facts{Path: "/x"}) {
		t.Fatal("zero-condition rule must not match — remove this guard and this test must fail")
	}
}
