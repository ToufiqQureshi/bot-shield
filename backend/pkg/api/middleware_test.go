package api

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/auth"
	"github.com/golang-jwt/jwt/v5"
)

// testAuth spins up a fake Supabase JWKS endpoint and returns a
// Verifier wired to it, plus a way to sign tokens against the same
// key — mirrors pkg/auth's own test helper, duplicated here since it
// needs to stay unexported within each package.
type testAuth struct {
	server *httptest.Server
	priv   *ecdsa.PrivateKey
	kid    string
}

func newTestAuth(t *testing.T) *testAuth {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating test key: %v", err)
	}
	ta := &testAuth{priv: priv, kid: "test-key-1"}

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/v1/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{{
				"kid": ta.kid,
				"kty": "EC",
				"crv": "P-256",
				"x":   base64.RawURLEncoding.EncodeToString(priv.PublicKey.X.Bytes()),
				"y":   base64.RawURLEncoding.EncodeToString(priv.PublicKey.Y.Bytes()),
			}},
		})
	})
	ta.server = httptest.NewServer(mux)
	t.Cleanup(ta.server.Close)
	return ta
}

func (ta *testAuth) verifier(t *testing.T) *auth.Verifier {
	t.Helper()
	v, err := auth.NewVerifier(ta.server.URL)
	if err != nil {
		t.Fatalf("auth.NewVerifier: %v", err)
	}
	return v
}

type testClaims struct {
	jwt.RegisteredClaims
}

func (ta *testAuth) sign(t *testing.T, subject string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, testClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	tok.Header["kid"] = ta.kid
	s, err := tok.SignedString(ta.priv)
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}
	return s
}

func TestRequireAuth_RejectsMissingToken(t *testing.T) {
	v := newTestAuth(t).verifier(t)
	handlerRan := false
	h := RequireAuth(v, func(w http.ResponseWriter, r *http.Request) { handlerRan = true })

	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if handlerRan {
		t.Fatal("wrapped handler ran without a token")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuth_RejectsInvalidToken(t *testing.T) {
	v := newTestAuth(t).verifier(t)
	handlerRan := false
	h := RequireAuth(v, func(w http.ResponseWriter, r *http.Request) { handlerRan = true })

	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	rec := httptest.NewRecorder()
	h(rec, req)

	if handlerRan {
		t.Fatal("wrapped handler ran with a forged token")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuth_AcceptsValidTokenAndSetsUserID(t *testing.T) {
	ta := newTestAuth(t)
	v := ta.verifier(t)
	token := ta.sign(t, "usr_real_user")

	var gotUserID string
	h := RequireAuth(v, func(w http.ResponseWriter, r *http.Request) {
		gotUserID = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotUserID != "usr_real_user" {
		t.Fatalf("UserIDFromContext = %q, want %q", gotUserID, "usr_real_user")
	}
}

func TestRequireAuth_TokenFromDifferentKeyRejected(t *testing.T) {
	v := newTestAuth(t).verifier(t)
	otherProject := newTestAuth(t) // different key pair, simulating a different Supabase project
	token := otherProject.sign(t, "usr_attacker")

	handlerRan := false
	h := RequireAuth(v, func(w http.ResponseWriter, r *http.Request) { handlerRan = true })

	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h(rec, req)

	if handlerRan {
		t.Fatal("wrapped handler ran with a token signed by a different project's key")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuth_OptionsPassesWithoutToken(t *testing.T) {
	v := newTestAuth(t).verifier(t)
	handlerRan := false
	h := RequireAuth(v, func(w http.ResponseWriter, r *http.Request) { handlerRan = true })

	req := httptest.NewRequest(http.MethodOptions, "/anything", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if handlerRan {
		t.Fatal("wrapped handler should not run for an OPTIONS preflight")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("OPTIONS status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Fatal("OPTIONS response missing CORS headers")
	}
}
