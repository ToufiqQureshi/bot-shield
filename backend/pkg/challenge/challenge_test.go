package challenge_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/ToufiqQureshi/bot-shield/pkg/challenge"
)

var (
	tokenRe = regexp.MustCompile(`token", "([^"]+)"`)
	nonceRe = regexp.MustCompile(`encode\("([^"]+)"\)`)
)

const (
	challengePath = "/__botshield/challenge"
	verifyPath    = "/__botshield/verify"
)

func newChallenge(t *testing.T) *challenge.Challenge {
	t.Helper()
	c, err := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"))
	if err != nil {
		t.Fatalf("NewChallenge: %v", err)
	}
	return c
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func validCanvas() string {
	return "data:image/png;base64," + strings.Repeat("A", 150)
}

// fetchPage performs GET and extracts token + nonce from the JS.
func fetchPage(t *testing.T, h http.Handler, path string) (token, nonce string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status %d", path, rec.Code)
	}
	body := rec.Body.String()
	tm := tokenRe.FindStringSubmatch(body)
	nm := nonceRe.FindStringSubmatch(body)
	if tm == nil || nm == nil {
		t.Fatalf("could not extract token/nonce from: %s", body)
	}
	return tm[1], nm[1]
}

// postVerify posts the challenge answer form.
func postVerify(h http.Handler, token, answer, canvas, automation string) *httptest.ResponseRecorder {
	form := url.Values{}
	form.Set("token", token)
	form.Set("answer", answer)
	form.Set("canvas", canvas)
	form.Set("automation", automation)
	req := httptest.NewRequest(http.MethodPost, verifyPath, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestChallengeRealFlowPasses: browser-like client completes the full flow.
func TestChallengeRealFlowPasses(t *testing.T) {
	c := newChallenge(t)
	h := c.Handler()

	token, nonce := fetchPage(t, h, challengePath+"?next=1")
	rec := postVerify(h, token, sha256Hex(nonce), validCanvas(), "false")

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status: want 303, got %d", rec.Code)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected a passed cookie, got none")
	}

	// The issued cookie must be accepted by Passed()
	passReq := httptest.NewRequest(http.MethodGet, "/", nil)
	passReq.AddCookie(cookies[0])
	if !c.Passed(passReq) {
		t.Fatal("Passed() returned false for a freshly issued cookie")
	}
}

// TestChallengeRejectsWrongAnswer: wrong SHA-256 must not pass.
func TestChallengeRejectsWrongAnswer(t *testing.T) {
	c := newChallenge(t)
	h := c.Handler()
	token, _ := fetchPage(t, h, challengePath)
	rec := postVerify(h, token, strings.Repeat("0", 64), validCanvas(), "false")
	if rec.Code == http.StatusSeeOther {
		t.Fatal("wrong answer must not pass")
	}
}

// TestChallengeRejectsAutomationFlag: navigator.webdriver=true must not pass.
func TestChallengeRejectsAutomationFlag(t *testing.T) {
	c := newChallenge(t)
	h := c.Handler()
	token, nonce := fetchPage(t, h, challengePath)
	rec := postVerify(h, token, sha256Hex(nonce), validCanvas(), "true")
	if rec.Code == http.StatusSeeOther {
		t.Fatal("automation=true must not pass")
	}
}

// TestChallengeRejectsMissingCanvas: no canvas proof must not pass.
func TestChallengeRejectsMissingCanvas(t *testing.T) {
	c := newChallenge(t)
	h := c.Handler()
	token, nonce := fetchPage(t, h, challengePath)
	rec := postVerify(h, token, sha256Hex(nonce), "", "false")
	if rec.Code == http.StatusSeeOther {
		t.Fatal("missing canvas must not pass")
	}
}

// TestChallengeRejectsTamperedToken: a forged token must not pass.
func TestChallengeRejectsTamperedToken(t *testing.T) {
	c := newChallenge(t)
	h := c.Handler()
	token, nonce := fetchPage(t, h, challengePath)
	tampered := token[:len(token)-1] + "X"
	rec := postVerify(h, tampered, sha256Hex(nonce), validCanvas(), "false")
	if rec.Code == http.StatusSeeOther {
		t.Fatal("tampered token must not pass")
	}
}

// TestPassedRejectsForgedCookie: a hand-crafted cookie must not be trusted.
func TestPassedRejectsForgedCookie(t *testing.T) {
	c := newChallenge(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "X-BotShield-Passed", Value: "1700000000.forged"})
	if c.Passed(req) {
		t.Fatal("Passed() must reject a forged cookie")
	}
}

// TestChallengeHandlerMethodNotAllowed: GET on verify and POST on challenge must 405.
func TestChallengeHandlerMethodNotAllowed(t *testing.T) {
	c := newChallenge(t)
	h := c.Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, challengePath, nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST challenge: want 405, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, verifyPath, nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET verify: want 405, got %d", rec.Code)
	}
}

// TestChallengeRejectsHeadlessFlag: headless=true (detected VM WebGL) must not pass.
func TestChallengeRejectsHeadlessFlag(t *testing.T) {
	c := newChallenge(t)
	h := c.Handler()
	token, nonce := fetchPage(t, h, challengePath)

	form := url.Values{}
	form.Set("token", token)
	form.Set("answer", sha256Hex(nonce))
	form.Set("canvas", validCanvas())
	form.Set("automation", "false")
	form.Set("headless", "true")

	req := httptest.NewRequest(http.MethodPost, verifyPath, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code == http.StatusSeeOther {
		t.Fatal("headless=true must not pass")
	}
}

// TestProbeJSServed: /__botshield/probe.js must return valid JS.
func TestProbeJSServed(t *testing.T) {
	c := newChallenge(t)
	h := c.Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/__botshield/probe.js", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /__botshield/probe.js: want 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Errorf("Content-Type: want javascript, got %s", ct)
	}
}
