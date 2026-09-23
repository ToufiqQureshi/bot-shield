package tenant_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"sync"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/challenge"
	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/core"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"
)

// helper to build a live httptest origin + its reverse proxy
func newOrigin(t *testing.T) (string, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv.URL, srv
}

func newStore(t *testing.T) *tenant.Store {
	t.Helper()
	store := tenant.NewStore()
	urlA, _ := newOrigin(t)
	urlB, _ := newOrigin(t)

	proxyA, _ := core.NewOriginProxy(urlA)
	proxyB, _ := core.NewOriginProxy(urlB)

	store.Add("a", tenant.TenantConfig{Target: urlA, Mode: config.ModeEnforce, Policy: config.PolicyStrict, EvidenceToken: "tok-a"}, []string{"a.example.com"}, proxyA)
	store.Add("b", tenant.TenantConfig{Target: urlB, Mode: config.ModeEnforce, Policy: config.PolicyStrict, EvidenceToken: "tok-b"}, []string{"b.example.com"}, proxyB)
	return store
}

func TestGetByID_NotFound(t *testing.T) {
	store := tenant.NewStore()
	_, err := store.GetByID("does-not-exist")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetByHost_Wildcard(t *testing.T) {
	store := tenant.NewStore()
	originURL, _ := newOrigin(t)
	proxy, _ := core.NewOriginProxy(originURL)
	store.Add("default", tenant.TenantConfig{Target: originURL, Mode: config.ModeEnforce}, []string{"*"}, proxy)

	got, err := store.GetByHost("anything.io")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "default" {
		t.Errorf("got tenant %q, want default", got.ID)
	}
}

func TestGetByHost_LoadsDatabaseTenantBeforeWildcard(t *testing.T) {
	store := tenant.NewStore()
	defaultURL, _ := newOrigin(t)
	customerURL, _ := newOrigin(t)
	defaultProxy, _ := core.NewOriginProxy(defaultURL)
	customerProxy, _ := core.NewOriginProxy(customerURL)
	store.Add("default", tenant.TenantConfig{Target: defaultURL, Mode: config.ModeEnforce}, []string{"*"}, defaultProxy)
	store.ProxyFactory = func(target string) (*httputil.ReverseProxy, error) {
		if target != customerURL {
			t.Fatalf("proxy factory target = %q, want customer origin", target)
		}
		return customerProxy, nil
	}
	store.TenantLoader = func(_ context.Context, host string) (string, string, string, string, string, string, error) {
		if host != "customer.example.com" {
			t.Fatalf("tenant loader host = %q, want customer.example.com", host)
		}
		return "customer", customerURL, "enforce", "", tenant.StatusActive, "owner-1", nil
	}

	got, err := store.GetByHost("customer.example.com")
	if err != nil {
		t.Fatalf("GetByHost: %v", err)
	}
	if got.ID != "customer" {
		t.Fatalf("GetByHost returned %q, want database tenant", got.ID)
	}
	if got.Config.OwnerUserID != "owner-1" {
		t.Fatalf("Config.OwnerUserID = %q, want owner-1 — a policy provider keys off this field", got.Config.OwnerUserID)
	}
}

func TestGetByHostRejectsPendingDatabaseTenantBeforeWildcard(t *testing.T) {
	store := tenant.NewStore()
	defaultURL, _ := newOrigin(t)
	pendingURL, _ := newOrigin(t)
	defaultProxy, _ := core.NewOriginProxy(defaultURL)
	pendingProxy, _ := core.NewOriginProxy(pendingURL)
	store.Add("default", tenant.TenantConfig{Target: defaultURL, Mode: config.ModeEnforce}, []string{"*"}, defaultProxy)
	store.ProxyFactory = func(target string) (*httputil.ReverseProxy, error) {
		if target != pendingURL {
			t.Fatalf("proxy factory target = %q, want pending origin", target)
		}
		return pendingProxy, nil
	}
	store.TenantLoader = func(_ context.Context, host string) (string, string, string, string, string, string, error) {
		if host != "pending.example.com" {
			t.Fatalf("tenant loader host = %q, want pending.example.com", host)
		}
		return "pending", pendingURL, "enforce", "", "pending_verification", "owner-1", nil
	}

	got, err := store.GetByHost("pending.example.com")
	if err == nil {
		t.Fatalf("pending domain routed to tenant %q; want not found", got.ID)
	}
	if got, err := store.GetByID("pending"); err != nil || got.ID != "pending" {
		t.Fatalf("pending tenant should still be readable by ID for dashboard state, got tenant=%v err=%v", got, err)
	}
}

func TestGetByHostNegativeCachesUnknownDatabaseHost(t *testing.T) {
	store := tenant.NewStore()
	store.ProxyFactory = func(target string) (*httputil.ReverseProxy, error) {
		t.Fatalf("proxy factory should not run when loader misses, got target %q", target)
		return nil, nil
	}
	var calls int
	store.TenantLoader = func(_ context.Context, host string) (string, string, string, string, string, string, error) {
		if host != "missing.example.com" {
			t.Fatalf("tenant loader host = %q, want missing.example.com", host)
		}
		calls++
		return "", "", "", "", "", "", errors.New("missing")
	}

	for i := 0; i < 100; i++ {
		if _, err := store.GetByHost("missing.example.com"); err == nil {
			t.Fatal("unknown host unexpectedly resolved")
		}
	}
	if calls != 1 {
		t.Fatalf("unknown host should hit the loader once while negative-cached, got %d calls", calls)
	}
}

func TestAddRejectsCrossTenantHostMapping(t *testing.T) {
	store := tenant.NewStore()
	originURL, _ := newOrigin(t)
	proxy, _ := core.NewOriginProxy(originURL)
	if err := store.Add("a", tenant.TenantConfig{Target: originURL, Mode: config.ModeEnforce}, []string{"Example.COM:443"}, proxy); err != nil {
		t.Fatalf("add a: %v", err)
	}
	if err := store.Add("b", tenant.TenantConfig{Target: originURL, Mode: config.ModeEnforce}, []string{"example.com"}, proxy); err == nil {
		t.Fatal("same canonical host must not map to two tenants")
	}
}

// TestTenantIsolation: data recorded for tenant A must never appear on tenant B.
func TestTenantIsolation(t *testing.T) {
	store := newStore(t)
	c, _ := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")
	guard := core.NewGuard(store, c)

	// send 5 requests to tenant A
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "http://a.example.com/", nil)
		guard.ServeHTTP(httptest.NewRecorder(), req)
	}

	tenA, _ := store.GetByID("a")
	tenB, _ := store.GetByID("b")

	if got := tenA.Stats.Total(); got != 5 {
		t.Errorf("tenant A: want 5 total, got %d", got)
	}
	// Clean traffic is challenged, not allowed: the interstitial is
	// mandatory for all traffic (docs/DECISIONS.md, 2026-09-19), so these
	// scoreless requests are counted as challenged.
	if got := tenA.Stats.Challenged(); got != 5 {
		t.Errorf("tenant A: want 5 challenged, got %d", got)
	}
	// Tenant B must be untouched
	if got := tenB.Stats.Total(); got != 0 {
		t.Errorf("tenant B contaminated: want 0 total, got %d", got)
	}
	if got := len(tenB.Trail.Recent(10)); got != 0 {
		t.Errorf("tenant B trail contaminated: want 0 records, got %d", got)
	}
}

// TestTenantIsolationConcurrent: concurrent traffic to two tenants must never cross.
func TestTenantIsolationConcurrent(t *testing.T) {
	store := newStore(t)
	c, _ := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")
	guard := core.NewGuard(store, c)

	const workers, perWorker = 20, 100
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			host := "a.example.com"
			if id%2 == 0 {
				host = "b.example.com"
			}
			for j := 0; j < perWorker; j++ {
				req := httptest.NewRequest("GET", "http://"+host+"/", nil)
				guard.ServeHTTP(httptest.NewRecorder(), req)
			}
		}(i)
	}
	wg.Wait()

	tenA, _ := store.GetByID("a")
	tenB, _ := store.GetByID("b")
	expected := int64((workers / 2) * perWorker)

	if got := tenA.Stats.Total(); got != expected {
		t.Errorf("tenant A: want %d, got %d", expected, got)
	}
	if got := tenB.Stats.Total(); got != expected {
		t.Errorf("tenant B: want %d, got %d", expected, got)
	}
}

// TestStatsRecordDecisions: each decision type increments the correct counter.
func TestStatsRecordDecisions(t *testing.T) {
	store := newStore(t)
	tenA, _ := store.GetByID("a")

	tenA.Stats.Record(signals.DecisionAllow)
	tenA.Stats.Record(signals.DecisionAllow)
	tenA.Stats.Record(signals.DecisionChallenge)
	tenA.Stats.Record(signals.DecisionBlock)

	if got := tenA.Stats.Total(); got != 4 {
		t.Errorf("total: want 4, got %d", got)
	}
	if got := tenA.Stats.Passed(); got != 2 {
		t.Errorf("passed: want 2, got %d", got)
	}
	if got := tenA.Stats.Challenged(); got != 1 {
		t.Errorf("challenged: want 1, got %d", got)
	}
	if got := tenA.Stats.Blocked(); got != 1 {
		t.Errorf("blocked: want 1, got %d", got)
	}
}
