package api_test

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
	mux.HandleFunc("/auth/v1/.well-known/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]any{{
			"kid": ta.kid,
			"kty": "EC",
			"crv": "P-256",
			"x":   base64.RawURLEncoding.EncodeToString(ta.priv.X.Bytes()),
			"y":   base64.RawURLEncoding.EncodeToString(ta.priv.Y.Bytes()),
		}}})
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

func (ta *testAuth) sign(t *testing.T, subject string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.RegisteredClaims{
		Subject: subject, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	tok.Header["kid"] = ta.kid
	signed, err := tok.SignedString(ta.priv)
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}
	return signed
}
