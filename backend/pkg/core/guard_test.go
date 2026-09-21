package core_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/ToufiqQureshi/hakaishield/pkg/challenge"
	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/core"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"
)

// TestGuardChallengesCleanTrafficStrict: under PolicyStrict, a scoreless first request
// from a clean visitor is served the challenge page in place of the origin.
func TestGuardChallengesCleanTrafficStrict(t *testing.T) {
	store := tenant.NewStore()
	c, _ := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("origin"))
	}))
	defer target.Close()

	proxy, _ := core.NewOriginProxy(target.URL)
	store.Add("default", tenant.TenantConfig{
		Target: target.URL,
		Mode:   config.ModeEnforce,
		Policy: config.PolicyStrict,
	}, []string{"example.com"}, proxy)

	guard := core.NewGuard(store, c)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	rec := httptest.NewRecorder()

	guard.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from the challenge page, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "__hakaishield") {
		t.Fatalf("clean unscored traffic under PolicyStrict must be challenged, body=%q", rec.Body.String())
	}
}

// TestGuardAllowsCleanTrafficBalanced: under PolicyBalanced (the production default),
// clean traffic (score 0) is allowed to reach the origin with zero latency.
func TestGuardAllowsCleanTrafficBalanced(t *testing.T) {
	store := tenant.NewStore()
	c, _ := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("origin"))
	}))
	defer target.Close()

	proxy, _ := core.NewOriginProxy(target.URL)
	store.Add("default", tenant.TenantConfig{
		Target: target.URL,
		Mode:   config.ModeEnforce,
		Policy: config.PolicyBalanced,
	}, []string{"example.com"}, proxy)

	guard := core.NewGuard(store, c)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	rec := httptest.NewRecorder()

	guard.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from origin, got %d", rec.Code)
	}
	if rec.Body.String() != "origin" {
		t.Fatalf("clean traffic in balanced mode must reach origin; got body=%q", rec.Body.String())
	}
}

func TestGuardHealthzEndpoint(t *testing.T) {
	store := tenant.NewStore()
	c, _ := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")
	guard := core.NewGuard(store, c)

	req := httptest.NewRequest("GET", "http://any-host/__hakaishield/healthz", nil)
	rec := httptest.NewRecorder()

	guard.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("healthz: want 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("healthz: expected ok status, got %s", rec.Body.String())
	}
}

// TestGuardForwardsPassedTraffic: a visitor who already solved the
// challenge is forwarded to the origin without re-scoring (guard.go).
func TestGuardForwardsPassedTraffic(t *testing.T) {
	store := tenant.NewStore()
	c, _ := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")

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
	passed := solveChallenge(t, c, "example.com")

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.AddCookie(passed)
	rec := httptest.NewRecorder()

	guard.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Body.String(); got != "origin" {
		t.Fatalf("a passed visitor must reach the origin; body=%q", got)
	}
}

func TestGuardBlocksMaliciousJA4(t *testing.T) {
	store := tenant.NewStore()
	c, _ := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")

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

// solvePoW returns the smallest counter whose SHA-256 with nonce starts
// with "00" — the 8-bit proof-of-work the challenge page's JS computes.
func solvePoW(nonce string) string {
	for i := 0; ; i++ {
		sum := sha256.Sum256([]byte(nonce + strconv.Itoa(i)))
		if hex.EncodeToString(sum[:])[:2] == "00" {
			return strconv.Itoa(i)
		}
	}
}

// solveChallenge drives the real GET-challenge/POST-verify flow to
// obtain a genuine "passed" cookie, the same way a real browser would,
// rather than reaching into challenge internals.
func solveChallenge(t *testing.T, c *challenge.Challenge, host string) *http.Cookie {
	t.Helper()
	h := c.Handler()

	getReq := httptest.NewRequest(http.MethodGet, "/__hakaishield/challenge", nil)
	getReq.Host = host
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, getReq)
	body := getRec.Body.String()

	tm := regexp.MustCompile(`token", "([^"]+)"`).FindStringSubmatch(body)
	// The page's JS feeds the nonce into its PoW loop (`encode("nonce" +
	// counter)`), so match the string literal, not a parenthesised call.
	nm := regexp.MustCompile(`encode\("([^"]+)"`).FindStringSubmatch(body)
	if tm == nil || nm == nil {
		t.Fatalf("could not extract token/nonce from challenge page: %s", body)
	}
	answer := solvePoW(nm[1])

	form := url.Values{}
	form.Set("token", tm[1])
	form.Set("answer", answer)
	form.Set("canvas", "data:image/png;base64,"+strings.Repeat("A", 150))
	postReq := httptest.NewRequest(http.MethodPost, "/__hakaishield/verify", strings.NewReader(form.Encode()))
	postReq.Host = host
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postRec := httptest.NewRecorder()
	h.ServeHTTP(postRec, postReq)

	for _, ck := range postRec.Result().Cookies() {
		if ck.Name == "X-HakaiShield-Passed" {
			return ck
		}
	}
	t.Fatalf("verify did not set a passed cookie, status %d", postRec.Code)
	return nil
}

// TestGuardVelocityLimitsPassedSession: a challenge solve proves a
// client can run JS once. It must not buy unlimited-speed access to
// the origin afterward — CLAUDE.md Section 15/18 (bounded resource
// use). Without the velocity check in the Passed(r) branch of
// guard.go, this test's flood would sail through as 200s forever.
func TestGuardVelocityLimitsPassedSession(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	defer mr.Close()
	signals.InitRedis(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	defer signals.InitRedis(nil)

	store := tenant.NewStore()
	c, _ := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	proxy, _ := core.NewOriginProxy(target.URL)
	store.Add("default", tenant.TenantConfig{Target: target.URL, Mode: config.ModeEnforce}, []string{"example.com"}, proxy)

	guard := core.NewGuard(store, c)
	passedCookie := solveChallenge(t, c, "example.com")

	makeReq := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "http://example.com/", nil)
		req.AddCookie(passedCookie)
		rec := httptest.NewRecorder()
		guard.ServeHTTP(rec, req)
		return rec
	}

	// A handful of requests right after passing must still go through —
	// the point is to stop a flood, not to punish normal browsing.
	for i := 0; i < 3; i++ {
		if rec := makeReq(); rec.Code != http.StatusOK {
			t.Fatalf("request %d: want 200 before flooding, got %d", i, rec.Code)
		}
	}

	// Flood past the velocity threshold from the same passed session.
	var lastCode int
	for i := 0; i < 25; i++ {
		lastCode = makeReq().Code
	}
	if lastCode != http.StatusTooManyRequests {
		t.Fatalf("want 429 after flooding a passed session, got %d", lastCode)
	}
}

func TestGuardDeceptionMode(t *testing.T) {
	store := tenant.NewStore()
	c, _ := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")

	var receivedDecision string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedDecision = r.Header.Get("X-HakaiShield-Decision")
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
		t.Errorf("expected origin to receive X-HakaiShield-Decision: deceive, got %q", receivedDecision)
	}
}

// TestHoneypotTrapEndToEnd walks the whole loop the feature exists for:
// a deceived visitor is served HTML carrying the invisible trap link,
// fetching that link is recorded, and the recorded trip then shows up
// in the visitor's evidence on their next request.
func TestHoneypotTrapEndToEnd(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body><h1>prices</h1></body></html>"))
	}))
	defer origin.Close()

	store := tenant.NewStore()
	proxy, _ := core.NewOriginProxy(origin.URL)
	c, _ := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")
	store.Add("acme", tenant.TenantConfig{
		Target:    origin.URL,
		Mode:      config.ModeEnforce,
		Deception: true,
	}, []string{"example.com"}, proxy)

	guard := core.NewGuard(store, c)

	// Step 1: a scraper gets the deceived page and is handed the bait.
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.RemoteAddr = "203.0.113.9:44321"
	ctx := core.WithJA4(req.Context(), "t12d190800_4464c1bd5eb7_b3394627b738") // known Python requests
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req.WithContext(ctx))

	body := rec.Body.String()
	if !strings.Contains(body, signals.HoneypotPath) {
		t.Fatalf("deceived HTML should carry the trap link, got:\n%s", body)
	}

	// Step 2: only something walking the DOM follows that link.
	trapReq := httptest.NewRequest("GET", "http://example.com"+signals.HoneypotPath, nil)
	trapReq.RemoteAddr = "203.0.113.9:44322"
	trapCtx := core.WithJA4(trapReq.Context(), "t12d190800_4464c1bd5eb7_b3394627b738")
	trapRec := httptest.NewRecorder()
	guard.ServeHTTP(trapRec, trapReq.WithContext(trapCtx))

	if trapRec.Code != http.StatusNotFound {
		t.Errorf("the trap should give a crawler nothing back: got %d, want 404", trapRec.Code)
	}

	// Step 3: the trip is now evidence against that caller.
	if !signals.HoneypotTripped("acme", "203.0.113.9", "t12d190800_4464c1bd5eb7_b3394627b738") {
		t.Fatal("fetching the trap path should have been recorded against the caller")
	}

	// ...and only against that caller, not the whole tenant.
	if signals.HoneypotTripped("acme", "203.0.113.10", "t12d190800_4464c1bd5eb7_b3394627b738") {
		t.Error("the trip must not spill onto other visitors of the same tenant")
	}
}

// TestHoneypotTrapIgnoresUnknownHosts: the trap records into detection
// state, so it must sit behind the tenant lookup. Otherwise anyone
// pointing a DNS record at the service could write into it for free.
func TestHoneypotTrapIgnoresUnknownHosts(t *testing.T) {
	store := tenant.NewStore()
	c, _ := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")
	guard := core.NewGuard(store, c)

	req := httptest.NewRequest("GET", "http://not-a-customer.example"+signals.HoneypotPath, nil)
	req.RemoteAddr = "203.0.113.11:1234"
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req.WithContext(core.WithJA4(req.Context(), "t13d1516h2_8daaf6152771_e5627efa2ab1")))

	if rec.Code != 421 {
		t.Errorf("an unknown host should be refused before the trap runs: got %d, want 421", rec.Code)
	}
	if signals.HoneypotTripped("", "203.0.113.11", "t13d1516h2_8daaf6152771_e5627efa2ab1") {
		t.Error("a request for a host we do not serve must not reach detection state")
	}
}
