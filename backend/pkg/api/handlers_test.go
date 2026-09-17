package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ToufiqQureshi/bot-shield/pkg/config"
	"github.com/ToufiqQureshi/bot-shield/pkg/signals"
	"github.com/ToufiqQureshi/bot-shield/pkg/tenant"
)

func TestDashboardStatsHandler(t *testing.T) {
	store := tenant.NewStore()
	store.Add("default", tenant.TenantConfig{Mode: config.ModeEnforce}, []string{"*"}, nil)

	// Add some dummy stats
	ten, _ := store.GetByID("default")
	ten.Stats.Record(signals.DecisionAllow)
	ten.Stats.Record(signals.DecisionBlock)

	handler := DashboardStatsHandler(store)

	t.Run("valid tenant", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?tenant=default", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp statsResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.TotalRequests != 2 {
			t.Errorf("expected 2 total requests, got %d", resp.TotalRequests)
		}
		if resp.Passed != 1 {
			t.Errorf("expected 1 passed request, got %d", resp.Passed)
		}
		if resp.Challenged != 0 {
			t.Errorf("expected 0 challenged requests, got %d", resp.Challenged)
		}
		if resp.Blocked != 1 {
			t.Errorf("expected 1 blocked request, got %d", resp.Blocked)
		}
	})

	t.Run("missing tenant defaults to default", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})

	t.Run("unknown tenant", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?tenant=unknown", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405, got %d", w.Code)
		}
	})
	
	t.Run("options method", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if w.Header().Get("Access-Control-Allow-Methods") == "" {
			t.Fatal("expected CORS headers")
		}
	})
}

func TestDashboardEvidenceHandler(t *testing.T) {
	store := tenant.NewStore()
	store.Add("default", tenant.TenantConfig{Mode: config.ModeEnforce, EvidenceToken: "secret"}, []string{"*"}, nil)

	handler := DashboardEvidenceHandler(store)

	t.Run("valid token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?tenant=default", nil)
		req.Header.Set("Authorization", "Bearer secret")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?tenant=default", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", w.Code)
		}
	})

	t.Run("missing token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?tenant=default", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", w.Code)
		}
	})

	t.Run("unknown tenant", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?tenant=unknown", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})
}
