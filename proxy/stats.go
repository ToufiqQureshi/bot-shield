package proxy

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
)

// Stats counts what Guard has decided, for the dashboard (ROADMAP
// item 12). In-memory only - resets on restart, same class of
// limitation as Challenge's in-memory secret (docs/DECISIONS.md).
// Real durable analytics need the planned Postgres store.
type Stats struct {
	// Mode is reported alongside the counts so nobody can read them
	// without knowing whether they describe what happened or only what
	// would have happened (docs/ROADMAP.md item 18).
	Mode Mode

	total      atomic.Int64
	passed     atomic.Int64
	challenged atomic.Int64
	blocked    atomic.Int64
}

func (s *Stats) record(d Decision) {
	s.total.Add(1)
	switch d {
	case DecisionBlock:
		s.blocked.Add(1)
	case DecisionChallenge:
		s.challenged.Add(1)
	default:
		s.passed.Add(1)
	}
}

type statsResponse struct {
	TotalRequests int64 `json:"total_requests"`
	Passed        int64 `json:"passed"`
	Challenged    int64 `json:"challenged"`
	Blocked       int64 `json:"blocked"`
	// Mode and Enforcing say whether the counts above describe traffic
	// that was really allowed/challenged/blocked, or only scored.
	Mode      string `json:"mode"`
	Enforcing bool   `json:"enforcing"`
}

// Handler serves the dashboard's stats endpoint. Shape and field
// names match the contract Antigravity posted in
// agentchat/chat.jsonl on 2026-09-15: GET /api/v1/dashboard/stats ->
// {total_requests, passed, challenged, blocked}. CORS is open on this
// one read-only, aggregate-only endpoint (no per-visitor data) so the
// dashboard's own dev server (a different origin) can call it
// directly without a proxy in front of it yet.
func (s *Stats) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(statsResponse{
			TotalRequests: s.total.Load(),
			Passed:        s.passed.Load(),
			Challenged:    s.challenged.Load(),
			Blocked:       s.blocked.Load(),
			Mode:          s.Mode.String(),
			Enforcing:     s.Mode == ModeEnforce,
		})
	})
}
