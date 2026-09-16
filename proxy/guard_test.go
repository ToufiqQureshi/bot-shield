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
func startGuard(t *testing.T, origin string) (addr string, challenge *Challenge) {
	t.Helper()

	p, err := New(origin)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	challenge, err = NewChallenge()
	if err != nil {
		t.Fatalf("NewChallenge: %v", err)
	}
	guard := NewGuard(p, challenge, &Stats{})

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

	return addr, challenge
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

	addr, _ := startGuard(t, origin.URL)

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

	guardAddr, _ := startGuard(t, origin.URL)
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

	guardAddr, _ := startGuard(t, origin.URL)
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

	guardAddr, challenge := startGuard(t, origin.URL)

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
