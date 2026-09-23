package tenantpolicy

import (
	"errors"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/policy"
)

func TestDocumentValidation(t *testing.T) {
	base := Document{Mode: "shadow", Rules: []policy.Rule{{ID: "r1", Name: "limit busy API", Enabled: true, Action: policy.ActionRateLimit, Conditions: []policy.Condition{
		{Field: policy.FieldRequestClass, Operator: policy.OpEquals, Value: policy.ClassAPI},
		{Field: policy.FieldSignal, Operator: policy.OpEquals, Value: "velocity_spike"},
	}}}, Allowlist: []string{"192.0.2.0/24"}, ChallengeTheme: "branded"}
	if err := base.Validate(); err != nil {
		t.Fatalf("valid document: %v", err)
	}
	bad := base
	bad.Rules = append([]policy.Rule(nil), base.Rules...)
	bad.Rules[0].Conditions = bad.Rules[0].Conditions[:1]
	if err := bad.Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("rate limit without velocity = %v", err)
	}
	bad = base
	bad.Allowlist = []string{"192.0.2.1"}
	if err := bad.Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("non-CIDR allowlist = %v", err)
	}
	bad = base
	bad.Mode = "enforce-now"
	if err := bad.Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid mode = %v", err)
	}
	bad = base
	bad.Rules = append([]policy.Rule(nil), base.Rules...)
	bad.Rules = append(bad.Rules, bad.Rules[0])
	if err := bad.Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("duplicate ID = %v", err)
	}
}
