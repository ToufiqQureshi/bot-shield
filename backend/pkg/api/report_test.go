package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/api"
	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/core"
	"github.com/ToufiqQureshi/hakaishield/pkg/evidence"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"
)

// newTestStore builds a store with one "default" tenant pointing at a local test server.
func newTestStore(t *testing.T, token string) *tenant.Store {
	t.Helper()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(origin.Close)

	proxy, _ := core.NewOriginProxy(origin.URL)
	store := tenant.NewStore()
	store.Add("default", tenant.TenantConfig{
		Target:        origin.URL,
		Mode:          config.ModeEnforce,
		EvidenceToken: token,
	}, []string{"*"}, proxy)
	return store
}

// TestDashboardStatsHandler_Success verifies the stats endpoint returns correct counters.
func TestDashboardStatsHandler_Success(t *testing.T) {
	store := newTestStore(t, "")

	ten, _ := store.GetByID("default")
	ten.Stats.Record(signals.DecisionAllow)
	ten.Stats.Record(signals.DecisionAllow)
	ten.Stats.Record(signals.DecisionBlock)

	req := httptest.NewRequest("GET", "/api/v1/dashboard/stats?tenant=default", nil)
	rec := httptest.NewRecorder()
	api.DashboardStatsHandler(store).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}

	var resp struct {
		TotalRequests int64  `json:"total_requests"`
		Passed        int64  `json:"passed"`
		Blocked       int64  `json:"blocked"`
		Mode          string `json:"mode"`
		Enforcing     bool   `json:"enforcing"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.TotalRequests != 3 {
		t.Errorf("total_requests: want 3, got %d", resp.TotalRequests)
	}
	if resp.Passed != 2 {
		t.Errorf("passed: want 2, got %d", resp.Passed)
	}
	if resp.Blocked != 1 {
		t.Errorf("blocked: want 1, got %d", resp.Blocked)
	}
	if !resp.Enforcing {
		t.Error("enforcing: want true")
	}
}

// TestDashboardStatsHandler_UnknownTenant verifies 404 for a missing tenant.
func TestDashboardStatsHandler_UnknownTenant(t *testing.T) {
	store := newTestStore(t, "")
	req := httptest.NewRequest("GET", "/api/v1/dashboard/stats?tenant=ghost", nil)
	rec := httptest.NewRecorder()
	api.DashboardStatsHandler(store).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

// TestDashboardStatsHandler_MethodNotAllowed verifies POST is rejected.
func TestDashboardStatsHandler_MethodNotAllowed(t *testing.T) {
	store := newTestStore(t, "")
	req := httptest.NewRequest("POST", "/api/v1/dashboard/stats?tenant=default", nil)
	rec := httptest.NewRecorder()
	api.DashboardStatsHandler(store).ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("want 405, got %d", rec.Code)
	}
}

// TestDashboardTopOffendersHandler_Success verifies the top offenders are ranked by total requests.
func TestDashboardTopOffendersHandler_Success(t *testing.T) {
	store := newTestStore(t, "secret-token")
	ten, _ := store.GetByID("default")

	// ja4-1 appears 2 times, ja4-2 appears 3 times — ja4-2 should be first.
	ten.Trail.Record(evidence.Evidence{JA4: "ja4-1", Decision: signals.DecisionAllow.String(), Signals: []string{"none"}})
	ten.Trail.Record(evidence.Evidence{JA4: "ja4-1", Decision: signals.DecisionBlock.String(), Signals: []string{"bad"}})
	ten.Trail.Record(evidence.Evidence{JA4: "ja4-2", Decision: signals.DecisionChallenge.String(), Signals: []string{"sus"}})
	ten.Trail.Record(evidence.Evidence{JA4: "ja4-2", Decision: signals.DecisionChallenge.String(), Signals: []string{"sus"}})
	ten.Trail.Record(evidence.Evidence{JA4: "ja4-2", Decision: signals.DecisionBlock.String(), Signals: []string{"very_sus"}})

	req := httptest.NewRequest("GET", "/api/v1/dashboard/offenders?tenant=default", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	api.DashboardTopOffendersHandler(store).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}

	var offenders []struct {
		JA4     string `json:"ja4"`
		Total   int    `json:"total"`
		Blocked int    `json:"blocked"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&offenders); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(offenders) != 2 {
		t.Fatalf("want 2 offenders, got %d", len(offenders))
	}
	if offenders[0].JA4 != "ja4-2" {
		t.Errorf("top offender: want ja4-2, got %s", offenders[0].JA4)
	}
	if offenders[0].Total != 3 {
		t.Errorf("ja4-2 total: want 3, got %d", offenders[0].Total)
	}
}

// TestDashboardTopOffendersHandler_Unauthorized verifies the token is enforced.
func TestDashboardTopOffendersHandler_Unauthorized(t *testing.T) {
	store := newTestStore(t, "secret-token")
	req := httptest.NewRequest("GET", "/api/v1/dashboard/offenders?tenant=default", nil)
	// No Authorization header
	rec := httptest.NewRecorder()
	api.DashboardTopOffendersHandler(store).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}
