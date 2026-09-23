package challenge

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html/template"
	"image"
	"image/draw"
	"image/png"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/observability"
	"github.com/redis/go-redis/v9"

	"github.com/ToufiqQureshi/hakaishield/pkg/labels"
)

// Challenge issues a JS-only puzzle to every visitor Guard does not
// block (core/guard.go). A plain HTTP client that never runs JavaScript
// never even attempts the verify step; the proof-of-work answer stops a
// cached/replayed response; the
// canvas proof raises the bar toward needing a real browser engine.
type Challenge struct {
	secret     []byte
	theme      string
	nonceStore NonceStore
	// labels, when set, remembers what a challenged request looked like
	// so a solve can label it human. It never affects whether a
	// challenge is issued or passed.
	labels *labels.Recorder
}

type themeContextKey struct{}

// WithTheme selects a validated tenant theme for this request only.
func WithTheme(ctx context.Context, theme string) context.Context {
	if theme != "ghost" && theme != "branded" {
		return ctx
	}
	return context.WithValue(ctx, themeContextKey{}, theme)
}

// NonceStore consumes solved challenge nonces exactly once. Implementations
// must be bounded and fast because verification is visitor-controlled.
type NonceStore interface {
	Consume(ctx context.Context, nonce string, now time.Time, ttl time.Duration) bool
}

type localNonceStore struct {
	mu      sync.Mutex
	used    map[string]time.Time
	ordered []nonceUse
	head    int
}

type nonceUse struct {
	nonce string
	at    time.Time
}

// challengeMaxAge bounds how long an issued puzzle stays solvable —
// long enough for a slow real page load, short enough that a captured
// token can't be replayed much later.
const challengeMaxAge = 2 * time.Minute

// passedCookie marks a visitor who already solved a challenge, so
// they aren't re-challenged on every request in the same session.
const passedCookie = "X-HakaiShield-Passed" // #nosec G101 -- cookie name, not a credential
const passedMaxAge = 30 * time.Minute
const maxUsedChallenges = 50_000

const challengePath = "/__hakaishield/challenge"
const verifyPath = "/__hakaishield/verify"

// maxVerifyBodyBytes bounds the POST body from an unauthenticated,
// visitor-controlled endpoint. A real canvas proof is a few KB; this
// leaves headroom without letting one caller send an unbounded body.
const maxVerifyBodyBytes = 64 * 1024

// NewChallenge takes a shared secret for signing challenges.
// By using a shared secret provided at startup (e.g., via CLI flag),
// any instance in a multi-node deployment can verify a challenge
// issued by any other instance, making the challenge system completely
// stateless and database-free.
func NewChallenge(secret []byte, theme string) (*Challenge, error) {
	if len(secret) == 0 {
		return nil, fmt.Errorf("proxy: challenge secret cannot be empty")
	}
	if theme == "" {
		theme = "ghost"
	}
	return &Challenge{secret: secret, theme: theme, nonceStore: newLocalNonceStore()}, nil
}

func newLocalNonceStore() *localNonceStore {
	return &localNonceStore{used: make(map[string]time.Time)}
}

// SetNonceStore replaces the default single-process replay guard. Passing nil
// restores the local fallback used for development and single-node deployments.
// SetLabelRecorder attaches label collection for the learned scorer
// (pkg/decide). Passing nil turns it off, which is the default.
//
// Call it during setup, before serving traffic: the recorder is read
// without locking on the request path.
func (c *Challenge) SetLabelRecorder(r *labels.Recorder) {
	c.labels = r
}

func (c *Challenge) SetNonceStore(store NonceStore) {
	if store == nil {
		store = newLocalNonceStore()
	}
	c.nonceStore = store
}

func (c *Challenge) sign(payload string) string {
	mac := hmac.New(sha256.New, c.secret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// token binds a nonce, issue time, difficulty, host, and the page to return
// to into one tamper-evident string, so verification needs no server-side
// storage per outstanding challenge and the client cannot pick its own
// difficulty.
func (c *Challenge) token(nonce, redirectPath, host string, difficulty int, issuedAt time.Time) string {
	encodedPath := base64.RawURLEncoding.EncodeToString([]byte(redirectPath))
	encodedHost := base64.RawURLEncoding.EncodeToString([]byte(canonicalHost(host)))
	payload := strings.Join([]string{nonce, strconv.FormatInt(issuedAt.Unix(), 10), strconv.Itoa(difficulty), encodedPath, encodedHost}, "|")
	encoded := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return encoded + "." + c.sign(encoded)
}

// challengeToken is the verified content of an issued puzzle. Difficulty
// travels inside the signature so a client cannot lower it.
type challengeToken struct {
	nonce        string
	redirectPath string
	difficulty   int
}

// parseToken verifies the signature and expiry and returns the
// embedded fields. A tampered, malformed, or expired token is
// rejected here, not left for the caller to notice.
func (c *Challenge) parseToken(tok, host string) (challengeToken, bool) {
	parts := strings.SplitN(tok, ".", 2)
	if len(parts) != 2 {
		return challengeToken{}, false
	}
	encoded, sig := parts[0], parts[1]
	if subtle.ConstantTimeCompare([]byte(sig), []byte(c.sign(encoded))) != 1 {
		return challengeToken{}, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return challengeToken{}, false
	}
	fields := strings.SplitN(string(raw), "|", 5)
	if len(fields) != 5 {
		return challengeToken{}, false
	}
	issuedUnix, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return challengeToken{}, false
	}
	issuedAt := time.Unix(issuedUnix, 0)
	age := time.Since(issuedAt)
	if age < 0 || age > challengeMaxAge {
		return challengeToken{}, false // expired, or timestamped in the future
	}
	difficulty, err := strconv.Atoi(fields[2])
	if err != nil {
		return challengeToken{}, false
	}
	if difficulty < minDifficulty {
		difficulty = minDifficulty
	}
	if difficulty > maxDifficulty {
		difficulty = maxDifficulty
	}
	pathBytes, err := base64.RawURLEncoding.DecodeString(fields[3])
	if err != nil {
		return challengeToken{}, false
	}
	hostBytes, err := base64.RawURLEncoding.DecodeString(fields[4])
	if err != nil {
		return challengeToken{}, false
	}
	if string(hostBytes) != canonicalHost(host) {
		return challengeToken{}, false // token issued for a different host
	}
	return challengeToken{
		nonce:        fields[0],
		redirectPath: safeRedirectPath(string(pathBytes)),
		difficulty:   difficulty,
	}, true
}

// canonicalHost normalizes the request host before it participates in signed
// challenge state. Hostnames are case-insensitive and an optional port is not
// part of tenant identity.
func canonicalHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	} else {
		raw = strings.Trim(raw, "[]")
	}
	return strings.TrimSuffix(strings.ToLower(raw), ".")
}

// consume marks a valid nonce as used once. Entries are bounded and expire
// with the challenge so an attacker cannot turn verification into unbounded
// process memory. Multi-node deployments should use the planned Redis-backed
// nonce store; this local guard still closes replay on a single instance.
func (s *localNonceStore) Consume(_ context.Context, nonce string, now time.Time, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Issuance is chronological on the request path, so expired entries
	// leave from the front. Work is proportional to entries actually evicted.
	for s.head < len(s.ordered) && now.Sub(s.ordered[s.head].at) >= ttl {
		s.popOldest()
	}
	if _, exists := s.used[nonce]; exists {
		return false
	}
	if len(s.used) >= maxUsedChallenges {
		// The oldest consumed nonce has the least replay life left.
		s.popOldest()
	}
	s.used[nonce] = now
	s.ordered = append(s.ordered, nonceUse{nonce: nonce, at: now})
	if s.head > 1024 && s.head*2 >= len(s.ordered) {
		copy(s.ordered, s.ordered[s.head:])
		s.ordered = s.ordered[:len(s.ordered)-s.head]
		s.head = 0
	}
	return true
}

func (s *localNonceStore) popOldest() {
	oldest := s.ordered[s.head]
	delete(s.used, oldest.nonce)
	s.ordered[s.head] = nonceUse{}
	s.head++
}

// RedisNonceStore shares replay protection across nodes. On Redis failure it
// falls back to a local bounded store so a dependency outage degrades to
// single-node replay protection instead of locking out real visitors.
type RedisNonceStore struct {
	client   *redis.Client
	prefix   string
	fallback *localNonceStore
}

func NewRedisNonceStore(client *redis.Client, prefix string) *RedisNonceStore {
	if prefix == "" {
		prefix = "hakaishield:challenge:nonce:"
	}
	return &RedisNonceStore{client: client, prefix: prefix, fallback: newLocalNonceStore()}
}

func (s *RedisNonceStore) Consume(ctx context.Context, nonce string, now time.Time, ttl time.Duration) bool {
	if s == nil || s.client == nil {
		return false
	}
	ok, err := s.client.SetNX(ctx, s.prefix+nonce, "1", ttl).Result()
	if err != nil {
		observability.Inc("challenge_nonce_redis_error_total")
		return s.fallback.Consume(ctx, nonce, now, ttl)
	}
	if !ok {
		observability.Inc("challenge_nonce_replay_reject_total")
	}
	return ok
}

// safeRedirectPath keeps "return to the page you asked for" from
// becoming an open redirect: only an in-site path is accepted.
func safeRedirectPath(raw string) string {
	if raw == "" || raw[0] != '/' || strings.HasPrefix(raw, "//") || strings.ContainsAny(raw, "\\") {
		return "/"
	}
	return raw
}

func randomNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// validPoW verifies the Proof-of-Work: SHA-256(nonce + answer) must start
// with `difficulty` hex zeros. The difficulty comes from the signed token,
// never from the request, and is clamped so an absurd value cannot turn
// verification into a long loop on our side.
func validPoW(nonce, answer string, difficulty int) bool {
	// Prevent unbounded body attacks. A counter for the hardest puzzle is
	// still only a few digits, so a long answer is malformed by definition.
	if len(answer) == 0 || len(answer) > 20 {
		return false
	}
	if difficulty < minDifficulty {
		difficulty = minDifficulty
	}
	if difficulty > maxDifficulty {
		difficulty = maxDifficulty
	}
	sum := sha256.Sum256([]byte(nonce + answer))
	hashHex := hex.EncodeToString(sum[:])
	return strings.HasPrefix(hashHex, strings.Repeat("0", difficulty))
}

// Canvas data is untrusted. Keep both compressed and decoded work bounded
// before inspecting pixels; a valid PNG by itself does not prove a browser ran.
const canvasDataPrefix = "data:image/png;base64,"
const maxCanvasProofLen = 48 * 1024
const canvasWidth, canvasHeight = 300, 150

func validCanvasProof(proof string) bool {
	if !strings.HasPrefix(proof, canvasDataPrefix) || len(proof) > maxCanvasProofLen {
		return false
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(proof, canvasDataPrefix))
	if err != nil {
		return false
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width != canvasWidth || config.Height != canvasHeight {
		return false
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return false
	}
	var pixels []byte
	switch decoded := img.(type) {
	case *image.NRGBA:
		pixels = decoded.Pix
	case *image.RGBA:
		pixels = decoded.Pix
	default:
		canvas := image.NewNRGBA(image.Rect(0, 0, canvasWidth, canvasHeight))
		draw.Draw(canvas, canvas.Bounds(), img, image.Point{}, draw.Src)
		pixels = canvas.Pix
	}
	background := pixels[:4]
	changed := 0
	for i := 4; i < len(pixels); i += 4 {
		pixel := pixels[i : i+4]
		if pixel[0] != background[0] || pixel[1] != background[1] || pixel[2] != background[2] || pixel[3] != background[3] {
			changed++
		}
	}
	return changed >= 32 && changed < canvasWidth*canvasHeight/2
}

type challengeData struct {
	Nonce        string
	Token        string
	VerifyPath   string
	RedirectPath string
	Theme        string
	// Difficulty is the number of leading hex zeros the page's PoW must
	// produce. Rendered into the page so the client does the right amount
	// of work; the server re-checks it from the signed token regardless.
	Difficulty int
}

// challengePage's script base64-encodes the classic automation-tell
// property names (via the _d/atob helper) so they don't appear as
// plain text in the served page — only in this Go source, which never
// reaches a visitor's browser. Decoded, in the order they appear:
// callPhantom, _phantom, __nightmare, __selenium_unwrapped,
// __webdriver_evaluate, __driver_evaluate, cdc_adoQpoasnfa76pfcZLmcfl_,
// cdc_adoQpoasnfa76pfcZLmcfl_Array, __playwright, __puppeteer,
// __pwInitScripts. TestChallengePageObfuscatesAutomationTells asserts
// none of them leak into the rendered page as plain text.
var challengePage = template.Must(template.New("challenge").Parse(`<!doctype html>
<html><head><meta charset="utf-8">
{{if eq .Theme "ghost"}}
<title></title><style>body{background:#fff;margin:0;padding:0;}</style>
{{else if eq .Theme "branded"}}
<title>Securing connection...</title>
<style>
body { font-family: sans-serif; text-align: center; margin-top: 15%; background: #f9f9f9; color: #333; }
.loader { border: 4px solid #ddd; border-top: 4px solid #3498db; border-radius: 50%; width: 40px; height: 40px; animation: spin 1s linear infinite; margin: 20px auto; }
@keyframes spin { 0% { transform: rotate(0deg); } 100% { transform: rotate(360deg); } }
h2 { font-weight: normal; font-size: 1.2rem; }
</style>
{{else}}
<title>Checking your browser</title>
{{end}}
</head>
<body>
{{if eq .Theme "ghost"}}
<!-- Invisible ghost mode -->
{{else if eq .Theme "branded"}}
<h2>Securing your connection...</h2><div class="loader" aria-hidden="true"></div>
{{else}}
<p>Checking your browser before continuing&hellip;</p>
{{end}}
<noscript><p>JavaScript is required to verify this browser. Enable it and reload this page, or contact the site owner for access.</p></noscript>
<script>
(async function () {
  function showRecovery() {
    document.body.textContent = "Browser verification could not complete. ";
    var retry = document.createElement("a");
    retry.href = "{{.RedirectPath}}";
    retry.textContent = "Try again";
    var notice = document.createElement("p");
    notice.setAttribute("role", "alert");
    notice.appendChild(retry);
    document.body.appendChild(notice);
  }
  try {
    var start = Date.now();
    
    // Proof-of-Work: find a counter where SHA-256(nonce + counter) starts
    // with the server's required number of leading hex zeros. The server
    // picks that count from the request's risk, capped so even the hardest
    // puzzle stays under a second on a phone. A clean visitor only ever
    // computes a handful of hashes.
    var difficulty = {{.Difficulty}};
    var prefix = "";
    for (var p = 0; p < difficulty; p++) { prefix += "0"; }
    var counter = 0;
    var answer = "";
    var enc = new TextEncoder();
    while (true) {
      var data = enc.encode("{{.Nonce}}" + counter.toString());
      var digest = await crypto.subtle.digest("SHA-256", data);
      var hashArray = Array.from(new Uint8Array(digest));
      var hashHex = hashArray.map(function(b) { return b.toString(16).padStart(2, "0"); }).join("");
      if (hashHex.substring(0, difficulty) === prefix) {
        answer = counter.toString();
        break;
      }
      counter++;
      // A pathological cap only. At the hardest difficulty the expected
      // count is 4096, so 200000 is roughly 49 standard deviations away:
      // if it ever fires, the answer fails server-side verification and
      // the page retries, which is the safe direction (fail closed, not
      // a bypass) rather than hanging the tab forever.
      if (counter > 200000) { answer = ""; break; }
    }

    var canvasProof = "";
    try {
      var c = document.createElement("canvas");
      var ctx = c.getContext("2d");
      ctx.textBaseline = "top";
      ctx.font = "16px Arial";
      ctx.fillText("hakaishield", 2, 2);
      canvasProof = c.toDataURL();
    } catch (e) {}

    // _d decodes the base64-encoded automation-tell property names
    function _d(s) { return atob(s); }

    var automation = false;
    try {
      if (navigator.webdriver) automation = true;
      if (window[_d("Y2FsbFBoYW50b20=")] || window[_d("X3BoYW50b20=")] || window[_d("X19uaWdodG1hcmU=")]) automation = true;
      if (document[_d("X19zZWxlbml1bV91bndyYXBwZWQ=")] || document[_d("X193ZWJkcml2ZXJfZXZhbHVhdGU=")] || document[_d("X19kcml2ZXJfZXZhbHVhdGU=")]) automation = true;
      if (window[_d("Y2RjX2Fkb1Fwb2FzbmZhNzZwZmNaTG1jZmxf")] || window[_d("Y2RjX2Fkb1Fwb2FzbmZhNzZwZmNaTG1jZmxfQXJyYXk=")]) automation = true;
      if (window[_d("X19wbGF5d3JpZ2h0")] || window[_d("X19wdXBwZXRlZXI=")]) automation = true;

      // Stealth evasion artifact: property descriptor on navigator.webdriver
      var desc = Object.getOwnPropertyDescriptor(navigator, "webdriver");
      if (desc && (desc.value === false || desc.get)) automation = true;

      // Chrome consistency: authentic Chrome defines window.chrome
      if (/Chrome/.test(navigator.userAgent) && !/Edge|Edg/.test(navigator.userAgent)) {
        if (!window.chrome || typeof window.chrome !== "object") automation = true;
      }

      // Playwright's own init-script injection leaves this global set
      if (typeof window[_d("X19wd0luaXRTY3JpcHRz")] !== "undefined") automation = true;

      // Puppeteer's classic default viewport.
      if (window.innerWidth === 800 && window.innerHeight === 600) automation = true;

      // Client-hints brand inconsistency
      if (navigator.userAgentData && navigator.userAgentData.getHighEntropyValues) {
        try {
          var brandInfo = await navigator.userAgentData.getHighEntropyValues(["fullVersionList"]);
          var brands = (brandInfo && brandInfo.fullVersionList) || [];
          var hasChromium = brands.some(function (b) { return b.brand === "Chromium"; });
          var hasChrome = brands.some(function (b) { return b.brand === "Google Chrome"; });
          if (hasChromium && !hasChrome) automation = true;
        } catch (e) {}
      }

      // Advanced Stealth Detection: Error.stack tracing
      // Headless browsers evaluating scripts often leave traces like "evaluate" or "puppeteer_evaluation_script"
      try {
        throw new Error("stack_trace_check");
      } catch (err) {
        if (err.stack) {
          if (err.stack.indexOf(_d("cHVwcGV0ZWVyX2V2YWx1YXRpb25fc2NyaXB0")) !== -1 || 
              err.stack.indexOf(_d("X19wbGF5d3JpZ2h0X2V2YWx1YXRpb25fc2NyaXB0")) !== -1 ||
              err.stack.indexOf("evaluate@") !== -1) {
            automation = true;
          }
        }
      }

      // Advanced Stealth Detection: navigator.permissions inconsistency
      try {
        var perm = await navigator.permissions.query({name: 'notifications'});
        if (Notification && Notification.permission === 'denied' && perm.state === 'prompt') {
          automation = true;
        }
      } catch (e) {}

    } catch (e) {}

    var headless = false;
    try {
      var glCanvas = document.createElement("canvas");
      var gl = glCanvas.getContext("webgl") || glCanvas.getContext("experimental-webgl");
      if (gl) {
        var dbg = gl.getExtension("WEBGL_debug_renderer_info");
        if (dbg) {
          var rend = gl.getParameter(dbg.UNMASKED_RENDERER_WEBGL) || "";
          if (/SwiftShader|llvmpipe|VirtualBox|Mesa OffScreen/i.test(rend)) {
            headless = true;
          }
        }
      }
    } catch (e) {}

    var body = new URLSearchParams();
    body.set("token", "{{.Token}}");
    body.set("answer", answer);
    body.set("canvas", canvasProof);
    body.set("automation", String(automation));
    body.set("headless", String(headless));
    body.set("elapsed", String(Date.now() - start));

    var res = await fetch("{{.VerifyPath}}", {
      method: "POST",
      body: body,
      credentials: "same-origin",
    });
    if (res.redirected || res.ok) {
      window.location = res.url || "{{.RedirectPath}}";
    } else {
      showRecovery();
    }
  } catch (e) {
    showRecovery();
  }
})();
</script>
</body></html>
`))

// Serve issues a fresh puzzle: a nonce for the page's JS to
// hash, and a signed token carrying that nonce plus where to send the
// visitor once they pass. It's meant to be called with r.URL still
// pointing at the page the visitor actually asked for — i.e. served
// in place of proxying that request, not as a separate "go solve this
// first" landing page — so a passing visitor lands back on the real
// page they wanted. core.Guard calls this mid-proxy for every request
// it decides to challenge; the dedicated GET route in Handler() below
// stays available for manual testing.
func (c *Challenge) Serve(w http.ResponseWriter, r *http.Request) {
	nonce, err := randomNonce()
	if err != nil {
		http.Error(w, "challenge unavailable", http.StatusInternalServerError)
		return
	}
	redirectPath := safeRedirectPath(r.URL.RequestURI())
	host := canonicalHost(r.Host)
	if host == "" {
		http.Error(w, "challenge unavailable", http.StatusBadRequest)
		return
	}
	// Adaptive difficulty: the server decides how much work this visitor
	// must do, from the risk score Guard already computed and from how many
	// times this client has recently failed. Both are clamped to the
	// mobile-safe range inside difficultyFor.
	priorAttempts := c.attempts(r)
	difficulty := difficultyFor(RiskFromContext(r.Context()), priorAttempts)
	observability.Inc(counterIssued)
	if priorAttempts >= attemptsPerStep {
		observability.Inc(counterEscalatedIssued)
	}
	tok := c.token(nonce, redirectPath, host, difficulty, time.Now())

	// Remember what this request looked like, so solving the challenge
	// can label it human later. The sample is parked server-side against
	// the nonce and deliberately never put in the token: the token goes
	// to the client, and a list of which checks a bot tripped tells it
	// exactly what to fix.
	if sample, ok := labels.SampleFrom(r.Context()); ok && c.labels != nil {
		c.labels.ChallengeIssued(nonce, sample)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	theme := c.theme
	if selected, ok := r.Context().Value(themeContextKey{}).(string); ok {
		theme = selected
	}
	_ = challengePage.Execute(w, challengeData{
		Nonce:        nonce,
		Token:        tok,
		VerifyPath:   verifyPath,
		RedirectPath: redirectPath,
		Theme:        theme,
		Difficulty:   difficulty,
	})
}

// handleVerify checks the JS-computed answer against the token we
// issued. A wrong answer, bad token, or missing canvas proof is a
// hard 403: the visitor's next page load re-triggers the challenge
// from Serve, which issues a fresh token and puzzle.
func (c *Challenge) handleVerify(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxVerifyBodyBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "request too large or malformed", http.StatusBadRequest)
		return
	}

	info, ok := c.parseToken(r.FormValue("token"), r.Host)
	if !ok {
		http.Error(w, "invalid token", http.StatusForbidden)
		return
	}
	// Validate the client-reported telemetry before spending anything else
	// on it: a malformed field is a bad request, not a solve attempt.
	telemetry, ok := parseTelemetry(r.Form)
	if !ok {
		observability.Inc(counterTelemetryBad)
		http.Error(w, "malformed challenge telemetry", http.StatusBadRequest)
		return
	}
	now := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 50*time.Millisecond)
	defer cancel()
	if !c.nonceStore.Consume(ctx, info.nonce, now, challengeMaxAge) {
		http.Error(w, "challenge already used", http.StatusForbidden)
		return
	}
	answer := r.FormValue("answer")
	if !validPoW(info.nonce, answer, info.difficulty) {
		c.failSolve(w, r)
		http.Error(w, "incorrect answer", http.StatusForbidden)
		return
	}
	if !validCanvasProof(r.FormValue("canvas")) {
		c.failSolve(w, r)
		http.Error(w, "invalid canvas proof", http.StatusForbidden)
		return
	}
	// A stock Selenium/Puppeteer/Playwright automation framework
	// exposes navigator.webdriver, __pwInitScripts, a default 800x600
	// viewport, or a Chromium-without-Chrome client-hints brand, even
	// when the browser itself is real (so canvas/sha256 pass) —
	// ROADMAP item 6. We also detect headless cloud VM renderers
	// (SwiftShader/llvmpipe). These values are client-supplied, so they
	// fail the challenge rather than labelling the visitor, and a
	// suspiciously fast solve is only measured, never enforced.
	if telemetry.Automation || telemetry.Headless {
		c.failSolve(w, r)
		http.Error(w, "automation detected", http.StatusForbidden)
		return
	}

	// Verification grants a passed cookie, but the canvas and automation
	// values are client supplied. Record only a candidate human label;
	// the trainer excludes these observations by default.
	c.clearAttempts(w, r)
	recordChallengeOutcome(true, r.UserAgent())
	if telemetry.FastSolve {
		observability.Inc(counterFastSolve)
	}
	c.labels.ChallengeSolved(info.nonce)

	c.setPassedCookie(w, r.Host, info.difficulty)
	http.Redirect(w, r, info.redirectPath, http.StatusSeeOther)
}

// failSolve records a rejected solve attempt: the outcome counters, and
// the signed attempt cookie that makes this client's next puzzle harder.
// It is one helper so no reject branch can forget one of the two.
func (c *Challenge) failSolve(w http.ResponseWriter, r *http.Request) {
	recordChallengeOutcome(false, r.UserAgent())
	c.recordAttempt(w, r)
}

// Passed reports whether r already carries a valid, unexpired
// "solved the challenge" cookie signed by c. core.Guard checks this
// before re-challenging a visitor who already passed.
func (c *Challenge) Passed(r *http.Request) bool {
	ck, err := r.Cookie(passedCookie)
	if err != nil {
		return false
	}
	parts := strings.SplitN(ck.Value, ".", 2)
	if len(parts) != 2 {
		return false
	}
	if subtle.ConstantTimeCompare([]byte(parts[1]), []byte(c.sign(parts[0]))) != 1 {
		return false
	}
	rawPayload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	payload := strings.SplitN(string(rawPayload), "|", 3)
	if len(payload) != 3 || payload[2] != canonicalHost(r.Host) {
		return false
	}
	difficulty, err := strconv.Atoi(payload[1])
	if err != nil {
		return false
	}
	issuedUnix, err := strconv.ParseInt(payload[0], 10, 64)
	if err != nil {
		return false
	}
	age := time.Since(time.Unix(issuedUnix, 0))
	// Trust decays: a visitor who needed the hardest puzzle is trusted
	// for a shorter window than one who passed the lightest (Phase 2).
	// The window is re-derived from the signed difficulty, never from the
	// client's cookie MaxAge, which it controls.
	return age >= 0 && age <= passedTrustWindow(difficulty)
}

// Handler returns the HTTP surface for the challenge: GET issues a
// puzzle, POST verifies the answer. Not yet wired into the proxy's
// own request path (core.Guard calls Serve/handleVerify directly for
// the requests it decides to challenge); these routes stay available
// for manual testing and for the page's own verify POST.
func (c *Challenge) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(challengePath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		c.Serve(w, r)
	})
	mux.HandleFunc(verifyPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		c.handleVerify(w, r)
	})
	return mux
}
