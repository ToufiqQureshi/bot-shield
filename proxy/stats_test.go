package proxy

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatsHandlerShape(t *testing.T) {
	s := &Stats{}
	s.record(DecisionAllow)
	s.record(DecisionAllow)
	s.record(DecisionChallenge)
	s.record(DecisionBlock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/stats", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var got statsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response wasn't valid JSON matching the contract: %v (body: %s)", err, rec.Body.String())
	}
	want := statsResponse{TotalRequests: 4, Passed: 2, Challenged: 1, Blocked: 1, Mode: "enforce", Enforcing: true}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// Field names are the actual API contract with the dashboard
// - assert the raw JSON keys, not
// just that the Go struct round-trips through itself.
func TestStatsHandlerFieldNamesMatchContract(t *testing.T) {
	s := &Stats{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/stats", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, field := range []string{"total_requests", "passed", "challenged", "blocked"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("response is missing contract field %q, got %v", field, raw)
		}
	}
}

func TestStatsHandlerRejectsWrongMethod(t *testing.T) {
	s := &Stats{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dashboard/stats", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestStatsHandlerSetsCORSForDashboard(t *testing.T) {
	s := &Stats{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/stats", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want \"*\"", got)
	}
}

// Guard must actually count outcomes, not just decide them - this is
// what the dashboard's numbers depend on being real.
func TestGuardRecordsStats(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer origin.Close()

	gws, stats := startGuardWithStats(t, origin.URL)

	// Allowed: plain HTTP, no signal possible.
	resp, err := http.Get("http://" + gws.addr + "/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	statsReq := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/stats", nil)
	statsRec := httptest.NewRecorder()
	stats.Handler().ServeHTTP(statsRec, statsReq)
	var got statsResponse
	json.Unmarshal(statsRec.Body.Bytes(), &got)
	if got.TotalRequests != 1 || got.Passed != 1 {
		t.Fatalf("after one allowed plain-HTTP request, stats = %+v, want total=1 passed=1", got)
	}
}

type guardWithStats struct {
	addr  string
	stats *Stats
}

func startGuardWithStats(t *testing.T, origin string) (guardWithStats, *Stats) {
	t.Helper()
	p, err := New(origin)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	challenge, err := NewChallenge()
	if err != nil {
		t.Fatalf("NewChallenge: %v", err)
	}
	stats := &Stats{}
	guard := NewGuard(p, challenge, stats, NewTrail(), ModeEnforce)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: guard, ErrorLog: quietLogger()}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })

	return guardWithStats{addr: ln.Addr().String(), stats: stats}, stats
}

// The counts are meaningless without knowing whether they describe
// enforced decisions or shadow-mode ones, so the mode travels with
// them on every response.
func TestStatsHandlerReportsMode(t *testing.T) {
	for _, tc := range []struct {
		mode          Mode
		wantMode      string
		wantEnforcing bool
	}{
		{ModeEnforce, "enforce", true},
		{ModeShadow, "shadow", false},
	} {
		s := &Stats{Mode: tc.mode}
		s.record(DecisionBlock)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/stats", nil)
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)

		var got statsResponse
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if got.Mode != tc.wantMode {
			t.Errorf("mode = %q, want %q", got.Mode, tc.wantMode)
		}
		if got.Enforcing != tc.wantEnforcing {
			t.Errorf("enforcing = %v, want %v", got.Enforcing, tc.wantEnforcing)
		}
		if got.Blocked != 1 {
			t.Errorf("blocked = %d, want 1 — shadow mode still counts what it would have done", got.Blocked)
		}
	}
}

// Stats.Mode and Guard.mode were two sources of truth for the same
// fact: a Guard built in shadow mode with a default Stats reported
// "enforcing" while enforcing nothing. Guard now sets it, so the
// number a client reads and the behaviour they get cannot disagree.
func TestNewGuardSetsStatsMode(t *testing.T) {
	p, err := New("http://127.0.0.1:1")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	challenge, err := NewChallenge()
	if err != nil {
		t.Fatalf("NewChallenge: %v", err)
	}

	stats := &Stats{}
	NewGuard(p, challenge, stats, NewTrail(), ModeShadow)

	if stats.Mode != ModeShadow {
		t.Fatalf("stats.Mode = %v after NewGuard(..., ModeShadow), want shadow — the dashboard would claim enforcement that isn't happening", stats.Mode)
	}
}
