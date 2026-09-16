package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const testToken = "s3cret-token"

func testTrail(t *testing.T, size int, maxAge time.Duration) *Trail {
	t.Helper()
	return newTrail(size, maxAge)
}

// The whole point of the trail is answering "why was this request
// stopped?", so a record has to come back with the real score, the
// real decision and the real signal names - not just "something was
// recorded".
func TestTrailRecordsWhatDecidedTheRequest(t *testing.T) {
	tr := testTrail(t, 10, time.Hour)
	tr.record(Evidence{JA4: "t13d1516h2_abc_def", Signals: []string{"ua_mismatch"}, Score: 50, Decision: "challenge"})

	got := tr.Recent(0)
	if len(got) != 1 {
		t.Fatalf("Recent returned %d records, want 1", len(got))
	}
	e := got[0]
	if e.JA4 != "t13d1516h2_abc_def" {
		t.Errorf("JA4 = %q, want the fingerprint that was scored", e.JA4)
	}
	if e.Score != 50 {
		t.Errorf("Score = %d, want 50", e.Score)
	}
	if e.Decision != "challenge" {
		t.Errorf("Decision = %q, want challenge", e.Decision)
	}
	if len(e.Signals) != 1 || e.Signals[0] != "ua_mismatch" {
		t.Errorf("Signals = %v, want [ua_mismatch]", e.Signals)
	}
	if e.Time.IsZero() {
		t.Error("Time is zero; a record with no timestamp can't be correlated with anything")
	}
}

// Newest first: an ops engineer chasing a complaint that just came in
// should not have to page through yesterday's traffic to find it.
func TestTrailReturnsNewestFirst(t *testing.T) {
	tr := testTrail(t, 10, time.Hour)
	tr.record(Evidence{JA4: "first", Decision: "allow"})
	tr.record(Evidence{JA4: "second", Decision: "allow"})
	tr.record(Evidence{JA4: "third", Decision: "allow"})

	got := tr.Recent(0)
	want := []string{"third", "second", "first"}
	if len(got) != len(want) {
		t.Fatalf("Recent returned %d records, want %d", len(got), len(want))
	}
	for i, ja4 := range want {
		if got[i].JA4 != ja4 {
			t.Errorf("Recent()[%d].JA4 = %q, want %q", i, got[i].JA4, ja4)
		}
	}
}

// This runs against live adversarial traffic: memory must not grow
// with request volume (CLAUDE.md Section 9). Once full, the oldest
// record is the one that goes.
func TestTrailNeverGrowsPastCapacity(t *testing.T) {
	tr := testTrail(t, 3, time.Hour)
	for _, ja4 := range []string{"a", "b", "c", "d", "e"} {
		tr.record(Evidence{JA4: ja4, Decision: "allow"})
	}

	got := tr.Recent(0)
	if len(got) != 3 {
		t.Fatalf("Recent returned %d records, want at most the capacity of 3", len(got))
	}
	want := []string{"e", "d", "c"}
	for i, ja4 := range want {
		if got[i].JA4 != ja4 {
			t.Errorf("Recent()[%d].JA4 = %q, want %q (oldest records should be gone)", i, got[i].JA4, ja4)
		}
	}
	if len(tr.buf) != 3 {
		t.Errorf("underlying buffer grew to %d, want a fixed 3", len(tr.buf))
	}
}

// A retention limit is the other half of Section 9 and of Section 18's
// "only what a decision needs": old visitor records must age out even
// when the buffer is nowhere near full.
func TestTrailDropsRecordsPastMaxAge(t *testing.T) {
	tr := testTrail(t, 10, time.Hour)
	base := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)

	tr.now = func() time.Time { return base }
	tr.record(Evidence{JA4: "old", Decision: "allow"})

	tr.now = func() time.Time { return base.Add(90 * time.Minute) }
	tr.record(Evidence{JA4: "fresh", Decision: "allow"})

	got := tr.Recent(0)
	if len(got) != 1 {
		t.Fatalf("Recent returned %d records, want only the one inside the retention window", len(got))
	}
	if got[0].JA4 != "fresh" {
		t.Errorf("Recent()[0].JA4 = %q, want the record inside the window", got[0].JA4)
	}
}

func TestTrailRecentLimit(t *testing.T) {
	tr := testTrail(t, 10, time.Hour)
	for _, ja4 := range []string{"a", "b", "c", "d"} {
		tr.record(Evidence{JA4: ja4, Decision: "allow"})
	}

	if got := tr.Recent(2); len(got) != 2 {
		t.Errorf("Recent(2) returned %d records, want 2", len(got))
	}
	if got := tr.Recent(0); len(got) != 4 {
		t.Errorf("Recent(0) returned %d records, want all 4", len(got))
	}
	if got := tr.Recent(-5); len(got) != 4 {
		t.Errorf("Recent(-5) returned %d records, want all 4", len(got))
	}
	if got := tr.Recent(99); len(got) != 4 {
		t.Errorf("Recent(99) returned %d records, want all 4 that exist", len(got))
	}
}

// Live traffic hits this from many connections at once; a torn write
// or a data race here would corrupt the record an ops engineer later
// relies on. Run with -race for this to mean anything.
func TestTrailConcurrentRecord(t *testing.T) {
	tr := testTrail(t, 50, time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tr.record(Evidence{JA4: "concurrent", Decision: "allow"})
			tr.Recent(5)
		}()
	}
	wg.Wait()

	if got := len(tr.Recent(0)); got != 50 {
		t.Errorf("Recent returned %d records after 100 concurrent writes, want the capacity of 50", got)
	}
}

func evidenceRequest(t *testing.T, tr *Trail, method, target, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	tr.Handler(testToken).ServeHTTP(rec, req)
	return rec
}

// This endpoint exposes per-visitor fingerprints and, worse, tells a
// caller whether their own fingerprint is being flagged - an evasion
// oracle if left open. It must refuse anyone without the token.
func TestEvidenceHandlerRefusesWithoutToken(t *testing.T) {
	tr := testTrail(t, 10, time.Hour)
	tr.record(Evidence{JA4: "secret-fingerprint", Decision: "block"})

	for _, tc := range []struct{ name, token string }{
		{"no token", ""},
		{"wrong token", "not-the-token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := evidenceRequest(t, tr, http.MethodGet, "/api/v1/dashboard/evidence", tc.token)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
			if body := rec.Body.String(); strings.Contains(body, "secret-fingerprint") {
				t.Fatalf("unauthorized response leaked a record: %s", body)
			}
		})
	}
}

// An empty configured token must not become "no auth required" - that
// is the failure mode where a deployment silently ships wide open.
func TestEvidenceHandlerRefusesWhenNoTokenConfigured(t *testing.T) {
	tr := testTrail(t, 10, time.Hour)
	tr.record(Evidence{JA4: "secret-fingerprint", Decision: "block"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/evidence", nil)
	rec := httptest.NewRecorder()
	tr.Handler("").ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 when no token is configured", rec.Code)
	}
	if body := rec.Body.String(); strings.Contains(body, "secret-fingerprint") {
		t.Fatalf("response leaked a record: %s", body)
	}
}

func TestEvidenceHandlerReturnsRecords(t *testing.T) {
	tr := testTrail(t, 10, time.Hour)
	tr.record(Evidence{JA4: "ja4-one", Signals: []string{"fragmented_handshake"}, Score: 50, Decision: "challenge"})
	tr.record(Evidence{JA4: "ja4-two", Signals: []string{"fragmented_handshake", "ua_mismatch"}, Score: 100, Decision: "block"})

	rec := evidenceRequest(t, tr, http.MethodGet, "/api/v1/dashboard/evidence", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var got []Evidence
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d records, want 2", len(got))
	}
	if got[0].JA4 != "ja4-two" || got[0].Score != 100 || got[0].Decision != "block" {
		t.Errorf("newest record = %+v, want the ja4-two block at score 100", got[0])
	}
	if len(got[0].Signals) != 2 {
		t.Errorf("newest record signals = %v, want both signals named", got[0].Signals)
	}
}

func TestEvidenceHandlerLimitParam(t *testing.T) {
	tr := testTrail(t, 10, time.Hour)
	for _, ja4 := range []string{"a", "b", "c"} {
		tr.record(Evidence{JA4: ja4, Decision: "allow"})
	}

	for _, tc := range []struct {
		query string
		want  int
	}{
		{"?limit=2", 2},
		{"?limit=0", 3},
		{"?limit=abc", 3},
		{"", 3},
	} {
		rec := evidenceRequest(t, tr, http.MethodGet, "/api/v1/dashboard/evidence"+tc.query, testToken)
		var got []Evidence
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("%q: decoding response: %v", tc.query, err)
		}
		if len(got) != tc.want {
			t.Errorf("%q returned %d records, want %d", tc.query, len(got), tc.want)
		}
	}
}

func TestEvidenceHandlerRejectsNonGET(t *testing.T) {
	tr := testTrail(t, 10, time.Hour)
	rec := evidenceRequest(t, tr, http.MethodPost, "/api/v1/dashboard/evidence", testToken)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

// Stats can afford open CORS because it is aggregate-only. This
// endpoint is per-visitor data behind a token, so a wildcard here
// would hand it to any page the visitor happens to be on.
func TestEvidenceHandlerSendsNoWildcardCORS(t *testing.T) {
	tr := testTrail(t, 10, time.Hour)
	rec := evidenceRequest(t, tr, http.MethodGet, "/api/v1/dashboard/evidence", testToken)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want it unset on per-visitor data", got)
	}
}

// A zero-size trail would divide by zero on its first record, and
// that panic would happen in the request path. Construction must not
// hand back a Trail that can do this.
func TestNewTrailRejectsZeroSize(t *testing.T) {
	tr := newTrail(0, time.Hour)
	tr.record(Evidence{JA4: "only", Decision: "allow"})

	got := tr.Recent(0)
	if len(got) != 1 || got[0].JA4 != "only" {
		t.Fatalf("Recent = %+v, want the single record a minimum-size trail holds", got)
	}
}
