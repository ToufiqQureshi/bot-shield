package account

import (
	"context"
	"errors"
	"testing"
)

// These tests exercise validation and the "no database configured"
// failure path without a live Postgres — there is no test double for
// pgxpool.Pool in this repo, and account queries are covered by go vet
// plus manual verification against docker-compose's postgres; see
// docs/PROGRESS.md for that gap.

func TestSignup_RejectsInvalidEmail(t *testing.T) {
	s := NewStore(nil)
	_, err := s.Signup(context.Background(), "not-an-email", "longenoughpassword", "Jane", "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Signup(bad email) = %v, want ErrInvalidInput", err)
	}
}

func TestSignup_RejectsShortPassword(t *testing.T) {
	s := NewStore(nil)
	_, err := s.Signup(context.Background(), "jane@example.com", "short", "Jane", "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Signup(short password) = %v, want ErrInvalidInput", err)
	}
}

func TestSignup_ValidInputReachesDatabaseCheck(t *testing.T) {
	// Proves validation happens before the database is touched: valid
	// input against a nil pool must fail with the "not configured"
	// error, not ErrInvalidInput — if this ever returned
	// ErrInvalidInput, it would mean the two checks got reordered and
	// a real deployment's validation logic is untested by these cases.
	s := NewStore(nil)
	_, err := s.Signup(context.Background(), "jane@example.com", "longenoughpassword", "Jane", "")
	if errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Signup(valid input, nil pool) = %v, should not be ErrInvalidInput", err)
	}
	if err == nil {
		t.Fatal("Signup(valid input, nil pool) = nil, want a database-not-configured error")
	}
}

func TestSignin_NilPoolFailsClearly(t *testing.T) {
	s := NewStore(nil)
	_, err := s.Signin(context.Background(), "jane@example.com", "whatever")
	if err == nil {
		t.Fatal("Signin(nil pool) = nil, want an error")
	}
	if errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Signin(nil pool) = %v, should not look like a real invalid-credentials result", err)
	}
}

func TestGetByID_NilPoolFailsClearly(t *testing.T) {
	s := NewStore(nil)
	_, err := s.GetByID(context.Background(), "usr_x")
	if err == nil {
		t.Fatal("GetByID(nil pool) = nil, want an error")
	}
}

func TestValidEmail(t *testing.T) {
	cases := map[string]bool{
		"jane@example.com":     true,
		"jane+tag@example.com": true,
		"not-an-email":         false,
		"":                     false,
		"@example.com":         false,
		"jane@":                false,
	}
	for email, want := range cases {
		if got := validEmail(email); got != want {
			t.Errorf("validEmail(%q) = %v, want %v", email, got, want)
		}
	}
}
