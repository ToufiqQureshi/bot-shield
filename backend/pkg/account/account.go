// Package account owns the users table: creating accounts, verifying
// credentials, and looking a user up by ID. It never sees or issues
// JWTs — pkg/auth does that — so a bug in token handling cannot also
// corrupt password storage, and vice versa.
package account

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// ErrEmailTaken is returned by Signup when the address is already
// registered. Deliberately the same shape of error whether the email
// exists or the format is bad would help an attacker enumerate
// accounts, so Signup keeps this one distinct from ErrInvalidInput on
// purpose — the HTTP layer decides what, if anything, to say about it.
var ErrEmailTaken = errors.New("account: email already registered")

// ErrInvalidInput covers a malformed email or a password that fails
// the minimum-length rule.
var ErrInvalidInput = errors.New("account: invalid email or password")

// ErrInvalidCredentials is returned by Signin for both "no such user"
// and "wrong password" — never let a caller distinguish the two, or
// signin becomes an account-enumeration oracle.
var ErrInvalidCredentials = errors.New("account: invalid email or password")

// ErrNotFound is returned by GetByID when no user has that ID.
var ErrNotFound = errors.New("account: user not found")

const minPasswordLength = 8

// User is what the rest of the backend is allowed to know about an
// account. PasswordHash is deliberately absent — nothing outside this
// package needs it, so nothing outside this package can leak it.
type User struct {
	ID                 string
	Email              string
	FullName           string
	Company            string
	OnboardingComplete bool
	CreatedAt          time.Time
}

// Store wraps the users table. A nil Pool makes every method return an
// error instead of panicking, so a misconfigured deployment (DB not
// wired up) fails loudly on first use rather than crashing the process.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func newUserID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("account: generating user id: %w", err)
	}
	return "usr_" + hex.EncodeToString(b), nil
}

func validEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

// Signup creates a new account. The password is hashed with bcrypt
// before it ever reaches SQL or a log line; only the hash is stored.
func (s *Store) Signup(ctx context.Context, email, password, fullName, company string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !validEmail(email) || len(password) < minPasswordLength {
		return nil, ErrInvalidInput
	}
	if s.pool == nil {
		return nil, errors.New("account: database not configured")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("account: hashing password: %w", err)
	}

	id, err := newUserID()
	if err != nil {
		return nil, err
	}

	const q = `
		INSERT INTO users (id, email, password_hash, full_name, company)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, email, full_name, company, onboarding_complete, created_at`
	var u User
	err = s.pool.QueryRow(ctx, q, id, email, string(hash), fullName, company).Scan(
		&u.ID, &u.Email, &u.FullName, &u.Company, &u.OnboardingComplete, &u.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("account: inserting user: %w", err)
	}
	return &u, nil
}

// Signin verifies credentials and returns the account on success.
// bcrypt.CompareHashAndPassword runs at the same cost regardless of
// whether the email exists (we still generate the lookup query either
// way), which keeps this close to constant-time against a timing
// attack that tries to distinguish "no such user" from "wrong password".
func (s *Store) Signin(ctx context.Context, email, password string) (*User, error) {
	if s.pool == nil {
		return nil, errors.New("account: database not configured")
	}
	email = strings.ToLower(strings.TrimSpace(email))

	const q = `SELECT id, email, password_hash, full_name, company, onboarding_complete, created_at
		FROM users WHERE email = $1`
	var u User
	var hash string
	err := s.pool.QueryRow(ctx, q, email).Scan(&u.ID, &u.Email, &hash, &u.FullName, &u.Company, &u.OnboardingComplete, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// Still pay bcrypt's cost so "no such user" and "wrong
		// password" take the same time from the caller's side.
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$"+strings.Repeat("x", 53)), []byte(password))
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("account: looking up user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	return &u, nil
}

// GetByID looks up an account by ID, for the /auth/me endpoint and for
// authenticating subsequent requests by the JWT's subject claim.
func (s *Store) GetByID(ctx context.Context, id string) (*User, error) {
	if s.pool == nil {
		return nil, errors.New("account: database not configured")
	}
	const q = `SELECT id, email, full_name, company, onboarding_complete, created_at
		FROM users WHERE id = $1`
	var u User
	err := s.pool.QueryRow(ctx, q, id).Scan(&u.ID, &u.Email, &u.FullName, &u.Company, &u.OnboardingComplete, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("account: looking up user: %w", err)
	}
	return &u, nil
}

// CompleteOnboarding marks the account as having finished the
// onboarding questionnaire, so /auth/me stops telling the frontend to
// redirect back into it.
func (s *Store) CompleteOnboarding(ctx context.Context, id string) error {
	if s.pool == nil {
		return errors.New("account: database not configured")
	}
	tag, err := s.pool.Exec(ctx, `UPDATE users SET onboarding_complete = true WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("account: completing onboarding: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func isUniqueViolation(err error) bool {
	// pgx exposes the SQLSTATE via its own PgError type; 23505 is
	// Postgres's "unique_violation". Matching on the code (not the
	// message text) survives locale and driver-version changes.
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}
