package core_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/policy"
)

func TestTenantPolicyEnforcementAndEvidence(t *testing.T) {
	for _, tc := range []struct {
		name         string
		action       policy.Action
		mode         string
		baseline     config.PolicyMode
		wantStatus   int
		wantOrigin   int
		wantDecision string
	}{
		{"shadow block", policy.ActionBlock, "shadow", config.PolicyBalanced, 200, 1, "allow"},
		{"active block", policy.ActionBlock, "enforce", config.PolicyBalanced, 403, 0, "block"},
		{"active challenge", policy.ActionChallenge, "enforce", config.PolicyBalanced, 200, 0, "challenge"},
		{"active allow", policy.ActionAllow, "enforce", config.PolicyStrict, 200, 1, "allow"},
		{"active rate limit", policy.ActionRateLimit, "enforce", config.PolicyBalanced, 429, 0, "rate_limit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			guard, tenant, hits := shadowFixture(t, tc.baseline)
			p := &policy.Policy{TenantID: tenant.ID, Version: 3, Mode: tc.mode, BlockMessage: "blocked by site policy", Rules: []policy.Rule{{ID: "rule-1", Name: "home", Enabled: true, Action: tc.action, Conditions: []policy.Condition{{Field: policy.FieldPath, Operator: policy.OpEquals, Value: "/"}}}}}
			guard.WithPolicyProvider(func(id string) *policy.Policy {
				if id != tenant.ID {
					t.Fatalf("wrong tenant %q", id)
				}
				return p
			})
			req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
			rec := httptest.NewRecorder()
			guard.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status=%d want %d", rec.Code, tc.wantStatus)
			}
			if hits.count() != tc.wantOrigin {
				t.Fatalf("origin hits=%d want %d", hits.count(), tc.wantOrigin)
			}
			recent := tenant.Trail.Recent(1)
			if len(recent) != 1 || recent[0].Decision != tc.wantDecision || recent[0].Policy == nil || recent[0].Policy.Version != 3 {
				t.Fatalf("evidence=%+v", recent)
			}
			if tc.name == "active block" && !strings.Contains(rec.Body.String(), "blocked by site policy") {
				t.Fatalf("block message missing: %q", rec.Body.String())
			}
			if tc.name == "active rate limit" && rec.Header().Get("Retry-After") != "60" {
				t.Fatal("rate limit response missing Retry-After")
			}
		})
	}
}

func TestTenantPolicyDoesNotAllowHighRiskPathOnlyRule(t *testing.T) {
	guard, tenant, _ := shadowFixture(t, config.PolicyBalanced)
	guard.WithPolicyProvider(func(string) *policy.Policy {
		return &policy.Policy{Version: 1, Mode: "enforce", Rules: []policy.Rule{{ID: "allow", Enabled: true, Action: policy.ActionAllow, Conditions: []policy.Condition{{Field: policy.FieldPath, Operator: policy.OpEquals, Value: "/"}}}}}
	})
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	req.Header.Set("User-Agent", "python-requests/2.0") // scripting_tool is a hard block signal
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("high-risk path-only PASS status=%d want 403", rec.Code)
	}
	e := tenant.Trail.Recent(1)[0]
	if e.Policy == nil || !e.Policy.Matched || e.Policy.Enforced || e.Decision != "block" {
		t.Fatalf("evidence=%+v", e)
	}
}
