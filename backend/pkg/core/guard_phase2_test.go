package core_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/challenge"
	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/core"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"
)

// These tests prove the Phase 2 wiring end-to-end: the risk score the
// guard computes for a request must reach the challenge and raise the
// difficulty a visitor is actually served. Before this wiring existed,
// every challenge was issued at difficulty 1 and the whole adaptive
// feature was inert in the running service.

func phase2Guard(t *testing.T) *core.Guard {
	t.Helper()
	store := tenant.NewStore()
	c, err := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")
	if err != nil {
		t.Fatalf("NewChallenge: %v", err)
	}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	proxy, err := core.NewOriginProxy(target.URL)
	if err != nil {
		t.Fatalf("NewOriginProxy: %v", err)
	}
	store.Add("default", tenant.TenantConfig{
		Target: target.URL,
		Mode:   config.ModeEnforce,
	}, []string{"example.com"}, proxy)
	return core.NewGuard(store, c)
}

var difficultyRe = regexp.MustCompile(`var difficulty =\s*(\d+)`)

func servedDifficultyFromGuard(t *testing.T, req *http.Request) int {
	t.Helper()
	rec := httptest.NewRecorder()
	phase2Guard(t).ServeHTTP(rec, req)
	body := rec.Body.String()
	m := difficultyRe.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("guard did not serve a challenge page (status %d): %s", rec.Code, body)
	}
	d := 0
	for _, ch := range m[1] {
		d = d*10 + int(ch-'0')
	}
	return d
}

// TestGuardScoreRaisesChallengeDifficulty: a request that fired only the
// weak header_anomaly signal (score 25) must be served difficulty 2, not
// the lightest puzzle. If the guard stops passing the score into the
// challenge context, this test goes red with difficulty 1.
func TestGuardScoreRaisesChallengeDifficulty(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0) Chrome/120.0.0.0 Safari/537.36")
	// Browser-claiming UA with no Sec-Fetch-*/Sec-CH-UA headers fires
	// header_anomaly (25) and nothing else.
	req = req.WithContext(core.WithJA4(req.Context(), "t13d1516h2_8daaf6152771_e5627efa2ab1"))
	if got := servedDifficultyFromGuard(t, req); got != 2 {
		t.Errorf("score-25 request: served difficulty = %d, want 2", got)
	}
}

// TestGuardMismatchedBrowserServesHardestChallenge: a browser-claiming
// UA over a TLS 1.0 handshake fires ua_mismatch (50) — below the block
// bar, so it is challenged at the hardest difficulty. This is the score
// band the guard wiring must deliver to the challenge.
func TestGuardMismatchedBrowserServesHardestChallenge(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Firefox/120.0")
	// Sec-Fetch-Mode keeps header_anomaly out, so the score is exactly
	// the 50 from ua_mismatch.
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req = req.WithContext(core.WithJA4(req.Context(), "t10d1516h2_8daaf6152771_e5627efa2ab1"))
	if got := servedDifficultyFromGuard(t, req); got != 3 {
		t.Errorf("score-50 request: served difficulty = %d, want 3", got)
	}
}

// (Balanced-mode score-0 traffic is allowed without a challenge — that
// is TestGuardAllowsCleanTrafficBalanced. The difficulty-1 end-to-end
// case is covered by TestGuardStrictCleanTrafficStaysLightest, because
// strict is the only mode that challenges clean traffic.)

// TestGuardStrictCleanTrafficStaysLightest: under PolicyStrict even clean
// traffic is challenged, and a clean visitor must get the lightest puzzle
// — strictness must not silently raise the work we demand from real phones.
func TestGuardStrictCleanTrafficStaysLightest(t *testing.T) {
	store := tenant.NewStore()
	c, err := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")
	if err != nil {
		t.Fatalf("NewChallenge: %v", err)
	}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	proxy, err := core.NewOriginProxy(target.URL)
	if err != nil {
		t.Fatalf("NewOriginProxy: %v", err)
	}
	store.Add("default", tenant.TenantConfig{
		Target: target.URL,
		Mode:   config.ModeEnforce,
		Policy: config.PolicyStrict,
	}, []string{"example.com"}, proxy)
	guard := core.NewGuard(store, c)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req)
	body := rec.Body.String()
	m := difficultyRe.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("strict mode did not serve a challenge page: %s", strings.TrimSpace(body))
	}
	if m[1] != "1" {
		t.Errorf("clean traffic under strict: difficulty = %s, want 1", m[1])
	}
}

// Compile-time assertion the signals package still exports what the
// guard wiring depends on.
var _ = signals.DecisionChallenge
