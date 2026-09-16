package proxy

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Challenge issues a JS-only puzzle to traffic that scoring (ROADMAP
// item 5, not built yet) will decide is ambiguous. A plain HTTP
// client that never runs JavaScript never even attempts the verify
// step; the sha256 answer stops a cached/replayed response; the
// canvas proof raises the bar toward needing a real browser engine.
type Challenge struct {
	secret []byte
}

// challengeMaxAge bounds how long an issued puzzle stays solvable —
// long enough for a slow real page load, short enough that a captured
// token can't be replayed much later.
const challengeMaxAge = 2 * time.Minute

// passedCookie marks a visitor who already solved a challenge, so
// they aren't re-challenged on every request in the same session.
const passedCookie = "X-BotShield-Passed"
const passedMaxAge = 30 * time.Minute

const challengePath = "/__botshield/challenge"
const verifyPath = "/__botshield/verify"

// maxVerifyBodyBytes bounds the POST body from an unauthenticated,
// visitor-controlled endpoint. A real canvas proof is a few KB; this
// leaves headroom without letting one caller send an unbounded body.
const maxVerifyBodyBytes = 64 * 1024

// NewChallenge generates a fresh signing secret. It lives in memory
// only: restarting bot-shield, or running more than one instance,
// invalidates outstanding challenges and passed-cookies. That's fine
// for a single-process v1 deployment (docs/ARCHITECTURE.md); sharing
// the secret across instances needs the planned Redis store, not this.
func NewChallenge() (*Challenge, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("proxy: generating challenge secret: %w", err)
	}
	return &Challenge{secret: secret}, nil
}

func (c *Challenge) sign(payload string) string {
	mac := hmac.New(sha256.New, c.secret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// token binds a nonce, issue time, and the page to return to into one
// tamper-evident string, so verification needs no server-side storage
// per outstanding challenge.
func (c *Challenge) token(nonce, redirectPath string, issuedAt time.Time) string {
	encodedPath := base64.RawURLEncoding.EncodeToString([]byte(redirectPath))
	payload := strings.Join([]string{nonce, strconv.FormatInt(issuedAt.Unix(), 10), encodedPath}, "|")
	encoded := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return encoded + "." + c.sign(encoded)
}

// parseToken verifies the signature and expiry and returns the
// embedded fields. A tampered, malformed, or expired token is
// rejected here, not left for the caller to notice.
func (c *Challenge) parseToken(tok string) (nonce, redirectPath string, ok bool) {
	parts := strings.SplitN(tok, ".", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	encoded, sig := parts[0], parts[1]
	if subtle.ConstantTimeCompare([]byte(sig), []byte(c.sign(encoded))) != 1 {
		return "", "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", false
	}
	fields := strings.SplitN(string(raw), "|", 3)
	if len(fields) != 3 {
		return "", "", false
	}
	issuedUnix, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return "", "", false
	}
	issuedAt := time.Unix(issuedUnix, 0)
	age := time.Since(issuedAt)
	if age < 0 || age > challengeMaxAge {
		return "", "", false // expired, or timestamped in the future
	}
	pathBytes, err := base64.RawURLEncoding.DecodeString(fields[2])
	if err != nil {
		return "", "", false
	}
	return fields[0], safeRedirectPath(string(pathBytes)), true
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

// expectedAnswer is what the page's JS is asked to compute: the
// SHA-256 of the nonce we issued. Anyone who never fetched the page
// (so never saw the nonce) cannot produce it in advance.
func expectedAnswer(nonce string) string {
	sum := sha256.Sum256([]byte(nonce))
	return hex.EncodeToString(sum[:])
}

// canvasDataPrefix / minCanvasProofLen: a genuine canvas.toDataURL()
// render is a base64 PNG of at least a few hundred bytes. This is a
// shape check, not a render check — it's a client-reported string, so
// a bot that specifically studies bot-shield can fake a value that
// passes it without ever rendering anything. Documented as a known,
// accepted limitation (see docs/DECISIONS.md): validating the actual
// pixel content server-side is a project of its own, out of MVP scope.
const canvasDataPrefix = "data:image/png;base64,"
const minCanvasProofLen = 100

func validCanvasProof(proof string) bool {
	return strings.HasPrefix(proof, canvasDataPrefix) && len(proof) > minCanvasProofLen
}

type challengeData struct {
	Nonce        string
	Token        string
	VerifyPath   string
	RedirectPath string
}

var challengePage = template.Must(template.New("challenge").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>Checking your browser</title></head>
<body>
<p>Checking your browser before continuing&hellip;</p>
<script>
(async function () {
  try {
    var start = Date.now();
    var enc = new TextEncoder().encode("{{.Nonce}}");
    var digest = await crypto.subtle.digest("SHA-256", enc);
    var answer = Array.from(new Uint8Array(digest))
      .map(function (b) { return b.toString(16).padStart(2, "0"); })
      .join("");

    var canvasProof = "";
    try {
      var c = document.createElement("canvas");
      var ctx = c.getContext("2d");
      ctx.textBaseline = "top";
      ctx.font = "16px Arial";
      ctx.fillText("botshield", 2, 2);
      canvasProof = c.toDataURL();
    } catch (e) {}

    var automation = false;
    try {
      if (navigator.webdriver) automation = true;
      if (window.callPhantom || window._phantom || window.__nightmare) automation = true;
      if (document.__selenium_unwrapped || document.__webdriver_evaluate || document.__driver_evaluate) automation = true;
    } catch (e) {}

    var body = new URLSearchParams();
    body.set("token", "{{.Token}}");
    body.set("answer", answer);
    body.set("canvas", canvasProof);
    body.set("automation", String(automation));
    body.set("elapsed", String(Date.now() - start));

    var res = await fetch("{{.VerifyPath}}", {
      method: "POST",
      body: body,
      credentials: "same-origin",
    });
    if (res.redirected || res.ok) {
      window.location = res.url || "{{.RedirectPath}}";
    } else {
      document.body.textContent = "Verification failed.";
    }
  } catch (e) {
    document.body.textContent = "Verification failed.";
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
// page they wanted. The dedicated GET route in Handler() below is a
// standalone testing/demo convenience only; ROADMAP item 5's scoring
// engine is what will call this mid-proxy for a real request.
func (c *Challenge) Serve(w http.ResponseWriter, r *http.Request) {
	nonce, err := randomNonce()
	if err != nil {
		http.Error(w, "challenge unavailable", http.StatusInternalServerError)
		return
	}
	redirectPath := safeRedirectPath(r.URL.RequestURI())
	tok := c.token(nonce, redirectPath, time.Now())

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = challengePage.Execute(w, challengeData{
		Nonce:        nonce,
		Token:        tok,
		VerifyPath:   verifyPath,
		RedirectPath: redirectPath,
	})
}

// handleVerify checks the JS-computed answer against the token we
// issued. A wrong answer, bad token, or missing canvas proof gets a
// fresh puzzle back rather than a hard error — a real client that
// glitched should be able to just try again.
func (c *Challenge) handleVerify(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxVerifyBodyBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "request too large or malformed", http.StatusBadRequest)
		return
	}

	nonce, redirectPath, ok := c.parseToken(r.FormValue("token"))
	if !ok {
		c.Serve(w, r)
		return
	}
	answer := r.FormValue("answer")
	if subtle.ConstantTimeCompare([]byte(answer), []byte(expectedAnswer(nonce))) != 1 {
		c.Serve(w, r)
		return
	}
	if !validCanvasProof(r.FormValue("canvas")) {
		c.Serve(w, r)
		return
	}
	// A stock Selenium/Puppeteer/Playwright automation framework
	// exposes navigator.webdriver or similar globals even when the
	// browser itself is real (so canvas/sha256 pass) — ROADMAP item 6.
	// A tool built specifically to hide this (e.g. Patchright) defeats
	// it; that's a documented gap, not something this check claims to
	// catch (see docs/DECISIONS.md and docs/RESEARCH.md).
	if r.FormValue("automation") == "true" {
		c.Serve(w, r)
		return
	}

	c.setPassedCookie(w)
	http.Redirect(w, r, redirectPath, http.StatusSeeOther)
}

func (c *Challenge) setPassedCookie(w http.ResponseWriter) {
	payload := strconv.FormatInt(time.Now().Unix(), 10)
	value := payload + "." + c.sign(payload)
	http.SetCookie(w, &http.Cookie{
		Name:     passedCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   int(passedMaxAge.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// Passed reports whether r already carries a valid, unexpired
// "solved the challenge" cookie signed by c. Exposed for whatever
// wires the challenge into a real decision (ROADMAP item 5) to check
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
	issuedUnix, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}
	age := time.Since(time.Unix(issuedUnix, 0))
	return age >= 0 && age <= passedMaxAge
}

// Handler returns the HTTP surface for the challenge: GET issues a
// puzzle, POST verifies the answer. Not yet wired into the proxy's
// own request path automatically — deciding *who* gets challenged is
// ROADMAP item 5 (scoring engine); this is the mechanism it will call.
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
