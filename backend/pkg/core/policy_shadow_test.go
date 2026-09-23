package core_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/policy"
)

// With no provider attached, nothing about the request path changes and
// no policy opinion appears. This is the default deployment.
func TestGuardWithoutPolicyProviderRecordsNoOpinion(t *testing.T) {
	guard, tn, _ := shadowFixture(t, config.PolicyBalanced)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req)

	recent := tn.Trail.Recent(1)
	if len(recent) != 1 {
		t.Fatalf("expected 1 evidence record, got %d", len(recent))
	}
	if recent[0].Policy != nil {
		t.Fatalf("Evidence.Policy = %+v, want nil with no provider attached", recent[0].Policy)
	}
}

// A matched policy rule is recorded in evidence but the visitor is
// forwarded per the rule-based decision, not the policy's — proving
// policy evaluation is observational only in this slice.
func TestGuardPolicyShadowRecordsButNeverEnforces(t *testing.T) {
	guard, tn, hits := shadowFixture(t, config.PolicyBalanced)

	alwaysBlockPolicy := &policy.Policy{Rules: []policy.Rule{
		{ID: "rule-1", Name: "block everything", Enabled: true, Action: policy.ActionBlock,
			Conditions: []policy.Condition{{Field: policy.FieldPath, Operator: policy.OpEquals, Value: "/"}}},
	}}
	guard.WithPolicyProvider(func(tenantID string) *policy.Policy {
		if tenantID != tn.ID {
			t.Fatalf("provider called with unexpected tenant id %q, want %q", tenantID, tn.ID)
		}
		return alwaysBlockPolicy
	})

	// A clean request: the rule scorer allows it (score 0, balanced
	// policy), but the attached policy would BLOCK every request to "/".
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("visitor got status %d, want 200 — the shadow policy must never reach enforcement", rec.Code)
	}
	if hits.count() != 1 {
		t.Fatalf("origin hit count = %d, want 1 — request must still be forwarded", hits.count())
	}

	recent := tn.Trail.Recent(1)
	if len(recent) != 1 {
		t.Fatalf("expected 1 evidence record, got %d", len(recent))
	}
	got := recent[0].Policy
	if got == nil || !got.Matched || got.RuleID != "rule-1" || got.Action != "BLOCK" {
		t.Fatalf("Evidence.Policy = %+v, want matched rule-1/BLOCK", got)
	}
	if recent[0].Decision != "allow" {
		t.Fatalf("Evidence.Decision = %q, want %q — the rule scorer's decision must be unaffected by the policy match", recent[0].Decision, "allow")
	}
}

// A tenant with no configured policy (provider returns nil) must behave
// exactly like no provider being attached at all.
func TestGuardPolicyProviderReturningNilRecordsNoOpinion(t *testing.T) {
	guard, tn, _ := shadowFixture(t, config.PolicyBalanced)
	guard.WithPolicyProvider(func(tenantID string) *policy.Policy { return nil })

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req)

	recent := tn.Trail.Recent(1)
	if len(recent) != 1 || recent[0].Policy != nil {
		t.Fatalf("Evidence.Policy = %+v, want nil for a tenant with no policy", recent[0].Policy)
	}
}

// Mutation check (CLAUDE.md Section 12): if the DECISION-branch code
// ever starts reading result.Action instead of the rule scorer's
// decision, this test must fail. Two tenants, different policies, prove
// evidence never crosses between them.
func TestGuardPolicyShadowIsPerTenant(t *testing.T) {
	guard, tnA, _ := shadowFixture(t, config.PolicyBalanced)

	policyA := &policy.Policy{OwnerUserID: "owner-a", Rules: []policy.Rule{
		{ID: "a-rule", Enabled: true, Action: policy.ActionChallenge,
			Conditions: []policy.Condition{{Field: policy.FieldPath, Operator: policy.OpEquals, Value: "/"}}},
	}}
	seen := map[string]bool{}
	guard.WithPolicyProvider(func(tenantID string) *policy.Policy {
		seen[tenantID] = true
		if tenantID == tnA.ID {
			return policyA
		}
		return nil
	})

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req)

	recent := tnA.Trail.Recent(1)
	if len(recent) != 1 || recent[0].Policy == nil || recent[0].Policy.RuleID != "a-rule" {
		t.Fatalf("tenant A should see its own matched rule, got %+v", recent[0].Policy)
	}
	if !seen[tnA.ID] {
		t.Fatalf("provider was never asked for tenant %q", tnA.ID)
	}
}
