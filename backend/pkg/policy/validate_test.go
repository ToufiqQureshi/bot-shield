package policy

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateRuleBoundsRequestPathWork(t *testing.T) {
	tooMany := make([]Condition, maxRuleConditions+1)
	for i := range tooMany {
		tooMany[i] = Condition{Field: FieldPath, Operator: OpEquals, Value: "/"}
	}
	if err := ValidateRule(Rule{Action: ActionBlock, Conditions: tooMany}, 100); !errors.Is(err, ErrTooManyConditions) {
		t.Fatalf("too many conditions = %v, want ErrTooManyConditions", err)
	}
	longRegex := Rule{Action: ActionBlock, Conditions: []Condition{{Field: FieldPath, Operator: OpMatches, Value: strings.Repeat("a", maxConditionValueLen+1)}}}
	if err := ValidateRule(longRegex, 100); !errors.Is(err, ErrValueTooLong) {
		t.Fatalf("oversized regex = %v, want ErrValueTooLong", err)
	}
}

func TestValidateRule_NoConditions(t *testing.T) {
	err := ValidateRule(Rule{Action: ActionBlock}, 90)
	if !errors.Is(err, ErrNoConditions) {
		t.Fatalf("got %v, want ErrNoConditions", err)
	}
}

func TestValidateRule_UnknownAction(t *testing.T) {
	r := Rule{Action: "NUKE", Conditions: []Condition{{Field: FieldPath, Operator: OpEquals, Value: "/x"}}}
	if err := ValidateRule(r, 90); !errors.Is(err, ErrUnknownAction) {
		t.Fatalf("got %v, want ErrUnknownAction", err)
	}
}

func TestValidateRule_UnknownField(t *testing.T) {
	r := Rule{Action: ActionBlock, Conditions: []Condition{{Field: "Vibes", Operator: OpEquals, Value: "x"}}}
	if err := ValidateRule(r, 90); !errors.Is(err, ErrUnknownField) {
		t.Fatalf("got %v, want ErrUnknownField", err)
	}
}

func TestValidateRule_UnknownOperator(t *testing.T) {
	r := Rule{Action: ActionBlock, Conditions: []Condition{{Field: FieldPath, Operator: "YEETS", Value: "x"}}}
	if err := ValidateRule(r, 90); !errors.Is(err, ErrUnknownOperator) {
		t.Fatalf("got %v, want ErrUnknownOperator", err)
	}
}

func TestValidateRule_EmptyValue(t *testing.T) {
	r := Rule{Action: ActionBlock, Conditions: []Condition{{Field: FieldPath, Operator: OpEquals, Value: ""}}}
	if err := ValidateRule(r, 90); !errors.Is(err, ErrEmptyValue) {
		t.Fatalf("got %v, want ErrEmptyValue", err)
	}
}

func TestValidateRule_BadRegex(t *testing.T) {
	r := Rule{Action: ActionBlock, Conditions: []Condition{{Field: FieldPath, Operator: OpMatches, Value: "(unterminated"}}}
	if err := ValidateRule(r, 90); !errors.Is(err, ErrBadRegex) {
		t.Fatalf("got %v, want ErrBadRegex", err)
	}
}

func TestValidateRule_NumericOperatorNonNumericValue(t *testing.T) {
	r := Rule{Action: ActionBlock, Conditions: []Condition{{Field: FieldScore, Operator: OpGT, Value: "high"}}}
	if err := ValidateRule(r, 90); !errors.Is(err, ErrBadNumber) {
		t.Fatalf("got %v, want ErrBadNumber", err)
	}
}

func TestValidateRule_NumericOperatorOnNonScoreField(t *testing.T) {
	r := Rule{Action: ActionBlock, Conditions: []Condition{{Field: FieldPath, Operator: OpGT, Value: "5"}}}
	if err := ValidateRule(r, 90); !errors.Is(err, ErrOperatorNotValidForField) {
		t.Fatalf("got %v, want ErrOperatorNotValidForField", err)
	}
}

func TestValidateRule_ScoreWithContainsRejected(t *testing.T) {
	r := Rule{Action: ActionBlock, Conditions: []Condition{{Field: FieldScore, Operator: OpContains, Value: "50"}}}
	if err := ValidateRule(r, 90); !errors.Is(err, ErrOperatorNotValidForField) {
		t.Fatalf("got %v, want ErrOperatorNotValidForField", err)
	}
}

func TestValidateRule_ScoreWithNonNumericEqualsRejected(t *testing.T) {
	r := Rule{Action: ActionBlock, Conditions: []Condition{{Field: FieldScore, Operator: OpEquals, Value: "high"}}}
	if err := ValidateRule(r, 90); !errors.Is(err, ErrBadNumber) {
		t.Fatalf("got %v, want ErrBadNumber", err)
	}
}

func TestValidateRule_CIDRWithNonEqualsOperatorRejected(t *testing.T) {
	r := Rule{Action: ActionBlock, Conditions: []Condition{{Field: FieldCIDR, Operator: OpContains, Value: "203.0.113.0/24"}}}
	if err := ValidateRule(r, 90); !errors.Is(err, ErrOperatorNotValidForField) {
		t.Fatalf("got %v, want ErrOperatorNotValidForField", err)
	}
}

func TestValidateRule_MalformedCIDRRejected(t *testing.T) {
	r := Rule{Action: ActionBlock, Conditions: []Condition{{Field: FieldCIDR, Operator: OpEquals, Value: "not-a-cidr"}}}
	if err := ValidateRule(r, 90); !errors.Is(err, ErrBadCIDR) {
		t.Fatalf("got %v, want ErrBadCIDR", err)
	}
}

func TestValidateRule_ValidCIDRAccepted(t *testing.T) {
	r := Rule{Action: ActionBlock, Conditions: []Condition{{Field: FieldCIDR, Operator: OpEquals, Value: "203.0.113.0/24"}}}
	if err := ValidateRule(r, 90); err != nil {
		t.Fatalf("valid CIDR rejected: %v", err)
	}
}

// The dashboard's rule builder offers ASN/Geo/TLS Version, but nothing
// computes them yet — ValidateRule must reject a rule that uses one
// rather than let the dashboard show a rule as active that can never fire.
func TestValidateRule_NotYetSupportedFieldRejected(t *testing.T) {
	r := Rule{Action: ActionBlock, Conditions: []Condition{{Field: "Geo", Operator: OpEquals, Value: "RU"}}}
	if err := ValidateRule(r, 90); !errors.Is(err, ErrUnknownField) {
		t.Fatalf("got %v, want ErrUnknownField for a not-yet-supported field", err)
	}
}

func TestValidateRule_ValidRulePasses(t *testing.T) {
	r := Rule{Action: ActionBlock, Conditions: []Condition{{Field: FieldJA4, Operator: OpEquals, Value: "t13d..."}}}
	if err := ValidateRule(r, 90); err != nil {
		t.Fatalf("valid rule rejected: %v", err)
	}
}

// UA-only-allow guardrail.
func TestValidateRule_UAOnlyAllowRejected(t *testing.T) {
	r := Rule{Action: ActionAllow, Conditions: []Condition{{Field: FieldUserAgent, Operator: OpEquals, Value: "Mozilla/5.0"}}}
	if err := ValidateRule(r, 90); !errors.Is(err, ErrUAOnlyAllow) {
		t.Fatalf("got %v, want ErrUAOnlyAllow", err)
	}
}

func TestValidateRule_UAOnlyBlockRejected(t *testing.T) {
	r := Rule{Action: ActionBlock, Conditions: []Condition{{Field: FieldUserAgent, Operator: OpEquals, Value: "python-requests"}}}
	if err := ValidateRule(r, 100); !errors.Is(err, ErrUAOnlyBlock) {
		t.Fatalf("got %v, want ErrUAOnlyBlock", err)
	}
}

func TestValidateRule_UAPlusOtherConditionAllowIsFine(t *testing.T) {
	r := Rule{Action: ActionAllow, Conditions: []Condition{
		{Field: FieldUserAgent, Operator: OpEquals, Value: "Mozilla/5.0"},
		{Field: FieldJA4, Operator: OpEquals, Value: "t13d..."},
	}}
	if err := ValidateRule(r, 90); err != nil {
		t.Fatalf("UA + JA4 allow rule should be permitted: %v", err)
	}
}

func TestValidateRule_UAOnlyNonAllowActionIsFine(t *testing.T) {
	r := Rule{Action: ActionChallenge, Conditions: []Condition{{Field: FieldUserAgent, Operator: OpEquals, Value: "curl"}}}
	if err := ValidateRule(r, 90); err != nil {
		t.Fatalf("UA-only CHALLENGE should be permitted: %v", err)
	}
}

// Deceive-floor-above-block guardrail.
func TestValidateRule_DeceiveWithoutScoreFloorRejected(t *testing.T) {
	r := Rule{Action: ActionDeceive, Conditions: []Condition{{Field: FieldPath, Operator: OpEquals, Value: "/pricing"}}}
	if err := ValidateRule(r, 90); !errors.Is(err, ErrDeceiveNotStricter) {
		t.Fatalf("got %v, want ErrDeceiveNotStricter", err)
	}
}

func TestValidateRule_DeceiveFloorEqualToBlockRejected(t *testing.T) {
	r := Rule{Action: ActionDeceive, Conditions: []Condition{{Field: FieldScore, Operator: OpGTE, Value: "90"}}}
	if err := ValidateRule(r, 90); !errors.Is(err, ErrDeceiveNotStricter) {
		t.Fatalf("floor == block threshold must be rejected, got %v", err)
	}
}

func TestValidateRule_DeceiveFloorAboveBlockAccepted(t *testing.T) {
	r := Rule{Action: ActionDeceive, Conditions: []Condition{{Field: FieldScore, Operator: OpGT, Value: "91"}}}
	if err := ValidateRule(r, 90); err != nil {
		t.Fatalf("floor above block threshold should be accepted: %v", err)
	}
}

func TestValidateRule_DeceiveWithLTScoreConditionRejected(t *testing.T) {
	// An upper bound (LT) is not a floor — must not satisfy the guardrail.
	r := Rule{Action: ActionDeceive, Conditions: []Condition{{Field: FieldScore, Operator: OpLT, Value: "200"}}}
	if err := ValidateRule(r, 90); !errors.Is(err, ErrDeceiveNotStricter) {
		t.Fatalf("LT condition should not satisfy the floor guardrail, got %v", err)
	}
}

// Mutation check (CLAUDE.md Section 12): comment out the UA-only-allow
// check in ValidateRule and this test must start failing.
func TestMutation_UAOnlyAllowGuardrailMatters(t *testing.T) {
	r := Rule{Action: ActionAllow, Conditions: []Condition{{Field: FieldUserAgent, Operator: OpEquals, Value: "anything"}}}
	if err := ValidateRule(r, 90); err == nil {
		t.Fatal("UA-only allow must be rejected — remove the guardrail and this test must fail")
	}
}

// Mutation check: comment out the deceive-floor check and this must fail.
func TestMutation_DeceiveFloorGuardrailMatters(t *testing.T) {
	r := Rule{Action: ActionDeceive, Conditions: []Condition{{Field: FieldPath, Operator: OpEquals, Value: "/x"}}}
	if err := ValidateRule(r, 90); err == nil {
		t.Fatal("deceive without a score floor must be rejected — remove the guardrail and this test must fail")
	}
}
