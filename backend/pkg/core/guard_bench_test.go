package core_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/ToufiqQureshi/bot-shield/pkg/challenge"
	"github.com/ToufiqQureshi/bot-shield/pkg/config"
	"github.com/ToufiqQureshi/bot-shield/pkg/core"
	"github.com/ToufiqQureshi/bot-shield/pkg/tenant"
)

func newBenchGuard(b *testing.B) (http.Handler, *httptest.Server) {
	b.Helper()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	store := tenant.NewStore()
	proxy, _ := core.NewOriginProxy(origin.URL)
	store.Add("default", tenant.TenantConfig{
		Target: origin.URL,
		Mode:   config.ModeEnforce,
	}, []string{"*"}, proxy)
	c, _ := challenge.NewChallenge()
	return core.NewGuard(store, c), origin
}

// BenchmarkGuardServeHTTP measures request throughput through the full guard pipeline.
func BenchmarkGuardServeHTTP(b *testing.B) {
	guard, origin := newBenchGuard(b)
	defer origin.Close()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "http://test.local/", nil)
			rec := httptest.NewRecorder()
			guard.ServeHTTP(rec, req)
		}
	})
}

// BenchmarkGuardHighConcurrency simulates 500 concurrent goroutines hitting the proxy.
func BenchmarkGuardHighConcurrency(b *testing.B) {
	guard, origin := newBenchGuard(b)
	defer origin.Close()
	const concurrency = 500
	b.ResetTimer()

	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			req := httptest.NewRequest("GET", "http://test.local/resource", nil)
			rec := httptest.NewRecorder()
			guard.ServeHTTP(rec, req)
		}()
	}
	wg.Wait()
}

// TestGuardUnknownTenant verifies that requests to unregistered hosts return 404.
func TestGuardUnknownTenant(t *testing.T) {
	store := tenant.NewStore()
	c, _ := challenge.NewChallenge()
	guard := core.NewGuard(store, c)

	req := httptest.NewRequest("GET", "http://unknown-host.io/", nil)
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req)

	// Guard returns 421 Misdirected Request for unrecognised hosts (RFC 7540 §9.1.2).
	if rec.Code != 421 {
		t.Errorf("want 421 Misdirected Request, got %d", rec.Code)
	}
}

// TestGuardShadowModeDoesNotBlock verifies shadow mode never blocks traffic.
func TestGuardShadowModeDoesNotBlock(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer origin.Close()

	store := tenant.NewStore()
	proxy, _ := core.NewOriginProxy(origin.URL)
	store.Add("default", tenant.TenantConfig{
		Target: origin.URL,
		Mode:   config.ModeShadow,
	}, []string{"shadow.local"}, proxy)

	c, _ := challenge.NewChallenge()
	guard := core.NewGuard(store, c)

	// Even with a suspicious-looking (unreadable JA4) request, shadow mode must pass.
	req := httptest.NewRequest("GET", "http://shadow.local/", nil)
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("shadow mode must never block: got %d", rec.Code)
	}
}
