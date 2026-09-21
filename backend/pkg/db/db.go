package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Domain is one row of the tenants table as the dashboard cares about
// it: a protected origin owned by one account. It reuses the tenants
// table rather than a separate "domains" table — a domain is exactly
// what tenant.Store already models (one host mapped to one origin),
// so a second table for the same entity would just be a sync problem.
type Domain struct {
	ID        string
	Host      string
	Target    string
	Name      string
	Status    string
	CreatedAt time.Time
}

var DB *pgxpool.Pool

func Init(databaseURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("unable to connect to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("unable to ping database: %w", err)
	}

	DB = pool
	return initSchema(ctx)
}

func initSchema(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS tenants (
		id VARCHAR(255) PRIMARY KEY,
		host VARCHAR(255) UNIQUE NOT NULL,
		target VARCHAR(255) NOT NULL,
		mode VARCHAR(50) NOT NULL,
		evidence_token VARCHAR(255)
	);
	ALTER TABLE tenants ADD COLUMN IF NOT EXISTS owner_user_id VARCHAR(255);
	ALTER TABLE tenants ADD COLUMN IF NOT EXISTS name VARCHAR(255);
	ALTER TABLE tenants ADD COLUMN IF NOT EXISTS status VARCHAR(50) NOT NULL DEFAULT 'active';
	ALTER TABLE tenants ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
	CREATE INDEX IF NOT EXISTS idx_tenants_owner ON tenants(owner_user_id);

	CREATE TABLE IF NOT EXISTS users (
		id VARCHAR(255) PRIMARY KEY,
		email VARCHAR(255) UNIQUE NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		full_name VARCHAR(255),
		company VARCHAR(255),
		onboarding_complete BOOLEAN NOT NULL DEFAULT false,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);

	CREATE TABLE IF NOT EXISTS mitigation_rules (
		id VARCHAR(255) PRIMARY KEY,
		owner_user_id VARCHAR(255) NOT NULL,
		name VARCHAR(255) NOT NULL,
		conditions_json TEXT NOT NULL,
		action VARCHAR(50) NOT NULL,
		enabled BOOLEAN NOT NULL DEFAULT true,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	CREATE INDEX IF NOT EXISTS idx_rules_owner ON mitigation_rules(owner_user_id);

	CREATE TABLE IF NOT EXISTS protection_settings (
		owner_user_id VARCHAR(255) PRIMARY KEY,
		block_threshold INT NOT NULL DEFAULT 90,
		challenge_threshold INT NOT NULL DEFAULT 50,
		challenge_type VARCHAR(50) NOT NULL DEFAULT 'pow',
		honeypot_enabled BOOLEAN NOT NULL DEFAULT true,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	`
	_, err := DB.Exec(ctx, schema)
	return err
}

func GetTenant(ctx context.Context, host string) (id, target, mode, evidenceToken string, err error) {
	if DB == nil {
		return "", "", "", "", fmt.Errorf("database not initialized")
	}

	query := `SELECT id, target, mode, COALESCE(evidence_token, '') FROM tenants WHERE host = $1 LIMIT 1`
	err = DB.QueryRow(ctx, query, host).Scan(&id, &target, &mode, &evidenceToken)
	return
}

// GetTenantByID is GetTenant's counterpart for lookups by internal ID
// rather than incoming Host header — the dashboard API knows a
// domain's ID, not the host a live request would carry.
func GetTenantByID(ctx context.Context, id string) (host, target, mode, evidenceToken string, err error) {
	if DB == nil {
		return "", "", "", "", fmt.Errorf("database not initialized")
	}

	query := `SELECT host, target, mode, COALESCE(evidence_token, '') FROM tenants WHERE id = $1 LIMIT 1`
	err = DB.QueryRow(ctx, query, id).Scan(&host, &target, &mode, &evidenceToken)
	return
}

// ListDomains returns every tenant owned by ownerUserID, newest first.
func ListDomains(ctx context.Context, ownerUserID string) ([]Domain, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	const q = `SELECT id, host, target, COALESCE(name, host), status, created_at
		FROM tenants WHERE owner_user_id = $1 ORDER BY created_at DESC`
	rows, err := DB.Query(ctx, q, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("listing domains: %w", err)
	}
	defer rows.Close()

	out := []Domain{}
	for rows.Next() {
		var d Domain
		if err := rows.Scan(&d.ID, &d.Host, &d.Target, &d.Name, &d.Status, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning domain row: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// CreateDomain provisions a new protected origin for ownerUserID. It
// only writes the routing row (host, target, mode) — actually taking
// live traffic for this host also requires tenant.Store to pick it up,
// which happens lazily on the first request via Store.fetchFromDB, or
// immediately if the process is restarted with this row already
// present. There is currently no in-process "add tenant now, no
// restart needed" path; see docs/PROGRESS.md.
func CreateDomain(ctx context.Context, id, ownerUserID, host, target, name string) (*Domain, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	const q = `INSERT INTO tenants (id, host, target, mode, owner_user_id, name, status)
		VALUES ($1, $2, $3, 'enforce', $4, $5, 'pending_verification')
		RETURNING id, host, target, name, status, created_at`
	var d Domain
	err := DB.QueryRow(ctx, q, id, host, target, ownerUserID, name).Scan(
		&d.ID, &d.Host, &d.Target, &d.Name, &d.Status, &d.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating domain: %w", err)
	}
	return &d, nil
}
