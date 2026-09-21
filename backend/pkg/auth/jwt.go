// Package auth issues and verifies the JWTs that authenticate dashboard
// API requests. It knows nothing about passwords or the users table —
// that is pkg/account's job — only about turning a user ID into a
// signed token and back.
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// tokenTTL bounds how long a stolen token stays useful. Short enough
// that a leaked token is not a standing compromise, long enough that a
// dashboard session does not re-prompt for credentials mid-use.
const tokenTTL = 24 * time.Hour

// ErrInvalidToken covers every way a presented token can fail to prove
// identity: bad signature, wrong algorithm, expired, or malformed.
// Callers must not distinguish these — telling an attacker *why* a
// token was rejected only helps them craft the next one.
var ErrInvalidToken = errors.New("invalid or expired token")

type claims struct {
	UserID string `json:"uid"`
	jwt.RegisteredClaims
}

// Issuer signs and verifies dashboard session tokens with one HMAC
// secret. A zero-value Issuer is unusable; construct with NewIssuer.
type Issuer struct {
	secret []byte
}

// NewIssuer refuses an empty secret rather than silently signing every
// token with an empty key, which would let anyone forge a session.
func NewIssuer(secret []byte) (*Issuer, error) {
	if len(secret) == 0 {
		return nil, errors.New("auth: jwt secret must not be empty")
	}
	return &Issuer{secret: secret}, nil
}

// Issue returns a signed token asserting the given user ID for tokenTTL.
func (i *Issuer) Issue(userID string) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
		},
	})
	return token.SignedString(i.secret)
}

// Verify parses and validates a token, returning the user ID it
// asserts. It rejects anything not signed with HMAC by this issuer's
// secret (the "alg":"none" and algorithm-confusion families of attack),
// matching jwt/v5's documented safe-parsing pattern.
func (i *Issuer) Verify(tokenString string) (userID string, err error) {
	parsed, err := jwt.ParseWithClaims(tokenString, &claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return i.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !parsed.Valid {
		return "", ErrInvalidToken
	}

	c, ok := parsed.Claims.(*claims)
	if !ok || c.UserID == "" {
		return "", ErrInvalidToken
	}
	return c.UserID, nil
}
