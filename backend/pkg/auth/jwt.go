// Package auth verifies the session JWTs Supabase Auth issues for the
// dashboard. It no longer issues tokens itself — Supabase's own
// /auth/v1 endpoints (called directly from the frontend) do that now;
// this package only turns a presented token back into a user ID.
//
// Supabase signs new projects' tokens with ES256 against a per-project
// key pair, published at <project-url>/auth/v1/.well-known/jwks.json.
// Verifying against that public key means this backend never needs a
// shared secret at all — nothing here can forge a session even if the
// backend's own config leaks, which a shared HMAC secret could.
package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/observability"
	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken covers every way a presented token can fail to prove
// identity: bad signature, unknown key ID, expired, or malformed.
// Callers must not distinguish these — telling an attacker *why* a
// token was rejected only helps them craft the next one.
var ErrInvalidToken = errors.New("auth: invalid or expired token")

type claims struct {
	jwt.RegisteredClaims
}

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

type jwksResponse struct {
	Keys []jwk `json:"keys"`
}

// jwksTTL bounds how long a fetched key set is trusted before
// re-fetching. Supabase rotates signing keys rarely, but caching
// forever would mean a rotation silently breaks every session until
// this process restarts.
const jwksTTL = 1 * time.Hour

// Unknown key IDs are untrusted input. Remembering them briefly prevents a
// caller from forcing a JWKS request for every forged token while still
// allowing a real signing-key rotation to recover quickly.
const (
	unknownKidTTL  = 10 * time.Second
	maxUnknownKids = 1024
)

// Verifier validates dashboard session JWTs against a Supabase
// project's published JWKS. A zero-value Verifier is unusable;
// construct with NewVerifier.
type Verifier struct {
	jwksURL string
	client  *http.Client

	mu        sync.RWMutex
	keys      map[string]*ecdsa.PublicKey
	fetchedAt time.Time
	unknown   map[string]time.Time
	refreshMu sync.Mutex
}

// NewVerifier refuses an empty Supabase project URL rather than
// silently trusting nothing (and therefore rejecting every token) at
// runtime, where the failure would be a confusing "always 401"
// instead of a clear startup error.
func NewVerifier(supabaseURL string) (*Verifier, error) {
	supabaseURL = strings.TrimRight(strings.TrimSpace(supabaseURL), "/")
	if supabaseURL == "" {
		return nil, errors.New("auth: supabase URL must not be empty")
	}
	return &Verifier{
		jwksURL: supabaseURL + "/auth/v1/.well-known/jwks.json",
		client:  &http.Client{Timeout: 5 * time.Second},
		unknown: make(map[string]time.Time),
	}, nil
}

// Verify parses and validates a token, returning the user ID (the
// token's "sub" claim, Supabase's auth.users.id) it asserts.
func (v *Verifier) Verify(tokenString string) (userID string, err error) {
	parsed, err := jwt.ParseWithClaims(tokenString, &claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, ErrInvalidToken
		}
		kid, _ := t.Header["kid"].(string)
		key, err := v.key(kid)
		if err != nil {
			return nil, ErrInvalidToken
		}
		return key, nil
	}, jwt.WithValidMethods([]string{"ES256"}))
	if err != nil || !parsed.Valid {
		return "", ErrInvalidToken
	}

	c, ok := parsed.Claims.(*claims)
	if !ok || c.Subject == "" {
		return "", ErrInvalidToken
	}
	return c.Subject, nil
}

// key returns the public key for kid, fetching (or re-fetching, if the
// cache is stale or the id is unknown — covers a key rotation between
// fetches) the JWKS as needed.
func (v *Verifier) key(kid string) (*ecdsa.PublicKey, error) {
	now := time.Now()
	v.mu.RLock()
	key, ok := v.keys[kid]
	fresh := time.Since(v.fetchedAt) < jwksTTL
	unknownUntil, isUnknown := v.unknown[kid]
	v.mu.RUnlock()
	if ok && fresh {
		return key, nil
	}
	if isUnknown && now.Before(unknownUntil) {
		observability.Inc("auth_unknown_kid_cached_reject_total")
		return nil, fmt.Errorf("auth: unknown key id %q", kid)
	}

	// Serialize refreshes so a burst of tokens with attacker-chosen kids
	// cannot turn one cache miss into one outbound JWKS request per request.
	v.refreshMu.Lock()
	defer v.refreshMu.Unlock()

	// Another waiter may have refreshed or negatively cached this kid while
	// this call was blocked. Re-check before making another network request.
	now = time.Now()
	v.mu.RLock()
	key, ok = v.keys[kid]
	fresh = time.Since(v.fetchedAt) < jwksTTL
	unknownUntil, isUnknown = v.unknown[kid]
	v.mu.RUnlock()
	if ok && fresh {
		return key, nil
	}
	if isUnknown && now.Before(unknownUntil) {
		observability.Inc("auth_unknown_kid_cached_reject_total")
		return nil, fmt.Errorf("auth: unknown key id %q", kid)
	}

	if err := v.refresh(); err != nil {
		observability.Inc("auth_jwks_refresh_failure_total")
		return nil, err
	}

	v.mu.RLock()
	key, ok = v.keys[kid]
	v.mu.RUnlock()
	if !ok {
		v.rememberUnknownKid(kid, time.Now())
		observability.Inc("auth_unknown_kid_reject_total")
		return nil, fmt.Errorf("auth: unknown key id %q", kid)
	}
	return key, nil
}

func (v *Verifier) rememberUnknownKid(kid string, now time.Time) {
	v.mu.Lock()
	defer v.mu.Unlock()
	for existing, expiresAt := range v.unknown {
		if !now.Before(expiresAt) {
			delete(v.unknown, existing)
		}
	}
	if len(v.unknown) >= maxUnknownKids {
		var oldest string
		var oldestExpiry time.Time
		for existing, expiresAt := range v.unknown {
			if oldest == "" || expiresAt.Before(oldestExpiry) {
				oldest, oldestExpiry = existing, expiresAt
			}
		}
		if oldest != "" {
			delete(v.unknown, oldest)
		}
	}
	v.unknown[kid] = now.Add(unknownKidTTL)
}

func (v *Verifier) refresh() error {
	observability.Inc("auth_jwks_refresh_total")
	req, err := http.NewRequest(http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return fmt.Errorf("auth: building jwks request: %w", err)
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("auth: fetching jwks: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("auth: reading jwks response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("auth: jwks endpoint returned %d", resp.StatusCode)
	}

	var parsed jwksResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("auth: decoding jwks: %w", err)
	}

	keys := make(map[string]*ecdsa.PublicKey, len(parsed.Keys))
	for _, k := range parsed.Keys {
		if k.Kty != "EC" || k.Crv != "P-256" {
			continue // this project's key rotated to a curve/algorithm we don't handle yet
		}
		pub, err := decodeECPublicKey(k)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return errors.New("auth: jwks response had no usable ES256/P-256 keys")
	}

	v.mu.Lock()
	v.keys = keys
	v.fetchedAt = time.Now()
	v.mu.Unlock()
	observability.Inc("auth_jwks_refresh_success_total")
	return nil
}

func decodeECPublicKey(k jwk) (*ecdsa.PublicKey, error) {
	xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, fmt.Errorf("decoding x: %w", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, fmt.Errorf("decoding y: %w", err)
	}
	pub := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}
	if !pub.IsOnCurve(pub.X, pub.Y) {
		return nil, errors.New("public key point is not on P-256")
	}
	return pub, nil
}
