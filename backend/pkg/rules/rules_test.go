package rules

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/policy"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
)

func TestManaged_AllEnabledAndNamed(t *testing.T) {
	managed := Managed()
	if len(managed) == 0 {
		t.Fatal("Managed() returned no rules")
	}
	for _, r := range managed {
		if r.ID == "" || r.Name == "" || r.Description == "" {
			t.Errorf("managed rule missing a field: %+v", r)
		}
		if !r.Enabled {
			t.Errorf("managed rule %q reported disabled — these are always-on detection layers", r.ID)
		}
	}
}

func TestEncodeDecodeConditions_RoundTrip(t *testing.T) {
	in := []Condition{
		{Field: "JA4 Fingerprint", Operator: "EQUALS", Value: "t13d1516h2_8daaf6152771_0271d189196b"},
		{Field: "Threat Score", Operator: ">", Value: "80"},
	}

	encoded, err := encodeConditions(in)
	if err != nil {
		t.Fatalf("encodeConditions: %v", err)
	}

	out, err := decodeConditions(encoded)
	if err != nil {
		t.Fatalf("decodeConditions: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round trip mismatch: got %+v, want %+v", out, in)
	}
}

func TestEncodeConditions_NilBecomesEmptyArray(t *testing.T) {
	encoded, err := encodeConditions(nil)
	if err != nil {
		t.Fatalf("encodeConditions(nil): %v", err)
	}
	if encoded != "[]" {
		t.Fatalf("encodeConditions(nil) = %q, want %q (nil must not serialize as JSON null)", encoded, "[]")
	}
}

func TestDecodeConditions_RejectsMalformedJSON(t *testing.T) {
	if _, err := decodeConditions("not json"); err == nil {
		t.Fatal("decodeConditions(garbage) = nil error, want an error")
	}
}

func TestStore_NilPoolFailsClearly(t *testing.T) {
	s := NewStore(nil)
	ctx := context.Background()

	if _, err := s.List(ctx, "usr_x"); err == nil {
		t.Error("List(nil pool) = nil, want an error")
	}
	if _, err := s.Create(ctx, "usr_x", "name", []Condition{{Field: "f", Operator: "o", Value: "v"}}, "BLOCK"); err == nil {
		t.Error("Create(nil pool) = nil, want an error")
	}
	if err := s.SetEnabled(ctx, "usr_x", "rule_1", false); err == nil {
		t.Error("SetEnabled(nil pool) = nil, want an error")
	}
}

func TestValidateRuleRejectsUnsafeOrUnavailableRules(t *testing.T) {
	tests := []struct {
		name       string
		conditions []Condition
		action     string
	}{
		{"no conditions", nil, "BLOCK"},
		{"unknown field", []Condition{{Field: "Mystery", Operator: "EQUALS", Value: "x"}}, "BLOCK"},
		{"unavailable field", []Condition{{Field: "ASN", Operator: "EQUALS", Value: "123"}}, "BLOCK"},
		{"unknown operator", []Condition{{Field: "Request Path", Operator: "EXEC", Value: "/admin"}}, "BLOCK"},
		{"unknown action", []Condition{{Field: "Request Path", Operator: "EQUALS", Value: "/admin"}}, "DELETE"},
		{"UA only pass", []Condition{{Field: "User-Agent", Operator: "EQUALS", Value: "Googlebot"}}, "PASS"},
		{"UA only block", []Condition{{Field: "User-Agent", Operator: "EQUALS", Value: "python-requests"}}, "BLOCK"},
		{"deceive without score floor", []Condition{{Field: "Request Path", Operator: "EQUALS", Value: "/pricing"}}, "DECEIVE"},
		{"deceive at live block bar", []Condition{{Field: "Threat Score", Operator: string(policy.OpGTE), Value: "100"}}, "DECEIVE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateRule(tt.conditions, tt.action, signals.HardBlockThreshold()); !errors.Is(err, ErrInvalidRule) {
				t.Fatalf("validateRule() = %v, want ErrInvalidRule", err)
			}
		})
	}
}

func TestValidateRuleAllowsSupportedRulesAndUsesBlockFloor(t *testing.T) {
	valid := []struct {
		conditions []Condition
		action     string
	}{
		{[]Condition{{Field: "JA4 Fingerprint", Operator: "EQUALS", Value: "t13d..."}}, "BLOCK"},
		{[]Condition{{Field: "Request Path", Operator: "CONTAINS", Value: "/login"}}, "CHALLENGE"},
		{[]Condition{{Field: "User-Agent", Operator: "EQUALS", Value: "Googlebot"}, {Field: "JA4 Fingerprint", Operator: "EQUALS", Value: "t13d..."}}, "PASS"},
		{[]Condition{{Field: "Threat Score", Operator: string(policy.OpGTE), Value: "101"}}, "DECEIVE"},
	}
	for _, tt := range valid {
		if err := validateRule(tt.conditions, tt.action, signals.HardBlockThreshold()); err != nil {
			t.Errorf("validateRule(%v, %s) = %v", tt.conditions, tt.action, err)
		}
	}
	deceive := []Condition{{Field: "Threat Score", Operator: string(policy.OpGTE), Value: "101"}}
	if err := validateRule(deceive, "DECEIVE", 120); !errors.Is(err, ErrInvalidRule) {
		t.Fatalf("deceive below owner block threshold = %v, want ErrInvalidRule", err)
	}
}

func TestCreateRejectsInvalidRuleBeforeDatabase(t *testing.T) {
	_, err := NewStore(nil).Create(context.Background(), "owner", "bad", []Condition{{Field: "ASN", Operator: "EQUALS", Value: "123"}}, "BLOCK")
	if !errors.Is(err, ErrInvalidRule) {
		t.Fatalf("Create() = %v, want ErrInvalidRule", err)
	}
}

func TestCreateRejectsOversizedRuleNameBeforeDatabase(t *testing.T) {
	_, err := NewStore(nil).Create(context.Background(), "owner", strings.Repeat("a", 129), []Condition{{Field: "Request Path", Operator: "EQUALS", Value: "/"}}, "BLOCK")
	if !errors.Is(err, ErrInvalidRule) {
		t.Fatalf("Create() = %v, want ErrInvalidRule", err)
	}
}
