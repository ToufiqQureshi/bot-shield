package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// testJWKS spins up a fake Supabase JWKS endpoint backed by a freshly
// generated ES256 key pair, so tests can sign real tokens against it
// without touching the network or a real Supabase project.
type testJWKS struct {
	server   *httptest.Server
	priv     *ecdsa.PrivateKey
	kid      string
	requests atomic.Int32
	handler  func(w http.ResponseWriter, r *http.Request)
}

func newTestJWKS(t *testing.T) *testJWKS {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating test key: %v", err)
	}

	tj := &testJWKS{priv: priv, kid: "test-key-1"}
	tj.handler = func(w http.ResponseWriter, r *http.Request) {
		tj.requests.Add(1)
		body := map[string]any{
			"keys": []map[string]any{
				{
					"kid": tj.kid,
					"kty": "EC",
					"crv": "P-256",
					"x":   base64.RawURLEncoding.EncodeToString(tj.priv.PublicKey.X.Bytes()),
					"y":   base64.RawURLEncoding.EncodeToString(tj.priv.PublicKey.Y.Bytes()),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(body)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/v1/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) { tj.handler(w, r) })
	tj.server = httptest.NewServer(mux)
	t.Cleanup(tj.server.Close)
	return tj
}

func (tj *testJWKS) verifier(t *testing.T) *Verifier {
	t.Helper()
	v, err := NewVerifier(tj.server.URL)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	return v
}

func (tj *testJWKS) sign(t *testing.T, subject string, expiresIn time.Duration) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
		},
	})
	tok.Header["kid"] = tj.kid
	s, err := tok.SignedString(tj.priv)
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}
	return s
}

func TestNewVerifier_RejectsEmptyURL(t *testing.T) {
	if _, err := NewVerifier(""); err == nil {
		t.Fatal("expected error for empty supabase URL, got nil")
	}
	if _, err := NewVerifier("   "); err == nil {
		t.Fatal("expected error for whitespace-only supabase URL, got nil")
	}
}

func TestVerify_RoundTrip(t *testing.T) {
	tj := newTestJWKS(t)
	v := tj.verifier(t)

	token := tj.sign(t, "11111111-1111-1111-1111-111111111111", time.Hour)
	userID, err := v.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if userID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("Verify returned %q, want the signed subject", userID)
	}
}

func TestVerify_RejectsExpiredToken(t *testing.T) {
	tj := newTestJWKS(t)
	v := tj.verifier(t)

	token := tj.sign(t, "usr", -time.Hour) // already expired
	if _, err := v.Verify(token); err != ErrInvalidToken {
		t.Fatalf("Verify(expired) = %v, want ErrInvalidToken", err)
	}
}

func TestVerify_RejectsTokenFromDifferentKey(t *testing.T) {
	tj := newTestJWKS(t)
	v := tj.verifier(t)

	other := newTestJWKS(t) // different key, own kid
	forged := other.sign(t, "attacker", time.Hour)

	if _, err := v.Verify(forged); err != ErrInvalidToken {
		t.Fatalf("Verify(forged) = %v, want ErrInvalidToken", err)
	}
}

func TestVerify_RejectsUnknownKeyID(t *testing.T) {
	tj := newTestJWKS(t)
	v := tj.verifier(t)

	tok := jwt.NewWithClaims(jwt.SigningMethodES256, claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "usr",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	tok.Header["kid"] = "some-other-kid-not-in-jwks"
	s, err := tok.SignedString(tj.priv)
	if err != nil {
		t.Fatalf("signing: %v", err)
	}

	if _, err := v.Verify(s); err != ErrInvalidToken {
		t.Fatalf("Verify(unknown kid) = %v, want ErrInvalidToken", err)
	}
}

func TestVerify_UnknownKeyIDRefreshIsBounded(t *testing.T) {
	tj := newTestJWKS(t)
	v := tj.verifier(t)

	for i := 0; i < 3; i++ {
		tok := jwt.NewWithClaims(jwt.SigningMethodES256, claims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "usr",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		})
		tok.Header["kid"] = "attacker-chosen-kid"
		signed, err := tok.SignedString(tj.priv)
		if err != nil {
			t.Fatalf("signing: %v", err)
		}
		if _, err := v.Verify(signed); err != ErrInvalidToken {
			t.Fatalf("Verify(unknown kid) = %v, want ErrInvalidToken", err)
		}
	}
	if got := tj.requests.Load(); got != 1 {
		t.Fatalf("JWKS fetched %d times for repeated unknown kid, want 1", got)
	}
}

func TestVerify_RejectsAlgNone(t *testing.T) {
	tj := newTestJWKS(t)
	v := tj.verifier(t)

	unsigned := jwt.NewWithClaims(jwt.SigningMethodNone, claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "attacker",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	s, err := unsigned.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("signing alg=none token: %v", err)
	}

	if _, err := v.Verify(s); err != ErrInvalidToken {
		t.Fatalf("Verify(alg=none) = %v, want ErrInvalidToken", err)
	}
}

func TestVerify_RejectsGarbage(t *testing.T) {
	tj := newTestJWKS(t)
	v := tj.verifier(t)

	if _, err := v.Verify("not.a.jwt"); err != ErrInvalidToken {
		t.Fatalf("Verify(garbage) = %v, want ErrInvalidToken", err)
	}
	if _, err := v.Verify(""); err != ErrInvalidToken {
		t.Fatalf("Verify(empty) = %v, want ErrInvalidToken", err)
	}
}

func TestVerify_RejectsMissingSubject(t *testing.T) {
	tj := newTestJWKS(t)
	v := tj.verifier(t)

	token := tj.sign(t, "", time.Hour)
	if _, err := v.Verify(token); err != ErrInvalidToken {
		t.Fatalf("Verify(no subject) = %v, want ErrInvalidToken", err)
	}
}

func TestVerify_RecoversFromKeyRotation(t *testing.T) {
	// A key rotated on Supabase's side between this backend's fetches
	// must not lock out every subsequent login — the next Verify call
	// for the new kid should trigger a fresh fetch rather than trusting
	// a stale cache forever.
	tj := newTestJWKS(t)
	v := tj.verifier(t)

	oldToken := tj.sign(t, "usr_old", time.Hour)
	if _, err := v.Verify(oldToken); err != nil {
		t.Fatalf("Verify(old key) before rotation: %v", err)
	}

	// Rotate: new key pair, new kid, same JWKS endpoint.
	newPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating rotated key: %v", err)
	}
	tj.priv = newPriv
	tj.kid = "test-key-2"

	newToken := tj.sign(t, "usr_new", time.Hour)
	userID, err := v.Verify(newToken)
	if err != nil {
		t.Fatalf("Verify(new key) after rotation: %v", err)
	}
	if userID != "usr_new" {
		t.Fatalf("Verify(new key) = %q, want %q", userID, "usr_new")
	}
}
