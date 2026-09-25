package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/api"
	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"
)

func TestDashboardStatsHandler(t *testing.T) {
	store := tenant.NewStore()
	store.Add("default", tenant.TenantConfig{Mode: config.ModeEnforce}, []string{"*"}, nil)

	// Add some dummy stats
	ten, _ := store.GetByID("default")
	ten.Stats.Record(signals.DecisionAllow)
	ten.Stats.Record(signals.DecisionBlock)

	ta := newTestAuth(t)
	token := ta.sign(t, "test-user")
	handler := api.DashboardStatsHandler(store, ta.verifier(t))

	t.Run("missing token is rejected", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?tenant=default", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 without bearer token, got %d", w.Code)
		}
	})

	t.Run("valid tenant", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?tenant=default", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp struct {
			TotalRequests     int64 `json:"total_requests"`
			Passed            int64 `json:"passed"`
			Challenged        int64 `json:"challenged"`
			Blocked           int64 `json:"blocked"`
			EgressBytes       int64 `json:"egress_bytes"`
			ChallengeSolves   int64 `json:"challenge_solves"`
			ChallengeFailures int64 `json:"challenge_failures"`
		}
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
		// P1 measurement fields must be present and numeric; a missing
		// field decodes as 0, which is indistinguishable from "no traffic
		// served" and would hide a wiring bug.
		ten.Stats.RecordEgressBytes(1234)
		ten.Stats.RecordChallengeSolved()
		ten.Stats.RecordChallengeFailed()
		w2 := httptest.NewRecorder()
		handler.ServeHTTP(w2, req)
		var resp2 struct {
			EgressBytes       int64 `json:"egress_bytes"`
			ChallengeSolves   int64 `json:"challenge_solves"`
			ChallengeFailures int64 `json:"challenge_failures"`
		}
		if err := json.NewDecoder(w2.Body).Decode(&resp2); err != nil {
			t.Fatalf("failed to decode measurement response: %v", err)
		}
		if resp2.EgressBytes != 1234 || resp2.ChallengeSolves != 1 || resp2.ChallengeFailures != 1 {
			t.Errorf("measurement fields wrong: %+v", resp2)
		}
	})

	t.Run("missing tenant defaults to default", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})

	t.Run("unknown tenant", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?tenant=unknown", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
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

	handler := api.DashboardEvidenceHandler(store)

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
