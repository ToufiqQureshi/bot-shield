package policyprovider

import (
	"context"
	"errors"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenantpolicy"
	"testing"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/policy"
	"github.com/ToufiqQureshi/hakaishield/pkg/rules"
	"github.com/ToufiqQureshi/hakaishield/pkg/settings"
)

type blockingTenantPolicies struct {
	started chan struct{}
	release chan struct{}
}

type routeTenantPolicies struct{}

func (routeTenantPolicies) LoadForTenant(_ context.Context, id string) (*tenantpolicy.Revision, error) {
	return &tenantpolicy.Revision{TenantID: id, Version: 2, OwnerUserID: "owner-1", Document: tenantpolicy.Document{
		Mode: "enforce", RouteClasses: map[string]string{"/account/signin": policy.ClassLogin},
	}}, nil
}

func TestLoadedTenantRouteLabelsAreIsolated(t *testing.T) {
	p := New(fakeTenants{"one": "owner-1", "other": "owner-2"}, newFakeRules(), fakeSettings{}).WithTenantPolicies(routeTenantPolicies{})
	pol, found := p.loadTenant("one", "owner-1", p.currentEpoch())
	if !found || pol == nil || pol.ClassifyRoute("/account/signin", "POST") != policy.ClassLogin {
		t.Fatalf("tenant route label lost: policy=%+v found=%v", pol, found)
	}
	if pol, found := p.loadTenant("other", "owner-2", p.currentEpoch()); found || pol != nil {
		t.Fatalf("wrong owner received route labels: policy=%+v found=%v", pol, found)
	}
}

func (b *blockingTenantPolicies) LoadForTenant(_ context.Context, id string) (*tenantpolicy.Revision, error) {
	select {
	case b.started <- struct{}{}:
	default:
	}
	<-b.release
	return &tenantpolicy.Revision{TenantID: id, Version: 1, OwnerUserID: "owner-1", Document: tenantpolicy.Document{Mode: "shadow", Rules: []policy.Rule{{ID: "r1", Name: "block admin", Enabled: true, Action: policy.ActionBlock, Conditions: []policy.Condition{{Field: policy.FieldPath, Operator: policy.OpEquals, Value: "/admin"}}}}}}, nil
}

func TestProductionProviderCacheMissNeverWaitsForDatabase(t *testing.T) {
	loader := &blockingTenantPolicies{started: make(chan struct{}, 1), release: make(chan struct{})}
	p := New(fakeTenants{"t1": "owner-1"}, newFakeRules(), fakeSettings{}).WithTenantPolicies(loader)
	start := time.Now()
	if got := p.ForTenant("t1"); got != nil {
		t.Fatalf("cold cache should use baseline, got %+v", got)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("request blocked on database for %s", elapsed)
	}
	select {
	case <-loader.started:
	case <-time.After(time.Second):
		t.Fatal("load did not start")
	}
	close(loader.release)
	deadline := time.Now().Add(time.Second)
	for {
		got := p.ForTenant("t1")
		if got != nil {
			if got.Version != 1 || policy.Evaluate(got, policy.Facts{Path: "/admin"}).RuleID != "r1" {
				t.Fatalf("loaded policy=%+v", got)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("cache did not receive loaded policy")
		}
		time.Sleep(time.Millisecond)
	}
}

// fakeTenants is a tenantLookup test double: a plain map from tenant ID
// to owner ID, so tests can exercise ForTenant without a real DB-backed
// tenant.Store.
type fakeTenants map[string]string

func (f fakeTenants) OwnerUserID(tenantID string) (string, bool) {
	owner, ok := f[tenantID]
	return owner, ok
}

// fakeRules is a ruleLister test double that counts calls per owner, so
// tests can assert caching actually avoids repeat DB hits.
type fakeRules struct {
	byOwner map[string][]rules.CustomRule
	err     error
	calls   map[string]int
}

func newFakeRules() *fakeRules {
	return &fakeRules{byOwner: map[string][]rules.CustomRule{}, calls: map[string]int{}}
}

func (f *fakeRules) List(_ context.Context, ownerUserID string) ([]rules.CustomRule, error) {
	f.calls[ownerUserID]++
	if f.err != nil {
		return nil, f.err
	}
	return f.byOwner[ownerUserID], nil
}

type fakeSettings struct {
	byOwner map[string]*settings.Protection
}

func (f fakeSettings) Get(_ context.Context, ownerUserID string) (*settings.Protection, error) {
	if p, ok := f.byOwner[ownerUserID]; ok {
		return p, nil
	}
	return &settings.Protection{BlockThreshold: 90, ChallengeThreshold: 50}, nil
}

func blockRule(id string) rules.CustomRule {
	return rules.CustomRule{
		ID: id, Name: id, Enabled: true, Action: "BLOCK",
		Conditions: []rules.Condition{{Field: "Request Path", Operator: "EQUALS", Value: "/admin"}},
	}
}

func TestForTenant_UnknownTenantReturnsNil(t *testing.T) {
	p := New(fakeTenants{}, newFakeRules(), fakeSettings{})
	if got := p.ForTenant("ghost"); got != nil {
		t.Fatalf("got %+v, want nil for an unknown tenant", got)
	}
}

func TestForTenant_TenantWithNoRulesReturnsNil(t *testing.T) {
	tenants := fakeTenants{"t1": "owner-1"}
	p := New(tenants, newFakeRules(), fakeSettings{})
	if got := p.ForTenant("t1"); got != nil {
		t.Fatalf("got %+v, want nil for an owner with no rules", got)
	}
}

func TestForTenant_ReturnsMatchingPolicy(t *testing.T) {
	tenants := fakeTenants{"t1": "owner-1"}
	rs := newFakeRules()
	rs.byOwner["owner-1"] = []rules.CustomRule{blockRule("r1")}
	p := New(tenants, rs, fakeSettings{})

	got := p.ForTenant("t1")
	if got == nil {
		t.Fatal("got nil, want a policy")
	}
	result := policy.Evaluate(got, policy.Facts{Path: "/admin"})
	if !result.Matched || result.RuleID != "r1" {
		t.Fatalf("Evaluate = %+v, want matched r1", result)
	}
}

// Two owners must never see each other's rules through the same cache.
func TestForTenant_TwoOwnersAreIsolated(t *testing.T) {
	tenants := fakeTenants{"t1": "owner-1", "t2": "owner-2"}
	rs := newFakeRules()
	rs.byOwner["owner-1"] = []rules.CustomRule{blockRule("owner1-rule")}
	rs.byOwner["owner-2"] = []rules.CustomRule{blockRule("owner2-rule")}
	p := New(tenants, rs, fakeSettings{})

	r1 := policy.Evaluate(p.ForTenant("t1"), policy.Facts{Path: "/admin"})
	r2 := policy.Evaluate(p.ForTenant("t2"), policy.Facts{Path: "/admin"})
	if r1.RuleID != "owner1-rule" {
		t.Fatalf("tenant t1 got rule %q, want owner1-rule", r1.RuleID)
	}
	if r2.RuleID != "owner2-rule" {
		t.Fatalf("tenant t2 got rule %q, want owner2-rule", r2.RuleID)
	}
}

// Two domains (tenants) owned by the same account must share that
// account's one policy — mitigation_rules is owner-scoped, not
// domain-scoped, and the cache must reflect that rather than treating
// each tenant ID as an independent cache key that happens to collide.
func TestForTenant_TwoDomainsSameOwnerShareRules(t *testing.T) {
	tenants := fakeTenants{"domain-a": "owner-1", "domain-b": "owner-1"}
	rs := newFakeRules()
	rs.byOwner["owner-1"] = []rules.CustomRule{blockRule("shared-rule")}
	p := New(tenants, rs, fakeSettings{})

	p.ForTenant("domain-a")
	p.ForTenant("domain-b")
	if rs.calls["owner-1"] != 1 {
		t.Fatalf("List called %d times for one owner across two domains, want 1 (cache should be keyed by owner)", rs.calls["owner-1"])
	}
}

func TestForTenant_CachesWithinTTL(t *testing.T) {
	tenants := fakeTenants{"t1": "owner-1"}
	rs := newFakeRules()
	rs.byOwner["owner-1"] = []rules.CustomRule{blockRule("r1")}
	p := New(tenants, rs, fakeSettings{})

	p.ForTenant("t1")
	p.ForTenant("t1")
	p.ForTenant("t1")
	if rs.calls["owner-1"] != 1 {
		t.Fatalf("List called %d times, want 1 within the cache TTL", rs.calls["owner-1"])
	}
}

func TestForTenant_RefetchesAfterTTLExpires(t *testing.T) {
	tenants := fakeTenants{"t1": "owner-1"}
	rs := newFakeRules()
	rs.byOwner["owner-1"] = []rules.CustomRule{blockRule("r1")}
	p := New(tenants, rs, fakeSettings{})

	now := time.Now()
	p.now = func() time.Time { return now }
	p.ForTenant("t1")
	p.now = func() time.Time { return now.Add(cacheTTL + time.Second) }
	p.ForTenant("t1")

	if rs.calls["owner-1"] != 2 {
		t.Fatalf("List called %d times, want 2 after TTL expiry", rs.calls["owner-1"])
	}
}

// A DB error must fail open (nil policy, i.e. no shadow opinion) rather
// than propagate — this is a shadow-only feature, and it must never be
// the thing that breaks request handling for a customer.
func TestForTenant_DBErrorFailsOpenAndNegativeCaches(t *testing.T) {
	tenants := fakeTenants{"t1": "owner-1"}
	rs := newFakeRules()
	rs.err = errors.New("connection refused")
	p := New(tenants, rs, fakeSettings{})

	if got := p.ForTenant("t1"); got != nil {
		t.Fatalf("got %+v, want nil on DB error", got)
	}
	p.ForTenant("t1")
	if rs.calls["owner-1"] != 1 {
		t.Fatalf("List called %d times, want 1 — a DB outage must not retry on every request", rs.calls["owner-1"])
	}
}

func TestForTenant_NilProviderIsNoOp(t *testing.T) {
	var p *Provider
	if got := p.ForTenant("anything"); got != nil {
		t.Fatalf("got %+v, want nil from a nil *Provider", got)
	}
}

func TestNew_NilDependencyIsNoOp(t *testing.T) {
	p := New(nil, newFakeRules(), fakeSettings{})
	if got := p.ForTenant("t1"); got != nil {
		t.Fatalf("got %+v, want nil when tenantLookup is nil", got)
	}
}

// DECEIVE rules must be validated against the owner's real configured
// threshold, not a default, so ToPolicy's blockThreshold plumbing here
// is exercised end to end rather than assumed.
func TestForTenant_UsesOwnerConfiguredBlockThreshold(t *testing.T) {
	tenants := fakeTenants{"t1": "owner-1"}
	rs := newFakeRules()
	rs.byOwner["owner-1"] = []rules.CustomRule{{
		ID: "deceive-rule", Name: "deceive-rule", Enabled: true, Action: "DECEIVE",
		Conditions: []rules.Condition{{Field: "Threat Score", Operator: ">", Value: "150"}},
	}}
	sg := fakeSettings{byOwner: map[string]*settings.Protection{
		"owner-1": {BlockThreshold: 200, ChallengeThreshold: 100},
	}}
	p := New(tenants, rs, sg)

	// Floor of 150 does not exceed the owner's configured 200, so
	// ValidateRule inside ToPolicy must drop this rule.
	got := p.ForTenant("t1")
	if got != nil && len(got.Rules) != 0 {
		t.Fatalf("expected the DECEIVE rule to be dropped (floor 150 <= configured threshold 200), got %+v", got.Rules)
	}
}

// Mutation check (CLAUDE.md Section 12): if the owner-scoped cache key
// were ever changed to the tenant ID instead, this test would still
// pass by accident (each tenant gets its own cache entry either way),
// so TestForTenant_TwoDomainsSameOwnerShareRules above is the one that
// actually exercises the distinction — remove owner-keying there and
// that test fails while this one would not, which is why both exist.
func TestMutation_CacheIsolationCatchesCrossTenantLeak(t *testing.T) {
	tenants := fakeTenants{"t1": "owner-1", "t2": "owner-2"}
	rs := newFakeRules()
	rs.byOwner["owner-1"] = []rules.CustomRule{blockRule("owner1-only")}
	p := New(tenants, rs, fakeSettings{})

	p.ForTenant("t1")
	if got := p.ForTenant("t2"); got != nil {
		t.Fatalf("tenant t2 (owner-2, no rules) got %+v — cache leaked across owners", got)
	}
}
