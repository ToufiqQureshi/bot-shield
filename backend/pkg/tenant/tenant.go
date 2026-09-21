package tenant

import (
	"context"
	"errors"
	"log"
	"net/http/httputil"
	"sync"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/db"
	"github.com/ToufiqQureshi/hakaishield/pkg/evidence"
	"github.com/ToufiqQureshi/hakaishield/pkg/stats"
)

// ErrTenantNotFound is returned when a requested tenant ID or host does not exist.
var ErrTenantNotFound = errors.New("tenant not found")

// TenantConfig holds the configuration specific to a single customer.
type TenantConfig struct {
	Target        string            // The origin server to protect (e.g., https://example.com)
	Mode          config.Mode       // Enforce or Shadow
	Policy        config.PolicyMode // Balanced or Strict
	EvidenceToken string            // Bearer token for the per-request evidence endpoint
	Deception     bool              // If true, high-confidence bot traffic is deceived instead of 403 blocked (ROADMAP 11a)
}

// Tenant represents a single customer's isolated environment.
// It holds its own proxy, stats, and evidence trail so data cannot leak across customers.
type Tenant struct {
	ID     string
	Config TenantConfig
	Stats  *stats.Stats
	Trail  *evidence.Trail
	Origin *httputil.ReverseProxy
}

// ProxyFactory is a callback to create origin proxies without creating import cycles.
type ProxyFactory func(target string) (*httputil.ReverseProxy, error)

// Store is a thread-safe implementation that maps hostnames and IDs to tenant environments.
type Store struct {
	mu           sync.RWMutex
	byHost       map[string]*Tenant
	byID         map[string]*Tenant
	ProxyFactory ProxyFactory
}

// NewStore creates a store for testing or single-node deployments.
func NewStore() *Store {
	return &Store{
		byHost: make(map[string]*Tenant),
		byID:   make(map[string]*Tenant),
	}
}

// Add provisions a new tenant environment and maps it to the given hosts.
func (s *Store) Add(id string, config TenantConfig, hosts []string, origin *httputil.ReverseProxy) error {
	t := &Tenant{
		ID:     id,
		Config: config,
		Stats:  &stats.Stats{Mode: config.Mode},
		Trail:  evidence.NewTrail(),
		Origin: origin,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.byID[id] = t
	for _, host := range hosts {
		s.byHost[host] = t
	}
	return nil
}

// GetByHost looks up a tenant by their incoming HTTP host header.
func (s *Store) GetByHost(host string) (*Tenant, error) {
	s.mu.RLock()
	t, ok := s.byHost[host]
	if !ok {
		t, ok = s.byHost["*"]
	}
	s.mu.RUnlock()

	if ok {
		return t, nil
	}

	// Not in local cache, try fetching from the database (lazy loading)
	return s.fetchFromDB(host)
}

func (s *Store) fetchFromDB(host string) (*Tenant, error) {
	if s.ProxyFactory == nil {
		return nil, ErrTenantNotFound
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	id, target, modeStr, evidenceToken, err := db.GetTenant(ctx, host)
	if err != nil {
		return nil, ErrTenantNotFound
	}

	return s.addFromDBRow(id, host, target, modeStr, evidenceToken)
}

// addFromDBRow turns one tenants-table row into a live Tenant and
// registers it under both lookup maps, so a dashboard request that
// found the row by ID and a proxy request that finds it later by host
// share the same in-memory Stats/Trail rather than each starting a
// fresh one.
func (s *Store) addFromDBRow(id, host, target, modeStr, evidenceToken string) (*Tenant, error) {
	// A stored mode we can't parse must never silently decide behaviour.
	// Treat an unknown value as enforce (fail closed) and say so, rather
	// than letting Go's zero value quietly pick a mode for a live tenant.
	mode, err := config.ParseMode(modeStr)
	if err != nil {
		log.Printf("hakaishield: tenant %q has unknown mode %q, defaulting to enforce: %v", id, modeStr, err)
		mode = config.ModeEnforce
	}
	proxy, err := s.ProxyFactory(target)
	if err != nil {
		return nil, err
	}

	tenantConfig := TenantConfig{
		Target:        target,
		Mode:          mode,
		EvidenceToken: evidenceToken,
	}

	if err := s.Add(id, tenantConfig, []string{host}, proxy); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.byID[id], nil
}

// GetByID looks up a tenant by their internal ID (for dashboard API
// use, where the caller knows a domain's ID from the tenants table but
// has no incoming request/Host header to look it up by).
func (s *Store) GetByID(id string) (*Tenant, error) {
	s.mu.RLock()
	t, ok := s.byID[id]
	s.mu.RUnlock()
	if ok {
		return t, nil
	}

	if s.ProxyFactory == nil {
		return nil, ErrTenantNotFound
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	host, target, modeStr, evidenceToken, err := db.GetTenantByID(ctx, id)
	if err != nil {
		return nil, ErrTenantNotFound
	}

	return s.addFromDBRow(id, host, target, modeStr, evidenceToken)
}
