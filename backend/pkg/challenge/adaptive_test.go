package challenge_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/challenge"
)

// The Phase 2 tests cover the adaptive behaviour: the server picks the
// difficulty from the risk score Guard attaches, escalates after failed
// solves, keeps the difficulty inside the signed token so a client cannot
// lower it, and lets a harder solve buy only a shorter trust window.

// diffFor expects the exported behaviour through the served page: GET the
// challenge with the given score attached and read the difficulty back.
// Going through the page (not an internal function) is deliberate — it is
// the only path a real visitor experiences, and it covers the whole chain
// (context plumbing, token, template render) at once.
func servedDifficulty(t *testing.T, score int) int {
	t.Helper()
	c := newChallenge(t)
	h := c.Handler()

	req := httptest.NewRequest(http.MethodGet, challengePath, nil)
	req = req.WithContext(challenge.WithRisk(req.Context(), score))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET challenge: status %d", rec.Code)
	}
	return pageDifficulty(t, rec.Body.String())
}

func TestDifficultyBandsFollowRiskScore(t *testing.T) {
	// score <= 0: clean traffic gets the lightest puzzle.
	if got := servedDifficulty(t, 0); got != 1 {
		t.Errorf("score 0: difficulty = %d, want 1", got)
	}
	// mid band: fired a weak signal, gets the original 8-bit puzzle.
	if got := servedDifficulty(t, 25); got != 2 {
		t.Errorf("score 25: difficulty = %d, want 2", got)
	}
	// strong band: the block-bar score gets the hardest puzzle.
	if got := servedDifficulty(t, 50); got != 3 {
		t.Errorf("score 50: difficulty = %d, want 3", got)
	}
	if got := servedDifficulty(t, 150); got != 3 {
		t.Errorf("score 150: difficulty = %d, want 3 (clamped, never above max)", got)
	}
}

func TestDifficultyNeverLeavesMobileSafeRange(t *testing.T) {
	// Absurd inputs must clamp, not escape the [1,3] range. A negative
	// score is not reachable from Guard, but the helper must not turn it
	// into an unclamped value if it ever is.
	for _, score := range []int{-100, -1, 0, 1, 49, 50, 100, 1000000} {
		if got := servedDifficulty(t, score); got < 1 || got > 3 {
			t.Errorf("score %d: difficulty %d outside [1,3]", score, got)
		}
	}
}

// solveFromTokenPage drives one full challenge flow with a chosen number
// of prior failed solves (via pre-set attempt cookies) and returns the
// verify response. attemptCookies are carried on both the GET (so Serve
// escalates) and the POST (irrelevant to verification, but realistic).
func solveWithAttempts(t *testing.T, score int, attemptCookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	c := newChallenge(t)
	h := c.Handler()

	get := httptest.NewRequest(http.MethodGet, challengePath, nil)
	get = get.WithContext(challenge.WithRisk(get.Context(), score))
	for _, ck := range attemptCookies {
		get.AddCookie(ck)
	}
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, get)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET challenge: status %d", getRec.Code)
	}
	body := getRec.Body.String()
	token := tokenRe.FindStringSubmatch(body)[1]
	nonce := nonceRe.FindStringSubmatch(body)[1]
	difficulty := pageDifficulty(t, body)

	form := url.Values{}
	form.Set("token", token)
	form.Set("answer", solvePoW(nonce, difficulty))
	form.Set("canvas", validCanvas())
	form.Set("automation", "false")
	post := httptest.NewRequest(http.MethodPost, verifyPath, strings.NewReader(form.Encode()))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, ck := range attemptCookies {
		post.AddCookie(ck)
	}
	postRec := httptest.NewRecorder()
	h.ServeHTTP(postRec, post)
	return postRec
}

// attemptCookieFrom builds a signed attempt cookie by actually failing a
// solve `n` times, so the cookie under test is always exactly what the
// server would really issue (never a hand-forged one).
func attemptCookiesFrom(t *testing.T, n int) []*http.Cookie {
	t.Helper()
	c := newChallenge(t)
	h := c.Handler()

	var cookies []*http.Cookie
	for i := 0; i < n; i++ {
		get := httptest.NewRequest(http.MethodGet, challengePath, nil)
		for _, ck := range cookies {
			get.AddCookie(ck)
		}
		getRec := httptest.NewRecorder()
		h.ServeHTTP(getRec, get)
		body := getRec.Body.String()
		token := tokenRe.FindStringSubmatch(body)[1]
		nonce := nonceRe.FindStringSubmatch(body)[1]
		difficulty := pageDifficulty(t, body)

		form := url.Values{}
		form.Set("token", token)
		// A well-formed answer that fails THIS page's PoW: take the next
		// counter after the real solution's predecessor...
		form.Set("answer", wrongPoW(nonce, difficulty))
		form.Set("canvas", validCanvas())
		form.Set("automation", "false")
		post := httptest.NewRequest(http.MethodPost, verifyPath, strings.NewReader(form.Encode()))
		post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for _, ck := range cookies {
			post.AddCookie(ck)
		}
		postRec := httptest.NewRecorder()
		h.ServeHTTP(postRec, post)
		if postRec.Code != http.StatusForbidden {
			t.Fatalf("expected failed solve (403), got %d", postRec.Code)
		}
		cookies = postRec.Result().Cookies()
		if len(cookies) == 0 {
			t.Fatalf("failed solve did not set an attempt cookie")
		}
	}
	return cookies
}

func TestFailedSolvesEscalateNextChallenge(t *testing.T) {
	// Start from a mid-band score (difficulty 2). Two failed solves must
	// push the next challenge to difficulty 3. Escalation uses the signed
	// attempt cookie — never per-IP state — so this test also proves the
	// cookie actually drives it.
	if got := servedDifficulty(t, 25); got != 2 {
		t.Fatalf("expected base difficulty 2 for score 25, got %d", got)
	}
	get := httptest.NewRequest(http.MethodGet, challengePath, nil)
	get = get.WithContext(challenge.WithRisk(get.Context(), 25))
	for _, ck := range attemptCookiesFrom(t, 2) {
		get.AddCookie(ck)
	}
	rec := httptest.NewRecorder()
	newChallenge(t).Handler().ServeHTTP(rec, get)
	if got := pageDifficulty(t, rec.Body.String()); got != 3 {
		t.Errorf("after 2 failed solves: difficulty = %d, want 3", got)
	}
}

func TestEscalationIsCappedAtMaxDifficulty(t *testing.T) {
	// maxAttempts failures later, the difficulty must still be 3 — never
	// beyond the mobile-safe cap, no matter how many failures accumulate.
	get := httptest.NewRequest(http.MethodGet, challengePath, nil)
	for _, ck := range attemptCookiesFrom(t, 6) {
		get.AddCookie(ck)
	}
	rec := httptest.NewRecorder()
	newChallenge(t).Handler().ServeHTTP(rec, get)
	if got := pageDifficulty(t, rec.Body.String()); got != 3 {
		t.Errorf("after 6 failed solves: difficulty = %d, want 3 (capped)", got)
	}
}

func TestForgedAttemptCookieIsIgnored(t *testing.T) {
	// An attacker-inflated attempt cookie must be treated as zero
	// attempts, not as escalation — and not as a crash either.
	get := httptest.NewRequest(http.MethodGet, challengePath, nil)
	get.AddCookie(&http.Cookie{Name: "X-HakaiShield-Attempt", Value: "NQ|example.com.definitely-not-signed"})
	rec := httptest.NewRecorder()
	newChallenge(t).Handler().ServeHTTP(rec, get)
	if got := pageDifficulty(t, rec.Body.String()); got != 1 {
		t.Errorf("forged attempt cookie: difficulty = %d, want 1 (unsigned cookie ignored)", got)
	}
}

func TestPassedCookieTrustWindowDecaysWithDifficulty(t *testing.T) {
	// A harder solve must buy a SHORTER window. Verify through the real
	// flow at each difficulty, then read the cookie's MaxAge back.
	want := map[int]int{1: 30 * 60, 2: 15 * 60, 3: 5 * 60}
	for difficulty, maxAge := range want {
		score := []int{0, 25, 50}[difficulty-1]
		rec := solveWithAttempts(t, score, nil)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("difficulty %d: verify status %d, want 303", difficulty, rec.Code)
		}
		var pass *http.Cookie
		for _, ck := range rec.Result().Cookies() {
			if ck.Name == "X-HakaiShield-Passed" {
				pass = ck
			}
		}
		if pass == nil {
			t.Fatalf("difficulty %d: no passed cookie set", difficulty)
		}
		if pass.MaxAge != maxAge {
			t.Errorf("difficulty %d: passed cookie MaxAge = %d, want %d", difficulty, pass.MaxAge, maxAge)
		}
	}
}

func TestPassedCookieCarriesDifficultyInsideSignature(t *testing.T) {
	// The trust window must be re-derived from the difficulty inside the
	// signed payload. The cleanest proof: tamper the difficulty inside the
	// payload (3 -> 1 would extend 5 min to 30 min if the server trusted
	// the plaintext) and expect the cookie to be rejected outright.
	c := newChallenge(t)
	rec := solveWithAttempts(t, 50, nil) // difficulty 3
	var pass *http.Cookie
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == "X-HakaiShield-Passed" {
			pass = ck
		}
	}
	if pass == nil {
		t.Fatal("no passed cookie set")
	}
	parts := strings.SplitN(pass.Value, ".", 2)
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode passed payload: %v", err)
	}
	fields := strings.Split(string(raw), "|")
	if len(fields) != 3 || fields[1] != "3" {
		t.Fatalf("unexpected passed payload %q", raw)
	}
	// Flip difficulty 3 -> 1 without re-signing.
	fields[1] = "1"
	tampered := base64.RawURLEncoding.EncodeToString([]byte(strings.Join(fields, "|")))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "X-HakaiShield-Passed", Value: tampered + "." + parts[1]})
	if c.Passed(req) {
		t.Fatal("a passed cookie with a tampered difficulty must be rejected")
	}
}

func TestTelemetryMalformedValuesAreRejected(t *testing.T) {
	// Booleans must be exactly "true"/"false"; elapsed must be a plain
	// bounded integer. Anything else is a 400, not a lenient default.
	cases := []struct {
		name       string
		automation string
		headless   string
		elapsed    string
	}{
		{"automation=1", "1", "false", ""},
		{"automation=TRUE", "TRUE", "false", ""},
		{"automation=yes", "yes", "false", ""},
		{"headless=1", "false", "1", ""},
		{"elapsed=negative", "false", "false", "-5"},
		{"elapsed=not-a-number", "false", "false", "soon"},
		{"elapsed=over-bound", "false", "false", "600001"},
		{"elapsed=float", "false", "false", "12.5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newChallenge(t)
			token, nonce, difficulty := fetchPage(t, c.Handler(), challengePath)
			form := url.Values{}
			form.Set("token", token)
			form.Set("answer", solvePoW(nonce, difficulty))
			form.Set("canvas", validCanvas())
			form.Set("automation", tc.automation)
			form.Set("headless", tc.headless)
			form.Set("elapsed", tc.elapsed)
			req := httptest.NewRequest(http.MethodPost, verifyPath, strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			c.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("malformed telemetry: status %d, want 400 (body=%s)", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestTelemetryFastSolveIsMeasuredNotEnforced(t *testing.T) {
	// A suspiciously fast solve must still PASS — the fast flag is a
	// measurement, and enforcing it would block cached-page loads and
	// clock-skewed clients (CLAUDE.md Section 14).
	c := newChallenge(t)
	token, nonce, difficulty := fetchPage(t, c.Handler(), challengePath)
	form := url.Values{}
	form.Set("token", token)
	form.Set("answer", solvePoW(nonce, difficulty))
	form.Set("canvas", validCanvas())
	form.Set("automation", "false")
	form.Set("headless", "false")
	form.Set("elapsed", "0")
	req := httptest.NewRequest(http.MethodPost, verifyPath, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	c.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("elapsed=0 must not fail the challenge, got %d", rec.Code)
	}
}

func TestTokenDifficultyCannotBeLowered(t *testing.T) {
	// The page solves difficulty 3. A client that rewrites the token's
	// difficulty field to 1 (keeping the same signature) must be rejected
	// — otherwise the whole adaptive ladder is client-optional.
	c := newChallenge(t)
	h := c.Handler()
	req := httptest.NewRequest(http.MethodGet, challengePath, nil)
	req = req.WithContext(challenge.WithRisk(req.Context(), 50))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	token := tokenRe.FindStringSubmatch(body)[1]
	nonce := nonceRe.FindStringSubmatch(body)[1]
	if got := pageDifficulty(t, body); got != 3 {
		t.Fatalf("expected served difficulty 3, got %d", got)
	}

	// Decode the token payload and drop its difficulty field 3 -> 1.
	parts := strings.SplitN(token, ".", 2)
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode token: %v", err)
	}
	fields := strings.Split(string(raw), "|")
	if len(fields) != 5 {
		t.Fatalf("unexpected token field count %d", len(fields))
	}
	fields[2] = "1"
	lowered := base64.RawURLEncoding.EncodeToString([]byte(strings.Join(fields, "|"))) + "." + parts[1]

	// The answer is only valid for difficulty 3; it must NOT satisfy the
	// tampered token's claimed difficulty 1 either way — and the signature
	// over the modified payload no longer matches, so verification must
	// reject with 403, not accept.
	form := url.Values{}
	form.Set("token", lowered)
	form.Set("answer", solvePoW(nonce, 3))
	form.Set("canvas", validCanvas())
	form.Set("automation", "false")
	post := httptest.NewRequest(http.MethodPost, verifyPath, strings.NewReader(form.Encode()))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postRec := httptest.NewRecorder()
	h.ServeHTTP(postRec, post)
	if postRec.Code != http.StatusForbidden {
		t.Fatalf("tampered-token verify: status %d, want 403", postRec.Code)
	}
}

func TestSolveClearsAttemptEscalation(t *testing.T) {
	// After a failed solve sets an attempt cookie, a successful solve must
	// clear it — a visitor who eventually passes starts from a clean slate.
	cookies := attemptCookiesFrom(t, 1)
	rec := solveWithAttempts(t, 0, cookies)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("verify: status %d, want 303", rec.Code)
	}
	var cleared bool
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == "X-HakaiShield-Attempt" && ck.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("successful solve did not clear the attempt cookie")
	}
}

func TestAttemptCookieExpires(t *testing.T) {
	// The escalation memory must be bounded in time (15 min), not forever.
	cookies := attemptCookiesFrom(t, 1)
	if len(cookies) != 1 {
		t.Fatalf("want exactly one attempt cookie, got %d", len(cookies))
	}
	if cookies[0].MaxAge <= 0 || cookies[0].MaxAge > int((15*time.Minute).Seconds()) {
		t.Errorf("attempt cookie MaxAge = %d, want in (0, 900]", cookies[0].MaxAge)
	}
}

func TestAttemptCookieExpiryIsEnforcedByServer(t *testing.T) {
	const secret = "test-secret-1234567890123456789012"
	for _, tc := range []struct {
		name string
		age  time.Duration
		want int
	}{
		{"fresh", time.Minute, 3},
		{"expired", 16 * time.Minute, 1},
		{"future", -time.Minute, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := base64.RawURLEncoding.EncodeToString([]byte("4|example.com|" + strconv.FormatInt(time.Now().Add(-tc.age).Unix(), 10)))
			mac := hmac.New(sha256.New, []byte(secret))
			_, _ = mac.Write([]byte(payload))
			cookie := &http.Cookie{Name: "X-HakaiShield-Attempt", Value: payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))}
			req := httptest.NewRequest(http.MethodGet, "http://example.com"+challengePath, nil)
			req.AddCookie(cookie)
			rec := httptest.NewRecorder()
			newChallenge(t).Handler().ServeHTTP(rec, req)
			if got := pageDifficulty(t, rec.Body.String()); got != tc.want {
				t.Fatalf("difficulty = %d, want %d", got, tc.want)
			}
		})
	}
}
