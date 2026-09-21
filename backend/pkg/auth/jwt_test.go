package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestNewIssuer_RejectsEmptySecret(t *testing.T) {
	if _, err := NewIssuer(nil); err == nil {
		t.Fatal("expected error for empty secret, got nil")
	}
	if _, err := NewIssuer([]byte{}); err == nil {
		t.Fatal("expected error for empty secret, got nil")
	}
}

func TestIssueVerify_RoundTrip(t *testing.T) {
	iss, err := NewIssuer([]byte("test-secret-32-bytes-long-enough"))
	if err != nil {
		t.Fatalf("NewIssuer: %v", err)
	}

	token, err := iss.Issue("usr_abc123")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	userID, err := iss.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if userID != "usr_abc123" {
		t.Fatalf("Verify returned userID %q, want %q", userID, "usr_abc123")
	}
}

func TestVerify_RejectsTamperedSignature(t *testing.T) {
	iss, _ := NewIssuer([]byte("secret-one"))
	other, _ := NewIssuer([]byte("secret-two"))

	token, err := iss.Issue("usr_abc123")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if _, err := other.Verify(token); err != ErrInvalidToken {
		t.Fatalf("Verify with wrong secret = %v, want ErrInvalidToken", err)
	}
}

func TestVerify_RejectsExpiredToken(t *testing.T) {
	iss, _ := NewIssuer([]byte("test-secret"))

	// Build a token whose ExpiresAt is already in the past, bypassing
	// Issue (which always sets a future expiry) so this test proves
	// Verify actually checks expiry rather than trusting Issue never
	// to produce one.
	expired := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		UserID: "usr_abc123",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		},
	})
	tokenStr, err := expired.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("signing expired token: %v", err)
	}

	if _, err := iss.Verify(tokenStr); err != ErrInvalidToken {
		t.Fatalf("Verify(expired) = %v, want ErrInvalidToken", err)
	}
}

func TestVerify_RejectsAlgNone(t *testing.T) {
	iss, _ := NewIssuer([]byte("test-secret"))

	// The classic "alg":"none" forgery: a token with no signature at
	// all that a naive verifier accepts as valid. WithValidMethods
	// must reject this outright.
	unsigned := jwt.NewWithClaims(jwt.SigningMethodNone, claims{
		UserID: "usr_attacker",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	tokenStr, err := unsigned.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("signing alg=none token: %v", err)
	}

	if _, err := iss.Verify(tokenStr); err != ErrInvalidToken {
		t.Fatalf("Verify(alg=none) = %v, want ErrInvalidToken", err)
	}
}

func TestVerify_RejectsGarbage(t *testing.T) {
	iss, _ := NewIssuer([]byte("test-secret"))
	if _, err := iss.Verify("not.a.jwt"); err != ErrInvalidToken {
		t.Fatalf("Verify(garbage) = %v, want ErrInvalidToken", err)
	}
	if _, err := iss.Verify(""); err != ErrInvalidToken {
		t.Fatalf("Verify(empty) = %v, want ErrInvalidToken", err)
	}
}

func TestVerify_RejectsMissingUserID(t *testing.T) {
	iss, _ := NewIssuer([]byte("test-secret"))
	empty := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	tokenStr, err := empty.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("signing: %v", err)
	}
	if _, err := iss.Verify(tokenStr); err != ErrInvalidToken {
		t.Fatalf("Verify(no uid claim) = %v, want ErrInvalidToken", err)
	}
}

func TestIssue_TokenHasThreeSegments(t *testing.T) {
	iss, _ := NewIssuer([]byte("test-secret"))
	token, err := iss.Issue("usr_abc123")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if got := strings.Count(token, "."); got != 2 {
		t.Fatalf("token has %d dots, want 2 (header.payload.signature)", got)
	}
}
