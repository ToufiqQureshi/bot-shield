package proxy

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
	"time"
)

func newTestChallenge(t *testing.T) *Challenge {
	t.Helper()
	c, err := NewChallenge()
	if err != nil {
		t.Fatalf("NewChallenge: %v", err)
	}
	return c
}

var (
	tokenRe = regexp.MustCompile(`token", "([^"]+)"`)
	nonceRe = regexp.MustCompile(`encode\("([^"]+)"\)`)
)

// fetchChallenge does what a real browser's first request would:
// GET the page and pull the token/nonce a real client's JS would see.
func fetchChallenge(t *testing.T, h http.Handler, path string) (token, nonce string, rec *httptest.ResponseRecorder) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status %d", path, rec.Code)
	}
	body := rec.Body.String()
	tm := tokenRe.FindStringSubmatch(body)
	nm := nonceRe.FindStringSubmatch(body)
	if tm == nil || nm == nil {
		t.Fatalf("could not extract token/nonce from challenge page: %s", body)
	}
	return tm[1], nm[1], rec
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func validCanvasValue() string {
	return canvasDataPrefix + strings.Repeat("A", minCanvasProofLen)
}

func postVerify(h http.Handler, token, answer, canvas string) *httptest.ResponseRecorder {
	return postVerifyFull(h, token, answer, canvas, "false")
}

func postVerifyFull(h http.Handler, token, answer, canvas, automation string) *httptest.ResponseRecorder {
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

// TestChallengeRealFlowPasses proves a client that actually does what
// the page's JS does (hash the nonce, send a canvas proof) gets
// through end to end: GET the challenge, POST the right answer, get
// redirected with a passed cookie.
func TestChallengeRealFlowPasses(t *testing.T) {
	c := newTestChallenge(t)
	h := c.Handler()

	token, nonce, _ := fetchChallenge(t, h, challengePath+"?next=1")
	rec := postVerify(h, token, sha256Hex(nonce), validCanvasValue())

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != passedCookie {
		t.Fatalf("expected %s cookie to be set, got %v", passedCookie, cookies)
	}

	passedReq := httptest.NewRequest(http.MethodGet, "/", nil)
	passedReq.AddCookie(cookies[0])
	if !c.Passed(passedReq) {
		t.Fatal("Passed() = false for a cookie just issued by a successful verify")
	}
}

func TestChallengeRejectsWrongAnswer(t *testing.T) {
	c := newTestChallenge(t)
	h := c.Handler()
	token, _, _ := fetchChallenge(t, h, challengePath)

	rec := postVerify(h, token, "0000000000000000000000000000000000000000000000000000000000000000", validCanvasValue())
	if rec.Code == http.StatusSeeOther {
		t.Fatal("wrong answer must not pass")
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatal("wrong answer must not set the passed cookie")
	}
}

// A real browser being driven by a stock automation framework
// (Selenium/Puppeteer/Playwright) passes the sha256+canvas checks
// (it's a genuine browser) but its own JS reports navigator.webdriver
// - ROADMAP item 6. Must not pass just because the render checks did.
func TestChallengeRejectsAutomationFlag(t *testing.T) {
	c := newTestChallenge(t)
	h := c.Handler()
	token, nonce, _ := fetchChallenge(t, h, challengePath)

	rec := postVerifyFull(h, token, sha256Hex(nonce), validCanvasValue(), "true")
	if rec.Code == http.StatusSeeOther {
		t.Fatal("automation=true must not pass, even with a correct answer and canvas proof")
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatal("automation=true must not set the passed cookie")
	}
}

func TestChallengeRejectsMissingCanvasProof(t *testing.T) {
	c := newTestChallenge(t)
	h := c.Handler()
	token, nonce, _ := fetchChallenge(t, h, challengePath)

	rec := postVerify(h, token, sha256Hex(nonce), "")
	if rec.Code == http.StatusSeeOther {
		t.Fatal("missing canvas proof must not pass")
	}
}

func TestChallengeRejectsTamperedToken(t *testing.T) {
	c := newTestChallenge(t)
	h := c.Handler()
	token, nonce, _ := fetchChallenge(t, h, challengePath)

	tampered := token[:len(token)-1] + "x"
	rec := postVerify(h, tampered, sha256Hex(nonce), validCanvasValue())
	if rec.Code == http.StatusSeeOther {
		t.Fatal("a tampered token must not pass verification")
	}
}

func TestChallengeRejectsExpiredToken(t *testing.T) {
	c := newTestChallenge(t)
	nonce, err := randomNonce()
	if err != nil {
		t.Fatal(err)
	}
	oldToken := c.token(nonce, "/", time.Now().Add(-challengeMaxAge-time.Second))

	rec := postVerify(c.Handler(), oldToken, sha256Hex(nonce), validCanvasValue())
	if rec.Code == http.StatusSeeOther {
		t.Fatal("an expired token must not pass verification")
	}
}

// TestChallengeAnswerFromWrongSecretFails proves the token/answer are
// actually bound to this Challenge's own secret, not just any HMAC —
// a second instance (e.g. after a restart) must not be able to verify
// what the first one issued, since the secret isn't shared anywhere.
func TestChallengeAnswerFromWrongSecretFails(t *testing.T) {
	issuer := newTestChallenge(t)
	verifier := newTestChallenge(t)

	token, nonce, _ := fetchChallenge(t, issuer.Handler(), challengePath)
	rec := postVerify(verifier.Handler(), token, sha256Hex(nonce), validCanvasValue())
	if rec.Code == http.StatusSeeOther {
		t.Fatal("a token signed by a different secret must not verify")
	}
}

func TestSafeRedirectPathRejectsOpenRedirect(t *testing.T) {
	cases := map[string]string{
		"/ok":             "/ok",
		"/ok?x=1":         "/ok?x=1",
		"":                "/",
		"//evil.com":      "/",
		"http://evil.com": "/",
		"/\\evil.com":     "/",
		"/a\\..\\b":       "/",
	}
	for in, want := range cases {
		if got := safeRedirectPath(in); got != want {
			t.Errorf("safeRedirectPath(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestChallengeRedirectsToOriginalPath proves a passing visitor lands
// back on the exact page (path + query) they were actually on when
// Serve was invoked in place of it — the real integration
// shape (ROADMAP item 5 intercepts a real request and serves the
// challenge instead of proxying it), not just "some page starting
// with the challenge route".
func TestChallengeRedirectsToOriginalPath(t *testing.T) {
	c := newTestChallenge(t)
	h := c.Handler()
	original := challengePath + "?r=%2Fdashboard"
	token, nonce, _ := fetchChallenge(t, h, original)
	rec := postVerify(h, token, sha256Hex(nonce), validCanvasValue())

	loc := rec.Result().Header.Get("Location")
	if loc != original {
		t.Fatalf("redirect Location = %q, want the exact original request %q", loc, original)
	}
}

// TestChallengeServedInPlaceOfArbitraryPathRedirectsThere proves the
// real integration shape: Serve called with r.URL pointing
// at an ordinary site path (not the dedicated test route) sends a
// passing visitor back to that same path.
func TestChallengeServedInPlaceOfArbitraryPathRedirectsThere(t *testing.T) {
	c := newTestChallenge(t)
	original := "/checkout?item=42"

	req := httptest.NewRequest(http.MethodGet, original, nil)
	rec := httptest.NewRecorder()
	c.Serve(rec, req)
	body := rec.Body.String()
	tm := tokenRe.FindStringSubmatch(body)
	nm := nonceRe.FindStringSubmatch(body)
	if tm == nil || nm == nil {
		t.Fatalf("could not extract token/nonce: %s", body)
	}

	verifyRec := postVerify(c.Handler(), tm[1], sha256Hex(nm[1]), validCanvasValue())
	if loc := verifyRec.Result().Header.Get("Location"); loc != original {
		t.Fatalf("redirect Location = %q, want %q", loc, original)
	}
}

func TestPassedRejectsForgedCookie(t *testing.T) {
	c := newTestChallenge(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: passedCookie, Value: "1700000000.forged-signature"})
	if c.Passed(req) {
		t.Fatal("Passed() must reject a cookie with a forged signature")
	}
}

func TestPassedRejectsExpiredCookie(t *testing.T) {
	c := newTestChallenge(t)
	payload := strconv.FormatInt(time.Now().Add(-passedMaxAge-time.Minute).Unix(), 10)
	value := payload + "." + c.sign(payload)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: passedCookie, Value: value})
	if c.Passed(req) {
		t.Fatal("Passed() must reject an expired signed cookie")
	}
}

func TestVerifyRejectsOversizedBody(t *testing.T) {
	c := newTestChallenge(t)
	huge := strings.Repeat("a", maxVerifyBodyBytes+1)
	req := httptest.NewRequest(http.MethodPost, verifyPath, strings.NewReader("canvas="+huge))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	c.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("oversized body: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestChallengeHandlerRejectsWrongMethods(t *testing.T) {
	c := newTestChallenge(t)
	h := c.Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, challengePath, nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST %s: status = %d, want %d", challengePath, rec.Code, http.StatusMethodNotAllowed)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, verifyPath, nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET %s: status = %d, want %d", verifyPath, rec.Code, http.StatusMethodNotAllowed)
	}
}
