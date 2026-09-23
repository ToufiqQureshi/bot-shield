package challenge

import (
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// The attempt and passed cookies are the two pieces of signed per-client
// state the challenge keeps. Both are host-bound, HttpOnly, Secure and
// signed with the same HMAC secret as the challenge token, so a visitor
// can drop either (losing only their own progress/trust) but cannot forge,
// inflate, or extend either.

// attemptCookie remembers consecutive failed solves on this client so the
// next challenge can be harder. It is signed, so a visitor can drop it
// (losing only their own escalation) but cannot inflate it to punish
// someone else, and it is host-bound like every other challenge cookie.
const attemptCookie = "X-HakaiShield-Attempt"
const attemptMaxAge = 15 * time.Minute

// attempts returns how many consecutive failed solves this client is
// carrying, from its signed cookie. A missing, forged, or host-mismatched
// cookie is simply zero attempts.
func (c *Challenge) attempts(r *http.Request) int {
	ck, err := r.Cookie(attemptCookie)
	if err != nil {
		return 0
	}
	parts := strings.SplitN(ck.Value, ".", 2)
	if len(parts) != 2 {
		return 0
	}
	if subtle.ConstantTimeCompare([]byte(parts[1]), []byte(c.sign(parts[0]))) != 1 {
		return 0
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0
	}
	fields := strings.SplitN(string(raw), "|", 2)
	if len(fields) != 2 || fields[1] != canonicalHost(r.Host) {
		return 0
	}
	n, err := strconv.Atoi(fields[0])
	if err != nil || n < 0 || n > maxAttempts {
		return 0
	}
	return n
}

// recordAttempt increments the signed attempt counter after a failed
// solve, so the visitor's next challenge is one step harder.
func (c *Challenge) recordAttempt(w http.ResponseWriter, r *http.Request) {
	next := c.attempts(r) + 1
	if next > maxAttempts {
		next = maxAttempts
	}
	payload := base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(next) + "|" + canonicalHost(r.Host)))
	http.SetCookie(w, &http.Cookie{
		Name:     attemptCookie,
		Value:    payload + "." + c.sign(payload),
		Path:     "/",
		MaxAge:   int(attemptMaxAge.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearAttempts drops the escalation after a successful solve, so a
// visitor who eventually passes starts from a clean slate.
func (c *Challenge) clearAttempts(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     attemptCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// setPassedCookie grants the passed cookie with a MaxAge equal to the
// trust window that difficulty earns. The difficulty itself rides inside
// the signed payload so Passed() can re-derive the window; the client
// cannot edit it without breaking the signature.
func (c *Challenge) setPassedCookie(w http.ResponseWriter, host string, difficulty int) {
	rawPayload := strings.Join([]string{strconv.FormatInt(time.Now().Unix(), 10), strconv.Itoa(difficulty), canonicalHost(host)}, "|")
	payload := base64.RawURLEncoding.EncodeToString([]byte(rawPayload))
	http.SetCookie(w, &http.Cookie{
		Name:     passedCookie,
		Value:    payload + "." + c.sign(payload),
		Path:     "/",
		MaxAge:   int(passedTrustWindow(difficulty).Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}
