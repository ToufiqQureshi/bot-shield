package challenge_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// FuzzVerifyBody posts arbitrary bodies to the verify endpoint. Without a
// token we issued, no body may ever pass the challenge, and malformed or
// oversized input must be rejected cleanly rather than crash the proxy.
func FuzzVerifyBody(f *testing.F) {
	for _, seed := range []string{
		"",
		"token=&nonce=&answer=",
		"token=a.b.c&answer=1&canvas=data:image/png;base64,AAAA",
		"%zz=%",
		strings.Repeat("a=b&", 20000),
	} {
		f.Add(seed)
	}
	h := newChallenge(f).Handler()

	f.Fuzz(func(t *testing.T, body string) {
		req := httptest.NewRequest(http.MethodPost, verifyPath, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusForbidden {
			t.Fatalf("forged body got status %d, want 400 or 403", rec.Code)
		}
		if len(rec.Result().Cookies()) != 0 {
			t.Fatal("forged body was issued a cookie")
		}
	})
}
