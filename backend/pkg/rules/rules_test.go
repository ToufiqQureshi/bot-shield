package rules

import (
	"context"
	"reflect"
	"testing"
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
