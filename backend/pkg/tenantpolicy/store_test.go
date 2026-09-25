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

func TestRouteClassDraftValidation(t *testing.T) {
	valid := Document{Mode: "shadow", RouteClasses: map[string]string{
		"/account/signin": policy.ClassLogin,
		"/order/confirm":  policy.ClassCheckout,
	}}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid route labels: %v", err)
	}
	for _, routes := range []map[string]string{
		{"/account/signin": policy.ClassAPI}, // loose API limit is unsafe
		{"/login": policy.ClassCheckout},     // built-in login cannot be weakened
		{"/account/../signin": policy.ClassLogin},
		{"/account/%73ignin": policy.ClassLogin},
		{"account/signin": policy.ClassLogin},
		{"/account/signin?x=1": policy.ClassLogin},
	} {
		doc := Document{Mode: "shadow", RouteClasses: routes}
		if err := doc.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("routes=%v err=%v, want invalid", routes, err)
		}
	}
}
