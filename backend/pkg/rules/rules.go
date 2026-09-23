// Package rules stores legacy account-wide mitigation rules. These remain
// shadow-only; activated tenant revisions live in pkg/tenantpolicy.
package rules

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/policy"
	"github.com/ToufiqQureshi/hakaishield/pkg/settings"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a rule ID doesn't exist for the caller.
var ErrNotFound = errors.New("rules: not found")

// ErrInvalidRule identifies a rule the live policy matcher cannot safely use.
var ErrInvalidRule = errors.New("rules: invalid rule")

// ManagedRule is a built-in detection layer the product always runs
// (pkg/signals). The dashboard lists these alongside custom rules so
// an operator can see everything protecting them in one place, but
// managed rules cannot be disabled from here — turning off JA4
// fingerprinting is a code change, not a toggle a visitor-facing
// account should have (CLAUDE.md Section 10).
type ManagedRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

// Managed returns the fixed list of always-on detection layers. It is
// a static description of pkg/signals, not a live read of it — keep
// this in sync by hand if a signal is added or removed there.
func Managed() []ManagedRule {
	return []ManagedRule{
		{ID: "mr-ja4", Name: "JA4 TLS Fingerprinting", Description: "Flags TLS handshakes matching known automation tooling.", Enabled: true},
		{ID: "mr-ua-mismatch", Name: "User-Agent Mismatch Detection", Description: "Flags a claimed browser that doesn't match its TLS handshake.", Enabled: true},
		{ID: "mr-velocity", Name: "Request Velocity Limiting", Description: "Flags request rates no human browsing session produces.", Enabled: true},
		{ID: "mr-honeypot", Name: "Honeypot Trap", Description: "Flags DOM-walking automation that follows a hidden bait link.", Enabled: true},
	}
}

// Condition is one clause of a custom rule, e.g. {"JA4 Fingerprint", "EQUALS", "t13d..."}.
type Condition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// CustomRule is an account-authored rule. Conditions is stored as JSON
// text rather than a normalized table — it's an opaque, ordered list
// the frontend built and the frontend reads back unchanged; giving it
// its own table would add relational structure nothing queries by.
type CustomRule struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Conditions []Condition `json:"conditions"`
	Action     string      `json:"action"`
	Enabled    bool        `json:"enabled"`
	CreatedAt  time.Time   `json:"createdAt"`
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func newRuleID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rules: generating rule id: %w", err)
	}
	return "rule_" + hex.EncodeToString(b), nil
}

// List returns every custom rule owned by ownerUserID, newest first.
func (s *Store) List(ctx context.Context, ownerUserID string) ([]CustomRule, error) {
	return s.list(ctx, ownerUserID, false)
}

// ListForPolicy bounds the rows materialized for request-path policy refresh.
func (s *Store) ListForPolicy(ctx context.Context, ownerUserID string) ([]CustomRule, error) {
	return s.list(ctx, ownerUserID, true)
}

func (s *Store) list(ctx context.Context, ownerUserID string, bounded bool) ([]CustomRule, error) {
	if s.pool == nil {
		return nil, errors.New("rules: database not configured")
	}
	q := `SELECT id, name, conditions_json, action, enabled, created_at
		FROM mitigation_rules WHERE owner_user_id = $1 ORDER BY created_at DESC, id DESC`
	if bounded {
		q += ` LIMIT 200`
	}
	rows, err := s.pool.Query(ctx, q, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("rules: listing: %w", err)
	}
	defer rows.Close()

	out := []CustomRule{}
	for rows.Next() {
		var r CustomRule
		var conditionsJSON string
		if err := rows.Scan(&r.ID, &r.Name, &conditionsJSON, &r.Action, &r.Enabled, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("rules: scanning row: %w", err)
		}
		r.Conditions, err = decodeConditions(conditionsJSON)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Create inserts a new custom rule owned by ownerUserID.
func (s *Store) Create(ctx context.Context, ownerUserID, name string, conditions []Condition, action string) (*CustomRule, error) {
	if strings.TrimSpace(name) == "" || len(name) > 128 {
		return nil, fmt.Errorf("%w: name must be 1 to 128 bytes", ErrInvalidRule)
	}
	liveBlockThreshold := signals.HardBlockThreshold()
	if err := validateRule(conditions, action, liveBlockThreshold); err != nil {
		return nil, err
	}
	if s.pool == nil {
		return nil, errors.New("rules: database not configured")
	}
	if action == string(policy.ActionDeceive) {
		protection, err := settings.NewStore(s.pool).Get(ctx, ownerUserID)
		if err != nil {
			return nil, fmt.Errorf("rules: reading protection settings: %w", err)
		}
		if protection.BlockThreshold > liveBlockThreshold {
			if err := validateRule(conditions, action, protection.BlockThreshold); err != nil {
				return nil, err
			}
		}
	}
	conditionsJSON, err := encodeConditions(conditions)
	if err != nil {
		return nil, err
	}
	id, err := newRuleID()
	if err != nil {
		return nil, err
	}

	const q = `INSERT INTO mitigation_rules (id, owner_user_id, name, conditions_json, action, enabled)
		VALUES ($1, $2, $3, $4, $5, true)
		RETURNING id, name, conditions_json, action, enabled, created_at`
	var r CustomRule
	var storedConditionsJSON string
	err = s.pool.QueryRow(ctx, q, id, ownerUserID, name, conditionsJSON, action).Scan(
		&r.ID, &r.Name, &storedConditionsJSON, &r.Action, &r.Enabled, &r.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("rules: inserting: %w", err)
	}
	r.Conditions = conditions
	return &r, nil
}

// maxPolicyRules bounds how many of an account's rules a single policy
// evaluation considers. Without a cap, one account accumulating rules
// over time would grow the per-request matcher cost without limit — the
// newest rules win, since List already orders by created_at DESC and
// Evaluate stops at the first match, so this keeps the ones most likely
// to reflect current intent (CLAUDE.md Section 15/19: bounded request-
// path work).
const maxPolicyRules = 200

// ToPolicy converts ownerUserID's stored rules into the pkg/policy form
// core.Guard's shadow evaluation reads. It is deliberately lossy in one
// direction: a rule that no longer satisfies ValidateRule — for example
// one saved before a guardrail existed, or with a field ValidateRule
// used to accept — is silently left out of the evaluated policy rather
// than returned as an error, because a stale row must never take down
// evaluation for every other rule the account has. The dashboard's own
// Create path is what stops new invalid rules from being saved.
func ToPolicy(ownerUserID string, rows []CustomRule, blockThreshold int) *policy.Policy {
	if len(rows) > maxPolicyRules {
		rows = rows[:maxPolicyRules]
	}
	out := make([]policy.Rule, 0, len(rows))
	for _, r := range rows {
		conditions := make([]policy.Condition, len(r.Conditions))
		for i, c := range r.Conditions {
			conditions[i] = policy.Condition{
				Field: policy.Field(c.Field), Operator: policy.Operator(c.Operator), Value: c.Value,
			}
		}
		rule := policy.Rule{
			ID: r.ID, Name: r.Name, Conditions: conditions,
			Action: policy.Action(r.Action), Enabled: r.Enabled,
		}
		if err := policy.ValidateRule(rule, blockThreshold); err != nil {
			continue
		}
		out = append(out, rule)
	}
	return &policy.Policy{OwnerUserID: ownerUserID, Rules: out}
}

// validateRule uses the same matcher contract that will later run in Guard.
// Unsupported dashboard fields are rejected rather than stored as rules that
// appear active but can never match a request.
func validateRule(conditions []Condition, action string, blockThreshold int) error {
	policyConditions := make([]policy.Condition, len(conditions))
	for i, c := range conditions {
		policyConditions[i] = policy.Condition{
			Field: policy.Field(c.Field), Operator: policy.Operator(c.Operator), Value: c.Value,
		}
	}
	if err := policy.ValidateRule(policy.Rule{Conditions: policyConditions, Action: policy.Action(action)}, blockThreshold); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidRule, err)
	}
	return nil
}

// SetEnabled toggles a rule the caller owns. It is scoped by
// ownerUserID as well as id so one account can never toggle another
// account's rule by guessing its ID.
func (s *Store) SetEnabled(ctx context.Context, ownerUserID, id string, enabled bool) error {
	if s.pool == nil {
		return errors.New("rules: database not configured")
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE mitigation_rules SET enabled = $1 WHERE id = $2 AND owner_user_id = $3`,
		enabled, id, ownerUserID)
	if err != nil {
		return fmt.Errorf("rules: updating: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func encodeConditions(c []Condition) (string, error) {
	if c == nil {
		c = []Condition{}
	}
	b, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("rules: encoding conditions: %w", err)
	}
	return string(b), nil
}

func decodeConditions(s string) ([]Condition, error) {
	var c []Condition
	if err := json.Unmarshal([]byte(s), &c); err != nil {
		return nil, fmt.Errorf("rules: decoding conditions: %w", err)
	}
	return c, nil
}
