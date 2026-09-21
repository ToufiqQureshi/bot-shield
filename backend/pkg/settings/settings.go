// Package settings stores each account's protection preferences
// (thresholds, challenge type, honeypot toggle). Like pkg/rules, this
// is preference storage the dashboard reads back — it is not yet
// wired into pkg/signals/score.go's live decision thresholds, which
// today are fixed in code (see docs/PROGRESS.md for that gap).
package settings

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Protection is one account's protection preferences.
type Protection struct {
	BlockThreshold     int    `json:"blockThreshold"`
	ChallengeThreshold int    `json:"challengeThreshold"`
	ChallengeType      string `json:"challengeType"`
	HoneypotEnabled    bool   `json:"honeypotEnabled"`
}

// ErrInvalid is returned when the block threshold would not exceed
// the challenge threshold — a configuration that makes every
// challenged request also a blocked one, silently turning off the
// challenge tier the operator thinks they still have.
var ErrInvalid = errors.New("settings: block threshold must exceed challenge threshold")

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Get returns ownerUserID's settings, or the schema defaults if the
// row doesn't exist yet (an account that never opened Protection
// Settings still gets a sane, real configuration back — never a blank
// page pretending to be data, per CLAUDE.md Section 27).
func (s *Store) Get(ctx context.Context, ownerUserID string) (*Protection, error) {
	if s.pool == nil {
		return nil, errors.New("settings: database not configured")
	}
	const q = `SELECT block_threshold, challenge_threshold, challenge_type, honeypot_enabled
		FROM protection_settings WHERE owner_user_id = $1`
	var p Protection
	err := s.pool.QueryRow(ctx, q, ownerUserID).Scan(&p.BlockThreshold, &p.ChallengeThreshold, &p.ChallengeType, &p.HoneypotEnabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return &Protection{BlockThreshold: 90, ChallengeThreshold: 50, ChallengeType: "pow", HoneypotEnabled: true}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("settings: reading: %w", err)
	}
	return &p, nil
}

// Upsert validates and stores ownerUserID's settings.
func (s *Store) Upsert(ctx context.Context, ownerUserID string, p Protection) error {
	if p.BlockThreshold <= p.ChallengeThreshold {
		return ErrInvalid
	}
	if s.pool == nil {
		return errors.New("settings: database not configured")
	}
	const q = `INSERT INTO protection_settings (owner_user_id, block_threshold, challenge_threshold, challenge_type, honeypot_enabled, updated_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (owner_user_id) DO UPDATE SET
			block_threshold = EXCLUDED.block_threshold,
			challenge_threshold = EXCLUDED.challenge_threshold,
			challenge_type = EXCLUDED.challenge_type,
			honeypot_enabled = EXCLUDED.honeypot_enabled,
			updated_at = now()`
	if _, err := s.pool.Exec(ctx, q, ownerUserID, p.BlockThreshold, p.ChallengeThreshold, p.ChallengeType, p.HoneypotEnabled); err != nil {
		return fmt.Errorf("settings: writing: %w", err)
	}
	return nil
}
