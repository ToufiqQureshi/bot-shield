// Package tenantpolicy stores immutable, tenant-scoped policy revisions.
package tenantpolicy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/policy"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("tenant policy: tenant or revision not found")
	ErrConflict = errors.New("tenant policy: version conflict")
	ErrInvalid  = errors.New("tenant policy: invalid document")
)

const MaxRules = 200

var ruleIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

type Document struct {
	Mode           string        `json:"mode"` // shadow or enforce
	Rules          []policy.Rule `json:"rules"`
	Allowlist      []string      `json:"allowlist,omitempty"` // CIDRs, verified against the resolved client IP
	ChallengeTheme string        `json:"challengeTheme,omitempty"`
	BlockMessage   string        `json:"blockMessage,omitempty"`
}

type Revision struct {
	TenantID    string    `json:"tenantId"`
	Version     int       `json:"version"`
	OwnerUserID string    `json:"-"`
	ActorUserID string    `json:"actorUserId"`
	CreatedAt   time.Time `json:"createdAt"`
	Document    Document  `json:"document"`
}

type RevisionSummary struct {
	Version     int       `json:"version"`
	ActorUserID string    `json:"actorUserId"`
	CreatedAt   time.Time `json:"createdAt"`
	Mode        string    `json:"mode"`
}

func (d Document) Validate() error {
	if d.Mode != "shadow" && d.Mode != "enforce" {
		return fmt.Errorf("%w: mode must be shadow or enforce", ErrInvalid)
	}
	if len(d.Rules) > MaxRules {
		return fmt.Errorf("%w: too many rules", ErrInvalid)
	}
	if len(d.Allowlist) > 100 {
		return fmt.Errorf("%w: too many allowlist entries", ErrInvalid)
	}
	for _, cidr := range d.Allowlist {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("%w: invalid allowlist CIDR", ErrInvalid)
		}
	}
	if d.ChallengeTheme != "" && d.ChallengeTheme != "ghost" && d.ChallengeTheme != "branded" {
		return fmt.Errorf("%w: unknown challenge theme", ErrInvalid)
	}
	if len(d.BlockMessage) > 256 || strings.ContainsAny(d.BlockMessage, "\r\n\x00") {
		return fmt.Errorf("%w: invalid block message", ErrInvalid)
	}
	ids := make(map[string]bool, len(d.Rules))
	for _, r := range d.Rules {
		if !ruleIDPattern.MatchString(r.ID) || strings.TrimSpace(r.Name) == "" || len(r.Name) > 128 || strings.ContainsAny(r.Name, "\r\n\x00") || ids[r.ID] {
			return fmt.Errorf("%w: invalid or duplicate rule id/name", ErrInvalid)
		}
		ids[r.ID] = true
		if err := policy.ValidateRule(r, signals.HardBlockThreshold()); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalid, err)
		}
		if r.Action == policy.ActionRateLimit && !hasVelocityCondition(r) {
			return fmt.Errorf("%w: rate limit needs a velocity signal", ErrInvalid)
		}
	}
	return nil
}

func hasVelocityCondition(r policy.Rule) bool {
	for _, c := range r.Conditions {
		if c.Field == policy.FieldSignal && (c.Value == "velocity_spike" || c.Value == "ja4_velocity_spike") {
			return true
		}
	}
	return false
}

type Store struct{ Pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{Pool: pool} }

// Latest returns the current document only when the authenticated owner owns the tenant.
func (s *Store) Latest(ctx context.Context, ownerID, tenantID string) (*Revision, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("tenant policy: database unavailable")
	}
	const q = `SELECT p.version,p.actor_user_id,p.created_at,p.document_json FROM tenant_policy_revisions p
		JOIN tenants t ON t.id=p.tenant_id AND t.owner_user_id=p.owner_user_id
		WHERE p.tenant_id=$1 AND p.owner_user_id=$2 ORDER BY p.version DESC LIMIT 1`
	return scanRevision(s.Pool.QueryRow(ctx, q, tenantID, ownerID), tenantID, ownerID)
}

func (s *Store) GetVersion(ctx context.Context, ownerID, tenantID string, version int) (*Revision, error) {
	if version < 1 {
		return nil, ErrNotFound
	}
	if s == nil || s.Pool == nil {
		return nil, errors.New("tenant policy: database unavailable")
	}
	const q = `SELECT p.version,p.actor_user_id,p.created_at,p.document_json FROM tenant_policy_revisions p JOIN tenants t ON t.id=p.tenant_id AND t.owner_user_id=p.owner_user_id WHERE p.tenant_id=$1 AND p.owner_user_id=$2 AND p.version=$3`
	return scanRevision(s.Pool.QueryRow(ctx, q, tenantID, ownerID, version), tenantID, ownerID)
}

// LoadForTenant is for the proxy: tenant identity came from the validated Host/SNI lookup.
func (s *Store) LoadForTenant(ctx context.Context, tenantID string) (*Revision, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("tenant policy: database unavailable")
	}
	const q = `SELECT p.version,p.actor_user_id,p.created_at,p.document_json,p.owner_user_id FROM tenant_policy_revisions p
		JOIN tenants t ON t.id=p.tenant_id AND t.owner_user_id=p.owner_user_id
		WHERE p.tenant_id=$1 ORDER BY p.version DESC LIMIT 1`
	var version int
	var actor, owner string
	var created time.Time
	var raw []byte
	if err := s.Pool.QueryRow(ctx, q, tenantID).Scan(&version, &actor, &created, &raw, &owner); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var doc Document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	return &Revision{TenantID: tenantID, Version: version, OwnerUserID: owner, ActorUserID: actor, CreatedAt: created, Document: doc}, nil
}

type rowScanner interface{ Scan(dest ...any) error }

func scanRevision(row rowScanner, tenantID, ownerID string) (*Revision, error) {
	var v int
	var actor string
	var created time.Time
	var raw []byte
	if err := row.Scan(&v, &actor, &created, &raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var doc Document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	return &Revision{TenantID: tenantID, Version: v, OwnerUserID: ownerID, ActorUserID: actor, CreatedAt: created, Document: doc}, nil
}

// Save appends one immutable revision. expectedVersion=0 creates the first revision.
// Locking the tenant row serializes writers and the owner check prevents cross-tenant writes.
func (s *Store) Save(ctx context.Context, ownerID, tenantID, actorID string, expectedVersion int, doc Document) (*Revision, error) {
	if err := doc.Validate(); err != nil {
		return nil, err
	}
	if expectedVersion < 0 {
		return nil, ErrConflict
	}
	if s == nil || s.Pool == nil {
		return nil, errors.New("tenant policy: database unavailable")
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id string
	if err = tx.QueryRow(ctx, `SELECT id FROM tenants WHERE id=$1 AND owner_user_id=$2 FOR UPDATE`, tenantID, ownerID).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var latest int
	if err = tx.QueryRow(ctx, `SELECT COALESCE(MAX(version),0) FROM tenant_policy_revisions WHERE tenant_id=$1`, tenantID).Scan(&latest); err != nil {
		return nil, err
	}
	if latest != expectedVersion {
		return nil, ErrConflict
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	var created time.Time
	version := latest + 1
	err = tx.QueryRow(ctx, `INSERT INTO tenant_policy_revisions (tenant_id,version,owner_user_id,actor_user_id,document_json)
		VALUES ($1,$2,$3,$4,$5) RETURNING created_at`, tenantID, version, ownerID, actorID, raw).Scan(&created)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &Revision{TenantID: tenantID, Version: version, OwnerUserID: ownerID, ActorUserID: actorID, CreatedAt: created, Document: doc}, nil
}

// Rollback creates a new revision from an older immutable document.
func (s *Store) Rollback(ctx context.Context, ownerID, tenantID, actorID string, expectedVersion, targetVersion int) (*Revision, error) {
	if targetVersion < 1 {
		return nil, ErrNotFound
	}
	if s == nil || s.Pool == nil {
		return nil, errors.New("tenant policy: database unavailable")
	}
	var raw []byte
	err := s.Pool.QueryRow(ctx, `SELECT p.document_json FROM tenant_policy_revisions p JOIN tenants t ON t.id=p.tenant_id AND t.owner_user_id=p.owner_user_id WHERE p.tenant_id=$1 AND p.owner_user_id=$2 AND p.version=$3`, tenantID, ownerID, targetVersion).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var doc Document
	if err = json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	// A rollback is a new candidate revision. It must pass shadow measurement
	// again, even when the historical revision had been active.
	doc.Mode = "shadow"
	// Save rechecks ownership, validates today's guardrails, and serializes the write.
	return s.Save(ctx, ownerID, tenantID, actorID, expectedVersion, doc)
}

func (s *Store) History(ctx context.Context, ownerID, tenantID string) ([]RevisionSummary, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("tenant policy: database unavailable")
	}
	rows, err := s.Pool.Query(ctx, `SELECT p.version,p.actor_user_id,p.created_at,p.document_json->>'mode' FROM tenant_policy_revisions p JOIN tenants t ON t.id=p.tenant_id AND t.owner_user_id=p.owner_user_id WHERE p.tenant_id=$1 AND p.owner_user_id=$2 ORDER BY p.version DESC LIMIT 100`, tenantID, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RevisionSummary{}
	for rows.Next() {
		var r RevisionSummary
		if err := rows.Scan(&r.Version, &r.ActorUserID, &r.CreatedAt, &r.Mode); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
