package proxy

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// startGuard runs the real path a live request takes: TLS capture,
// Guard deciding, and (when allowed) the reverse proxy reaching a real
// origin — same shape as capture_test.go's startCapture, but through
// Guard instead of the bare proxy.
func startGuard(t *testing.T, origin string) (addr string, challenge *Challenge, trail *Trail) {
	return startGuardMode(t, origin, ModeEnforce)
}

func startGuardMode(t *testing.T, origin string, mode Mode) (addr string, challenge *Challenge, trail *Trail) {
	t.Helper()

	p, err := New(origin)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	challenge, err = NewChallenge()
	if err != nil {
		t.Fatalf("NewChallenge: %v", err)
	}
	trail = NewTrail()
	guard := NewGuard(p, challenge, &Stats{Mode: mode}, trail, mode)

	rawLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr = rawLn.Addr().String()

	cfg := &tls.Config{Certificates: []tls.Certificate{selfSignedCert(t)}}
	srv := &http.Server{
		Handler:           guard,
		ConnContext:       ConnContext,
		ReadHeaderTimeout: 10 * time.Second,
		ErrorLog:          quietLogger(),
	}
	go srv.Serve(NewCaptureListener(rawLn, cfg))
	t.Cleanup(func() { srv.Close() })

	return addr, challenge, trail
}

func tlsClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		Timeout:   5 * time.Second,
	}
}

// A real modern browser handshake with an honest User-Agent triggers
// no signal at all, so it must reach the real origin unchanged.
func TestGuardAllowsNormalBrowser(t *testing.T) {
	reached := make(chan bool, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached <- true
		w.Write([]byte("ok"))
	}))
	defer origin.Close()

	addr, _, trail := startGuard(t, origin.URL)

	req, _ := http.NewRequest(http.MethodGet, "https://"+addr+"/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0 Safari/537.36")
	resp, err := tlsClient().Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	select {
	case <-reached:
	default:
		t.Fatal("origin was never reached")
	}

	if e := lastEvidence(t, trail); e.Decision != "allow" || e.Score != 0 || len(e.Signals) != 0 {
		t.Errorf("evidence = %+v, want an allow at score 0 with no signals", e)
	}
}

// Fragmented handshake + a claimed browser UA is both signals firing
// (score 100) - must be blocked outright, and the origin must never
// see the request.
func TestGuardBlocksCombinedSignals(t *testing.T) {
	reached := make(chan bool, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached <- true
	}))
	defer origin.Close()

	guardAddr, _, trail := startGuard(t, origin.URL)
	addr := fragmentingRelay(t, guardAddr)

	req, _ := http.NewRequest(http.MethodGet, "https://"+addr+"/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0 Safari/537.36")
	resp, err := tlsClient().Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	select {
	case <-reached:
		t.Fatal("origin was reached, want the request blocked before it got there")
	default:
	}

	e := lastEvidence(t, trail)
	if e.Decision != "block" || e.Score != 100 {
		t.Errorf("evidence = %+v, want a block at score 100", e)
	}
	if !hasSignal(e, "fragmented_handshake") || !hasSignal(e, "ua_mismatch") {
		t.Errorf("evidence signals = %v, want both signals named as the reason", e.Signals)
	}
}

// A single mid-strength signal (fragmented handshake, but a UA that
// never claimed to be a browser, so UAMismatch doesn't also fire)
// must only earn a challenge, never a block on its own (CLAUDE.md
// Section 6) - and the origin must not see it either, since it hasn't
// proven anything yet.
func TestGuardChallengesSingleSignal(t *testing.T) {
	reached := make(chan bool, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached <- true
	}))
	defer origin.Close()

	guardAddr, _, trail := startGuard(t, origin.URL)
	addr := fragmentingRelay(t, guardAddr)

	req, _ := http.NewRequest(http.MethodGet, "https://"+addr+"/some-page", nil)
	req.Header.Set("User-Agent", "curl/8.6.0")
	resp, err := tlsClient().Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (challenge page is a normal 200 response)", resp.StatusCode)
	}
	if !containsChallengeMarker(string(body)) {
		t.Fatalf("response body doesn't look like the challenge page: %s", body)
	}
	select {
	case <-reached:
		t.Fatal("origin was reached, want it challenged instead")
	default:
	}

	e := lastEvidence(t, trail)
	if e.Decision != "challenge" || e.Score != 50 {
		t.Errorf("evidence = %+v, want a challenge at score 50", e)
	}
	if !hasSignal(e, "fragmented_handshake") {
		t.Errorf("evidence signals = %v, want fragmented_handshake named", e.Signals)
	}
	if hasSignal(e, "ua_mismatch") {
		t.Errorf("evidence signals = %v, want ua_mismatch absent - curl never claimed to be a browser", e.Signals)
	}
}

func containsChallengeMarker(body string) bool {
	return strings.Contains(body, "crypto.subtle.digest") && strings.Contains(body, "canvas")
}

// A visitor who already solved the challenge must be let straight
// through, even carrying signals that would otherwise score a block -
// re-challenging someone who already proved they're a browser adds no
// signal and only costs a worse experience.
func TestGuardPassedCookieBypassesBadSignals(t *testing.T) {
	reached := make(chan bool, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached <- true
		w.Write([]byte("ok"))
	}))
	defer origin.Close()

	guardAddr, challenge, trail := startGuard(t, origin.URL)

	// Get a real passed cookie the way a real visitor would: solve the
	// challenge for real through its own handler.
	rec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/x", nil)
	challenge.Serve(rec, getReq)
	token, nonce := extractTokenAndNonce(t, rec.Body.String())

	verifyRec := postVerify(challenge.Handler(), token, sha256Hex(nonce), validCanvasValue())
	cookies := verifyRec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected a passed cookie, got %v", cookies)
	}

	// Now send that cookie on a request that would otherwise score a
	// block (fragmented handshake + claims to be a browser).
	addr := fragmentingRelay(t, guardAddr)
	req, _ := http.NewRequest(http.MethodGet, "https://"+addr+"/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0 Safari/537.36")
	req.AddCookie(cookies[0])

	resp, err := tlsClient().Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 - a passed cookie should bypass scoring entirely", resp.StatusCode)
	}
	select {
	case <-reached:
	default:
		t.Fatal("origin was never reached")
	}

	// The trail must say why this was allowed. Recording it as a plain
	// score-0 allow would claim the visitor looked clean, when in fact
	// they carried both bad signals and were let through on the cookie.
	if e := lastEvidence(t, trail); !hasSignal(e, "challenge_solved") {
		t.Errorf("evidence = %+v, want challenge_solved as the recorded reason", e)
	}
}

// lastEvidence is the record Guard wrote for the request just made -
// the trail is only useful if it matches what actually happened to a
// real request, not just what the unit tests hand it directly.
func lastEvidence(t *testing.T, trail *Trail) Evidence {
	t.Helper()
	got := trail.Recent(1)
	if len(got) != 1 {
		t.Fatalf("no evidence was recorded for the request")
	}
	return got[0]
}

func hasSignal(e Evidence, name string) bool {
	for _, s := range e.Signals {
		if s == name {
			return true
		}
	}
	return false
}

func extractTokenAndNonce(t *testing.T, body string) (token, nonce string) {
	t.Helper()
	tm := tokenRe.FindStringSubmatch(body)
	nm := nonceRe.FindStringSubmatch(body)
	if tm == nil || nm == nil {
		t.Fatalf("could not extract token/nonce: %s", body)
	}
	return tm[1], nm[1]
}

// Shadow mode's whole promise: traffic that would be blocked still
// reaches the origin. If this ever fails, a client running shadow mode
// is silently breaking their own customers.
func TestGuardShadowModeNeverBlocks(t *testing.T) {
	reached := make(chan bool, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached <- true
		w.Write([]byte("ok"))
	}))
	defer origin.Close()

	guardAddr, _, trail := startGuardMode(t, origin.URL, ModeShadow)
	addr := fragmentingRelay(t, guardAddr)

	req, _ := http.NewRequest(http.MethodGet, "https://"+addr+"/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0 Safari/537.36")
	resp, err := tlsClient().Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 — shadow mode must not block", resp.StatusCode)
	}
	if string(body) != "ok" {
		t.Fatalf("body = %q, want the origin's own response", body)
	}
	select {
	case <-reached:
	default:
		t.Fatal("origin was never reached; shadow mode blocked a request it was only supposed to score")
	}

	// It must still have scored and recorded the real decision —
	// otherwise the report it produces is worthless.
	e := lastEvidence(t, trail)
	if e.Decision != "block" || e.Score != 100 {
		t.Errorf("evidence = %+v, want the block-at-100 it would have made", e)
	}
	if e.Enforced {
		t.Error("evidence says Enforced=true in shadow mode; a reader could not tell this block never happened")
	}
}

// A score that would earn a challenge must not serve the challenge
// page either — shadow mode means the visitor sees nothing at all.
func TestGuardShadowModeNeverChallenges(t *testing.T) {
	reached := make(chan bool, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached <- true
		w.Write([]byte("origin page"))
	}))
	defer origin.Close()

	guardAddr, _, trail := startGuardMode(t, origin.URL, ModeShadow)
	addr := fragmentingRelay(t, guardAddr)

	req, _ := http.NewRequest(http.MethodGet, "https://"+addr+"/some-page", nil)
	req.Header.Set("User-Agent", "curl/8.6.0")
	resp, err := tlsClient().Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if containsChallengeMarker(string(body)) {
		t.Fatal("challenge page was served in shadow mode")
	}
	if string(body) != "origin page" {
		t.Fatalf("body = %q, want the origin's own response", body)
	}
	select {
	case <-reached:
	default:
		t.Fatal("origin was never reached")
	}

	if e := lastEvidence(t, trail); e.Decision != "challenge" || e.Enforced {
		t.Errorf("evidence = %+v, want a recorded-but-unenforced challenge", e)
	}
}

// Enforce mode must keep stamping Enforced=true, or the two modes
// become indistinguishable in the record.
func TestGuardEnforceModeMarksEvidenceEnforced(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer origin.Close()

	guardAddr, _, trail := startGuard(t, origin.URL)
	addr := fragmentingRelay(t, guardAddr)

	req, _ := http.NewRequest(http.MethodGet, "https://"+addr+"/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0 Safari/537.36")
	resp, err := tlsClient().Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if e := lastEvidence(t, trail); !e.Enforced {
		t.Errorf("evidence = %+v, want Enforced=true in enforce mode", e)
	}
}
