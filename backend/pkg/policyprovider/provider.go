// Package policyprovider builds core.Guard's PolicyProvider from real
// storage: it resolves a request's tenant to its owning account, reads
// that account's mitigation rules and protection settings, and hands
// back an evaluated pkg/policy.Policy — bounded and cached so a live
// request never waits on a Postgres round trip.
package policyprovider

import (
	"context"
	"sync"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/policy"
	"github.com/ToufiqQureshi/hakaishield/pkg/rules"
	"github.com/ToufiqQureshi/hakaishield/pkg/settings"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
)

// tenantLookup is the subset of *tenant.Store this package needs. An
// interface here (rather than importing pkg/tenant's concrete type)
// keeps this package testable without a real tenant.Store, and pkg/tenant
// never needs to know this package exists.
type tenantLookup interface {
	OwnerUserID(tenantID string) (ownerUserID string, ok bool)
}

// ruleLister is the subset of *rules.Store this package needs.
type ruleLister interface {
	List(ctx context.Context, ownerUserID string) ([]rules.CustomRule, error)
}

// settingsGetter is the subset of *settings.Store this package needs.
type settingsGetter interface {
	Get(ctx context.Context, ownerUserID string) (*settings.Protection, error)
}

const (
	// cacheTTL bounds how stale an owner's evaluated policy can be after
	// a dashboard rule change. Short enough that a customer testing a
	// new rule sees it take effect quickly; long enough that busy
	// traffic doesn't turn every request into a Postgres round trip.
	cacheTTL = 30 * time.Second
	// negativeCacheTTL is how long a failed lookup (DB error, or a
	// tenant/owner with nothing configured) is remembered, so an outage
	// or a free-tier tenant with no rules can't turn every one of its
	// requests into a fresh query.
	negativeCacheTTL = 10 * time.Second
	// maxCacheEntries bounds the cache's memory: without a limit, an
	// attacker who can create tenants (or a very large customer base)
	// could grow this map without bound (CLAUDE.md Section 15/19).
	maxCacheEntries = 4096
	// dbTimeout bounds how long a cache-miss lookup can block the
	// request that triggered it — matches pkg/tenant's own DB timeout.
	dbTimeout = 2 * time.Second
)

type cacheEntry struct {
	policy    *policy.Policy // nil is a valid, cached "no policy" result
	expiresAt time.Time
}

// Provider resolves and caches each account's live policy. The zero
// value is not usable; construct with New.
type Provider struct {
	tenants  tenantLookup
	rules    ruleLister
	settings settingsGetter

	mu    sync.Mutex
	cache map[string]cacheEntry // keyed by ownerUserID
	now   func() time.Time
}

// New builds a Provider. store, ruleStore and settingsStore must be
// non-nil dashboard-backed implementations (or test doubles satisfying
// the interfaces above); a nil dependency makes ForTenant a permanent
// no-op rather than panicking, since a shadow-only feature failing safe
// is preferable to it crashing request handling.
func New(store tenantLookup, ruleStore ruleLister, settingsStore settingsGetter) *Provider {
	return &Provider{
		tenants:  store,
		rules:    ruleStore,
		settings: settingsStore,
		cache:    make(map[string]cacheEntry),
		now:      time.Now,
	}
}

// ForTenant is a core.PolicyProvider: it resolves tenantID to its owning
// account and returns that account's evaluated policy, or nil when the
// tenant, its owner, or its rules can't be resolved — matching
// core.Guard's contract that a nil policy is always a safe no-op.
//
// It never uses a caller-supplied owner or account identity: tenantID is
// the same validated identity core.Guard already derived from SNI/Host,
// and the owner lookup goes through tenantLookup, never a request field
// (CLAUDE.md Section 16/17).
func (p *Provider) ForTenant(tenantID string) *policy.Policy {
	if p == nil || p.tenants == nil || p.rules == nil || p.settings == nil {
		return nil
	}
	ownerUserID, ok := p.tenants.OwnerUserID(tenantID)
	if !ok || ownerUserID == "" {
		return nil
	}

	if pol, ok := p.cached(ownerUserID); ok {
		return pol
	}
	return p.load(ownerUserID)
}

func (p *Provider) cached(ownerUserID string) (*policy.Policy, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.cache[ownerUserID]
	if !ok || p.now().After(e.expiresAt) {
		return nil, false
	}
	return e.policy, true
}

func (p *Provider) load(ownerUserID string) *policy.Policy {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	rows, err := p.rules.List(ctx, ownerUserID)
	if err != nil {
		p.store(ownerUserID, nil, negativeCacheTTL)
		return nil
	}
	if len(rows) == 0 {
		p.store(ownerUserID, nil, cacheTTL)
		return nil
	}

	protection, err := p.settings.Get(ctx, ownerUserID)
	blockThreshold := signals.HardBlockThreshold()
	if err == nil && protection != nil && protection.BlockThreshold > blockThreshold {
		blockThreshold = protection.BlockThreshold
	}

	pol := rules.ToPolicy(ownerUserID, rows, blockThreshold)
	p.store(ownerUserID, pol, cacheTTL)
	return pol
}

func (p *Provider) store(ownerUserID string, pol *policy.Policy, ttl time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := p.now()
	if _, exists := p.cache[ownerUserID]; !exists && len(p.cache) >= maxCacheEntries {
		p.evictOldestLocked(now)
	}
	p.cache[ownerUserID] = cacheEntry{policy: pol, expiresAt: now.Add(ttl)}
}

// evictOldestLocked drops every expired entry, then — if the cache is
// still full — the single soonest-to-expire one. Called with p.mu held.
func (p *Provider) evictOldestLocked(now time.Time) {
	for k, e := range p.cache {
		if now.After(e.expiresAt) {
			delete(p.cache, k)
		}
	}
	if len(p.cache) < maxCacheEntries {
		return
	}
	var oldestKey string
	var oldestExpiry time.Time
	for k, e := range p.cache {
		if oldestKey == "" || e.expiresAt.Before(oldestExpiry) {
			oldestKey, oldestExpiry = k, e.expiresAt
		}
	}
	if oldestKey != "" {
		delete(p.cache, oldestKey)
	}
}
