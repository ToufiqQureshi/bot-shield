package challenge_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/ToufiqQureshi/bot-shield/pkg/challenge"
)

var (
	tokenRe = regexp.MustCompile(`token", "([^"]+)"`)
	// The page's JS now feeds the nonce into the PoW loop, so the encode
	// call is followed by ` + counter` rather than `)`. Match the string
	// literal itself, not a parenthesised call.
	nonceRe = regexp.MustCompile(`encode\("([^"]+)"`)
)

const (
	challengePath = "/__botshield/challenge"
	verifyPath    = "/__botshield/verify"
)

func newChallenge(t *testing.T) *challenge.Challenge {
	t.Helper()
	c, err := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")
	if err != nil {
		t.Fatalf("NewChallenge: %v", err)
	}
	return c
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

// wrongPoW returns a counter whose hash does NOT satisfy the PoW, for
// exercising the reject path with a well-formed but incorrect answer.
func wrongPoW(nonce string) string {
	for i := 0; ; i++ {
		sum := sha256.Sum256([]byte(nonce + strconv.Itoa(i)))
		if hex.EncodeToString(sum[:])[:2] != "00" {
			return strconv.Itoa(i)
		}
	}
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
	rec := postVerify(h, token, solvePoW(nonce), validCanvas(), "false")

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
	token, nonce := fetchPage(t, h, challengePath)
	rec := postVerify(h, token, wrongPoW(nonce), validCanvas(), "false")
	if rec.Code == http.StatusSeeOther {
		t.Fatal("wrong answer must not pass")
	}
}

// TestChallengeRejectsAutomationFlag: navigator.webdriver=true must not pass.
func TestChallengeRejectsAutomationFlag(t *testing.T) {
	c := newChallenge(t)
	h := c.Handler()
	token, nonce := fetchPage(t, h, challengePath)
	rec := postVerify(h, token, solvePoW(nonce), validCanvas(), "true")
	if rec.Code == http.StatusSeeOther {
		t.Fatal("automation=true must not pass")
	}
}

// TestChallengeRejectsMissingCanvas: no canvas proof must not pass.
func TestChallengeRejectsMissingCanvas(t *testing.T) {
	c := newChallenge(t)
	h := c.Handler()
	token, nonce := fetchPage(t, h, challengePath)
	rec := postVerify(h, token, solvePoW(nonce), "", "false")
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
	rec := postVerify(h, tampered, solvePoW(nonce), validCanvas(), "false")
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
	form.Set("answer", solvePoW(nonce))
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

// TestChallengePageDetectsAdvancedAutomation: the client-side automation
// checks (__pwInitScripts, default Puppeteer viewport, Chromium-without-
// Chrome client-hints brand) are computed entirely in JS the Go tests
// never execute — Score()/handleVerify only ever see the boolean form
// field they produce. This test is the only thing that would catch a
// future edit silently deleting one of them from the served page.
func TestChallengePageDetectsAdvancedAutomation(t *testing.T) {
	c := newChallenge(t)
	h := c.Handler()

	req := httptest.NewRequest(http.MethodGet, challengePath, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()

	for _, marker := range []string{
		base64.StdEncoding.EncodeToString([]byte("__pwInitScripts")),
		"window.innerWidth === 800 && window.innerHeight === 600",
		"getHighEntropyValues",
		`b.brand === "Chromium"`,
		`b.brand === "Google Chrome"`,
	} {
		if !strings.Contains(body, marker) {
			t.Errorf("challenge page missing automation check %q", marker)
		}
	}
}

// TestChallengePageObfuscatesAutomationTells: the classic tell property
// names (cdc_..., __playwright, __puppeteer, __selenium_unwrapped, ...)
// must not appear in the served page as plain text — a scraper author's
// first move is to curl the challenge page and grep it for exactly
// these strings without ever running the JS. They are still present,
// base64-encoded, and decoded at runtime (see the _d helper).
func TestChallengePageObfuscatesAutomationTells(t *testing.T) {
	c := newChallenge(t)
	h := c.Handler()

	req := httptest.NewRequest(http.MethodGet, challengePath, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()

	plaintextTells := []string{
		"cdc_adoQpoasnfa76pfcZLmcfl_",
		"__playwright",
		"__puppeteer",
		"__pwInitScripts",
		"__selenium_unwrapped",
		"__webdriver_evaluate",
		"__driver_evaluate",
		"callPhantom",
		"_phantom",
		"__nightmare",
	}
	for _, tell := range plaintextTells {
		if strings.Contains(body, tell) {
			t.Errorf("challenge page leaks plaintext automation tell %q — should be base64-encoded", tell)
		}
		encoded := base64.StdEncoding.EncodeToString([]byte(tell))
		if !strings.Contains(body, encoded) {
			t.Errorf("challenge page missing encoded form of %q (want %q)", tell, encoded)
		}
	}
}
