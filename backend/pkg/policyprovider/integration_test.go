package policyprovider_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/challenge"
	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/core"
	"github.com/ToufiqQureshi/hakaishield/pkg/policyprovider"
	"github.com/ToufiqQureshi/hakaishield/pkg/rules"
	"github.com/ToufiqQureshi/hakaishield/pkg/settings"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"
)

// memRules is an in-memory ruleLister keyed by owner, standing in for a
// real rules.Store without a Postgres dependency — the real Store's own
// SQL is covered by pkg/rules' Postgres integration test.
type memRules map[string][]rules.CustomRule

func (m memRules) List(_ context.Context, ownerUserID string) ([]rules.CustomRule, error) {
	return m[ownerUserID], nil
}

type memSettings map[string]*settings.Protection

func (m memSettings) Get(_ context.Context, ownerUserID string) (*settings.Protection, error) {
	if p, ok := m[ownerUserID]; ok {
		return p, nil
	}
	return &settings.Protection{BlockThreshold: 90, ChallengeThreshold: 50}, nil
}

func blockPathRule(id, path string) rules.CustomRule {
	return rules.CustomRule{
		ID: id, Name: id, Enabled: true, Action: "BLOCK",
		Conditions: []rules.Condition{{Field: "Request Path", Operator: "EQUALS", Value: path}},
	}
}

// TestDBToGuardPath_TwoOwnersTwoDomains wires a real tenant.Store and a
// real core.Guard to policyprovider.Provider (the only fakes are the
// rules/settings storage, which is what a real dev-Postgres integration
// test in pkg/rules already covers), and drives an actual request through
// ServeHTTP for two different owners' domains. This is the "DB-to-guard
// path" the Phase 1 production review asked for: not unit-testing
// Provider in isolation, but proving core.Guard actually receives the
// right owner's policy for the right domain.
func TestDBToGuardPath_TwoOwnersTwoDomains(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(origin.Close)
	proxy, err := core.NewOriginProxy(origin.URL)
	if err != nil {
		t.Fatal(err)
	}

	store := tenant.NewStore()
	if err := store.Add("tenant-a", tenant.TenantConfig{Mode: config.ModeEnforce, Policy: config.PolicyBalanced, OwnerUserID: "owner-a"}, []string{"a.example.com"}, proxy); err != nil {
		t.Fatal(err)
	}
	if err := store.Add("tenant-b", tenant.TenantConfig{Mode: config.ModeEnforce, Policy: config.PolicyBalanced, OwnerUserID: "owner-b"}, []string{"b.example.com"}, proxy); err != nil {
		t.Fatal(err)
	}

	rs := memRules{
		"owner-a": {blockPathRule("a-rule", "/")},
		"owner-b": {blockPathRule("b-rule", "/")},
	}
	provider := policyprovider.New(store, rs, memSettings{})

	challengeHandler, err := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")
	if err != nil {
		t.Fatal(err)
	}
	guard := core.NewGuard(store, challengeHandler)
	guard.WithPolicyProvider(provider.ForTenant)

	for _, tc := range []struct{ host, wantRuleID string }{
		{"a.example.com", "a-rule"},
		{"b.example.com", "b-rule"},
	} {
		req := httptest.NewRequest("GET", "http://"+tc.host+"/", nil)
		rec := httptest.NewRecorder()
		guard.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s: visitor status = %d, want 200 — shadow policy must never enforce", tc.host, rec.Code)
		}

		tn, err := store.GetByHost(tc.host)
		if err != nil {
			t.Fatalf("%s: GetByHost: %v", tc.host, err)
		}
		recent := tn.Trail.Recent(1)
		if len(recent) != 1 || recent[0].Policy == nil || recent[0].Policy.RuleID != tc.wantRuleID {
			t.Fatalf("%s: Evidence.Policy = %+v, want matched %s", tc.host, recent, tc.wantRuleID)
		}
	}
}
