package challenge_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/challenge"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

var (
	tokenRe = regexp.MustCompile(`token", "([^"]+)"`)
	// The page's JS now feeds the nonce into the PoW loop, so the encode
	// call is followed by ` + counter` rather than `)`. Match the string
	// literal itself, not a parenthesised call.
	nonceRe = regexp.MustCompile(`encode\("([^"]+)"`)
	// The server renders the required PoW difficulty into the page so the
	// browser knows how much work to do. html/template's JS escaper pads
	// the value with spaces (` 1 `), so allow whitespace around it.
	difficultyRe = regexp.MustCompile(`var difficulty =\s*(\d+)`)
)

const (
	challengePath = "/__hakaishield/challenge"
	verifyPath    = "/__hakaishield/verify"
)

func newChallenge(t *testing.T) *challenge.Challenge {
	t.Helper()
	c, err := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")
	if err != nil {
		t.Fatalf("NewChallenge: %v", err)
	}
	return c
}

func TestNewChallengeRejectsShortSecret(t *testing.T) {
	if _, err := challenge.NewChallenge([]byte("short-secret"), ""); err == nil {
		t.Fatal("short shared secret must be rejected")
	}
	if _, err := challenge.NewChallenge([]byte("0123456789abcdef0123456789abcdef"), ""); err != nil {
		t.Fatalf("32-byte secret should be accepted: %v", err)
	}
}

// solvePoW returns the smallest counter whose SHA-256 with nonce starts
// with `zeros` leading hex zeros — exactly the proof-of-work the
// challenge page's JS computes for the difficulty the server picked.
// Tests must derive `zeros` from the served page, never hardcode it, or
// a difficulty change silently breaks (or worse, over-satisfies) them.
func solvePoW(nonce string, zeros int) string {
	prefix := strings.Repeat("0", zeros)
	for i := 0; ; i++ {
		sum := sha256.Sum256([]byte(nonce + strconv.Itoa(i)))
		if hex.EncodeToString(sum[:])[:zeros] == prefix {
			return strconv.Itoa(i)
		}
	}
}

// wrongPoW returns a counter whose hash does NOT satisfy the `zeros`
// proof-of-work, for exercising the reject path with a well-formed but
// incorrect answer.
func wrongPoW(nonce string, zeros int) string {
	prefix := strings.Repeat("0", zeros)
	for i := 0; ; i++ {
		sum := sha256.Sum256([]byte(nonce + strconv.Itoa(i)))
		if hex.EncodeToString(sum[:])[:zeros] != prefix {
			return strconv.Itoa(i)
		}
	}
}

// pageDifficulty reads the `var difficulty = N` the server rendered into
// the page. It fails the test when absent rather than guessing, so a
// helper can never silently solve the wrong puzzle.
func pageDifficulty(t *testing.T, body string) int {
	t.Helper()
	m := difficultyRe.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("challenge page does not render a difficulty: %s", body)
	}
	d, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatalf("bad difficulty %q: %v", m[1], err)
	}
	return d
}

func validCanvas() string {
	img := image.NewNRGBA(image.Rect(0, 0, 300, 150))
	for y := 10; y < 28; y++ {
		for x := 10; x < 50; x++ {
			img.Set(x, y, color.NRGBA{A: 255})
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		panic(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(out.Bytes())
}

func TestCanvasProofRejectsFakeAndBlankImages(t *testing.T) {
	for name, proof := range map[string]string{
		"base64 garbage": "data:image/png;base64," + strings.Repeat("A", 150),
		"wrong dimensions": func() string {
			var out bytes.Buffer
			_ = png.Encode(&out, image.NewNRGBA(image.Rect(0, 0, 1, 1)))
			return "data:image/png;base64," + base64.StdEncoding.EncodeToString(out.Bytes())
		}(),
		"blank image": func() string {
			var out bytes.Buffer
			_ = png.Encode(&out, image.NewNRGBA(image.Rect(0, 0, 300, 150)))
			return "data:image/png;base64," + base64.StdEncoding.EncodeToString(out.Bytes())
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			c := newChallenge(t)
			token, nonce, difficulty := fetchPage(t, c.Handler(), challengePath)
			if got := postVerify(c.Handler(), token, solvePoW(nonce, difficulty), proof, "false").Code; got != http.StatusForbidden {
				t.Fatalf("invalid canvas status = %d, want 403", got)
			}
		})
	}
}

func TestChallengePageOffersRecovery(t *testing.T) {
	c := newChallenge(t)
	rec := httptest.NewRecorder()
	c.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, challengePath, nil))
	page := rec.Body.String()
	for _, want := range []string{"<noscript>", "showRecovery();", "Try again", `role", "alert"`} {
		if !strings.Contains(page, want) {
			t.Errorf("challenge page missing recovery element %q", want)
		}
	}
}

// fetchPage performs GET and extracts token + nonce + the difficulty the
// page was told to solve, so helpers always do the work the real page
// would do.
func fetchPage(t *testing.T, h http.Handler, path string) (token, nonce string, difficulty int) {
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
	return tm[1], nm[1], pageDifficulty(t, body)
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

	token, nonce, difficulty := fetchPage(t, h, challengePath+"?next=1")
	rec := postVerify(h, token, solvePoW(nonce, difficulty), validCanvas(), "false")

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status: want 303, got %d", rec.Code)
	}
	// The issued cookie must be accepted by Passed(). The verify response
	// also clears the attempt cookie, so pick the passed cookie by name.
	var pass *http.Cookie
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == "X-HakaiShield-Passed" {
			pass = ck
		}
	}
	if pass == nil {
		t.Fatal("expected a passed cookie, got none")
	}
	passReq := httptest.NewRequest(http.MethodGet, "/", nil)
	passReq.AddCookie(pass)
	if !c.Passed(passReq) {
		t.Fatal("Passed() returned false for a freshly issued cookie")
	}
}

func TestChallengeRedisNonceStoreRejectsReplayAcrossInstances(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	secret := []byte("test-secret-1234567890123456789012")
	nodeA, err := challenge.NewChallenge(secret, "")
	if err != nil {
		t.Fatalf("nodeA NewChallenge: %v", err)
	}
	nodeB, err := challenge.NewChallenge(secret, "")
	if err != nil {
		t.Fatalf("nodeB NewChallenge: %v", err)
	}
	nodeA.SetNonceStore(challenge.NewRedisNonceStore(rdb, "test:nonce:"))
	nodeB.SetNonceStore(challenge.NewRedisNonceStore(rdb, "test:nonce:"))

	token, nonce, difficulty := fetchPage(t, nodeA.Handler(), challengePath+"?next=1")
	answer := solvePoW(nonce, difficulty)
	if rec := postVerify(nodeA.Handler(), token, answer, validCanvas(), "false"); rec.Code != http.StatusSeeOther {
		t.Fatalf("first verify status = %d, want 303", rec.Code)
	}
	if rec := postVerify(nodeB.Handler(), token, answer, validCanvas(), "false"); rec.Code != http.StatusForbidden {
		t.Fatalf("replay on second node status = %d, want 403", rec.Code)
	}
}

// TestChallengeRejectsWrongAnswer: wrong SHA-256 must not pass.
func TestChallengeRejectsWrongAnswer(t *testing.T) {
	c := newChallenge(t)
	h := c.Handler()
	token, nonce, difficulty := fetchPage(t, h, challengePath)
	rec := postVerify(h, token, wrongPoW(nonce, difficulty), validCanvas(), "false")
	if rec.Code == http.StatusSeeOther {
		t.Fatal("wrong answer must not pass")
	}
}

// TestChallengeRejectsAutomationFlag: navigator.webdriver=true must not pass.
func TestChallengeRejectsAutomationFlag(t *testing.T) {
	c := newChallenge(t)
	h := c.Handler()
	token, nonce, difficulty := fetchPage(t, h, challengePath)
	rec := postVerify(h, token, solvePoW(nonce, difficulty), validCanvas(), "true")
	if rec.Code == http.StatusSeeOther {
		t.Fatal("automation=true must not pass")
	}
}

// TestChallengeRejectsMissingCanvas: no canvas proof must not pass.
func TestChallengeRejectsMissingCanvas(t *testing.T) {
	c := newChallenge(t)
	h := c.Handler()
	token, nonce, difficulty := fetchPage(t, h, challengePath)
	rec := postVerify(h, token, solvePoW(nonce, difficulty), "", "false")
	if rec.Code == http.StatusSeeOther {
		t.Fatal("missing canvas must not pass")
	}
}

// TestChallengeRejectsTamperedToken: a forged token must not pass.
func TestChallengeRejectsTamperedToken(t *testing.T) {
	c := newChallenge(t)
	h := c.Handler()
	token, nonce, difficulty := fetchPage(t, h, challengePath)
	tampered := token[:len(token)-1] + "X"
	rec := postVerify(h, tampered, solvePoW(nonce, difficulty), validCanvas(), "false")
	if rec.Code == http.StatusSeeOther {
		t.Fatal("tampered token must not pass")
	}
}

func TestChallengeRejectsTokenOnDifferentHost(t *testing.T) {
	c := newChallenge(t)
	h := c.Handler()
	token, nonce, difficulty := fetchPage(t, h, challengePath)

	form := url.Values{}
	form.Set("token", token)
	form.Set("answer", solvePoW(nonce, difficulty))
	form.Set("canvas", validCanvas())
	req := httptest.NewRequest(http.MethodPost, verifyPath, strings.NewReader(form.Encode()))
	req.Host = "other.example"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code == http.StatusSeeOther {
		t.Fatal("a challenge token must not verify on a different host")
	}
}

func TestChallengeTokenIsSingleUse(t *testing.T) {
	c := newChallenge(t)
	h := c.Handler()
	token, nonce, difficulty := fetchPage(t, h, challengePath)
	first := postVerify(h, token, solvePoW(nonce, difficulty), validCanvas(), "false")
	if first.Code != http.StatusSeeOther {
		t.Fatalf("first verification: want 303, got %d", first.Code)
	}
	second := postVerify(h, token, solvePoW(nonce, difficulty), validCanvas(), "false")
	if second.Code == http.StatusSeeOther {
		t.Fatal("a challenge token must not be reusable")
	}
}

// TestPassedRejectsForgedCookie: a hand-crafted cookie must not be trusted.
func TestPassedRejectsForgedCookie(t *testing.T) {
	c := newChallenge(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "X-HakaiShield-Passed", Value: "1700000000.forged"})
	if c.Passed(req) {
		t.Fatal("Passed() must reject a forged cookie")
	}
}

func TestPassedRejectsCookieOnDifferentHost(t *testing.T) {
	c := newChallenge(t)
	h := c.Handler()
	token, nonce, difficulty := fetchPage(t, h, challengePath)
	passResponse := postVerify(h, token, solvePoW(nonce, difficulty), validCanvas(), "false")
	if passResponse.Code != http.StatusSeeOther {
		t.Fatalf("verification: want 303, got %d", passResponse.Code)
	}
	var pass *http.Cookie
	for _, cookie := range passResponse.Result().Cookies() {
		if cookie.Name == "X-HakaiShield-Passed" {
			pass = cookie
			break
		}
	}
	if pass == nil {
		t.Fatal("verification did not set a passed cookie")
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "other.example"
	req.AddCookie(pass)
	if c.Passed(req) {
		t.Fatal("a passed cookie must not transfer to another host")
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
	token, nonce, difficulty := fetchPage(t, h, challengePath)

	form := url.Values{}
	form.Set("token", token)
	form.Set("answer", solvePoW(nonce, difficulty))
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

// TestChallengePageDetectsGPUPlatformMismatch: the WebGL renderer-vs-OS
// cross-check is computed entirely in JS the Go tests never execute — the
// same blind spot TestChallengePageDetectsAdvancedAutomation exists for.
// This is the only thing that would catch a future edit silently deleting
// it from the served page.
func TestChallengePageDetectsGPUPlatformMismatch(t *testing.T) {
	c := newChallenge(t)
	h := c.Handler()

	req := httptest.NewRequest(http.MethodGet, challengePath, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()

	for _, marker := range []string{
		`Direct3D|\bD3D(?:9|11|12)\b`,
		`Metal Renderer|Apple GPU|Apple M[0-9]`,
		`Adreno|Mali-|PowerVR Rogue`,
	} {
		if !strings.Contains(body, marker) {
			t.Errorf("challenge page missing GPU-platform mismatch check %q", marker)
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
