package core_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ToufiqQureshi/bot-shield/pkg/challenge"
	"github.com/ToufiqQureshi/bot-shield/pkg/config"
	"github.com/ToufiqQureshi/bot-shield/pkg/core"
	"github.com/ToufiqQureshi/bot-shield/pkg/tenant"
)

func TestGuardPassesTraffic(t *testing.T) {
	store := tenant.NewStore()
	c, _ := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"))

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("origin"))
	}))
	defer target.Close()

	proxy, _ := core.NewOriginProxy(target.URL)
	store.Add("default", tenant.TenantConfig{
		Target: target.URL,
		Mode:   config.ModeEnforce,
	}, []string{"example.com"}, proxy)

	guard := core.NewGuard(store, c)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	rec := httptest.NewRecorder()

	guard.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rec.Code)
	}
}

func TestGuardBlocksMaliciousJA4(t *testing.T) {
	store := tenant.NewStore()
	c, _ := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"))

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	proxy, _ := core.NewOriginProxy(target.URL)
	store.Add("default", tenant.TenantConfig{
		Target:    target.URL,
		Mode:      config.ModeEnforce,
		Deception: false,
	}, []string{"example.com"}, proxy)

	guard := core.NewGuard(store, c)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	ctx := core.WithJA4(req.Context(), "t12d190800_4464c1bd5eb7_b3394627b738") // Known malicious Python requests
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden, got %d", rec.Code)
	}
}

func TestGuardDeceptionMode(t *testing.T) {
	store := tenant.NewStore()
	c, _ := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"))

	var receivedDecision string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedDecision = r.Header.Get("X-BotShield-Decision")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fake-decoy-data"))
	}))
	defer target.Close()

	proxy, _ := core.NewOriginProxy(target.URL)
	store.Add("default", tenant.TenantConfig{
		Target:    target.URL,
		Mode:      config.ModeEnforce,
		Deception: true, // Deception enabled!
	}, []string{"example.com"}, proxy)

	guard := core.NewGuard(store, c)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	ctx := core.WithJA4(req.Context(), "t12d190800_4464c1bd5eb7_b3394627b738") // Known malicious Python requests
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK from decoy response, got %d", rec.Code)
	}
	if receivedDecision != "deceive" {
		t.Errorf("expected origin to receive X-BotShield-Decision: deceive, got %q", receivedDecision)
	}
}
