// Package policyprovider builds core.Guard's PolicyProvider from real
// storage: it prefers a versioned tenant revision, otherwise falls back to
// the owner's legacy shadow rules. Production lookups refresh through a
// bounded background loader, so a cold cache never waits on Postgres.
package policyprovider

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/policy"
	"github.com/ToufiqQureshi/hakaishield/pkg/rules"
	"github.com/ToufiqQureshi/hakaishield/pkg/settings"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenantpolicy"
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
	tenants        tenantLookup
	rules          ruleLister
	settings       settingsGetter
	tenantPolicies interface {
		LoadForTenant(context.Context, string) (*tenantpolicy.Revision, error)
	}

	mu          sync.Mutex
	cache       map[string]cacheEntry // keyed by ownerUserID
	tenantCache map[string]cacheEntry
	now         func() time.Time
	async       bool
	loading     map[string]bool
	loadSlots   chan struct{}
	epoch       uint64
}

// New builds a Provider. store, ruleStore and settingsStore must be
// non-nil dashboard-backed implementations (or test doubles satisfying
// the interfaces above); a nil dependency makes ForTenant a permanent
// no-op rather than panicking, since a shadow-only feature failing safe
// is preferable to it crashing request handling.
func New(store tenantLookup, ruleStore ruleLister, settingsStore settingsGetter) *Provider {
	return &Provider{
		tenants:     store,
		rules:       ruleStore,
		settings:    settingsStore,
		cache:       make(map[string]cacheEntry),
		tenantCache: make(map[string]cacheEntry),
		now:         time.Now,
		loading:     make(map[string]bool),
		loadSlots:   make(chan struct{}, 32),
	}
}

// WithTenantPolicies installs the versioned tenant store. An explicit tenant
// revision takes precedence over legacy account-wide shadow rules.
func (p *Provider) WithTenantPolicies(store interface {
	LoadForTenant(context.Context, string) (*tenantpolicy.Revision, error)
}) *Provider {
	p.tenantPolicies = store
	p.async = true
	return p
}

func (p *Provider) InvalidateTenant(tenantID string) {
	p.mu.Lock()
	p.epoch++
	delete(p.tenantCache, tenantID)
	p.mu.Unlock()
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
	if p.async {
		return p.forTenantAsync(tenantID, ownerUserID)
	}
	if p.tenantPolicies != nil {
		if pol, ok := p.cachedTenant(tenantID); ok && pol != nil {
			return pol
		}
		if _, ok := p.cachedTenant(tenantID); ok {
			goto legacy
		}
		if pol, found := p.loadTenant(tenantID, ownerUserID, p.currentEpoch()); found {
			return pol
		}
	}

legacy:
	if pol, ok := p.cached(ownerUserID); ok {
		return pol
	}
	return p.load(ownerUserID)
}

// Production mode never waits on a database cache miss. A bounded number of
// background loaders refresh policy; until ready the existing scorer decides.
func (p *Provider) forTenantAsync(tenantID, ownerID string) *policy.Policy {
	if pol, ok := p.cachedTenant(tenantID); ok && pol != nil {
		return pol
	}
	legacy, legacyOK := p.cached(ownerID)
	_, tenantOK := p.cachedTenant(tenantID)
	if !tenantOK || !legacyOK {
		p.scheduleLoad(tenantID, ownerID)
	}
	if legacyOK {
		return legacy
	}
	return nil
}

func (p *Provider) scheduleLoad(tenantID, ownerID string) {
	p.mu.Lock()
	if p.loading[tenantID] {
		p.mu.Unlock()
		return
	}
	epoch := p.epoch
	select {
	case p.loadSlots <- struct{}{}:
		p.loading[tenantID] = true
	default:
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()
	go func() {
		defer func() { p.mu.Lock(); delete(p.loading, tenantID); p.mu.Unlock(); <-p.loadSlots }()
		if _, found := p.loadTenant(tenantID, ownerID, epoch); !found {
			p.load(ownerID)
		}
	}()
}

func (p *Provider) cachedTenant(tenantID string) (*policy.Policy, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.tenantCache[tenantID]
	if !ok || p.now().After(e.expiresAt) {
		return nil, false
	}
	return e.policy, true
}

func (p *Provider) currentEpoch() uint64 { p.mu.Lock(); defer p.mu.Unlock(); return p.epoch }

func (p *Provider) loadTenant(tenantID, ownerID string, epoch uint64) (*policy.Policy, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	r, err := p.tenantPolicies.LoadForTenant(ctx, tenantID)
	if err != nil || r == nil || r.OwnerUserID != ownerID {
		// Cache an absence briefly, then use legacy rules in shadow only.
		p.storeTenant(tenantID, nil, negativeCacheTTL, epoch)
		return nil, false
	}
	if err := r.Document.Validate(); err != nil {
		p.storeTenant(tenantID, nil, negativeCacheTTL, epoch)
		return nil, true
	}
	pol := &policy.Policy{TenantID: tenantID, OwnerUserID: ownerID, Version: r.Version, Mode: r.Document.Mode, Rules: r.Document.Rules, RouteClasses: r.Document.RouteClasses, ChallengeTheme: r.Document.ChallengeTheme, BlockMessage: r.Document.BlockMessage}
	for _, raw := range r.Document.Allowlist {
		_, cidr, _ := net.ParseCIDR(raw)
		pol.Allowlist = append(pol.Allowlist, cidr)
	}
	compiled, err := policy.Compile(pol)
	if err != nil {
		p.storeTenant(tenantID, nil, negativeCacheTTL, epoch)
		return nil, true
	}
	p.storeTenant(tenantID, compiled, cacheTTL, epoch)
	return compiled, true
}

func (p *Provider) storeTenant(id string, pol *policy.Policy, ttl time.Duration, epoch uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.epoch != epoch {
		return
	}
	if _, ok := p.tenantCache[id]; !ok && len(p.tenantCache) >= maxCacheEntries {
		for key, e := range p.tenantCache {
			if p.now().After(e.expiresAt) {
				delete(p.tenantCache, key)
			}
		}
		if len(p.tenantCache) >= maxCacheEntries {
			for key := range p.tenantCache {
				delete(p.tenantCache, key)
				break
			}
		}
	}
	p.tenantCache[id] = cacheEntry{policy: pol, expiresAt: p.now().Add(ttl)}
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

	var rows []rules.CustomRule
	var err error
	if bounded, ok := p.rules.(interface {
		ListForPolicy(context.Context, string) ([]rules.CustomRule, error)
	}); ok {
		rows, err = bounded.ListForPolicy(ctx, ownerUserID)
	} else {
		rows, err = p.rules.List(ctx, ownerUserID)
	}
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
	pol, err = policy.Compile(pol)
	if err != nil {
		p.store(ownerUserID, nil, negativeCacheTTL)
		return nil
	}
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
