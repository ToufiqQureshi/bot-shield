package tenant

import (
	"errors"
	"net/http/httputil"
	"sync"

	"github.com/ToufiqQureshi/bot-shield/pkg/config"
	"github.com/ToufiqQureshi/bot-shield/pkg/evidence"
	"github.com/ToufiqQureshi/bot-shield/pkg/stats"
)

// ErrTenantNotFound is returned when a requested tenant ID or host does not exist.
var ErrTenantNotFound = errors.New("tenant not found")

// TenantConfig holds the configuration specific to a single customer.
type TenantConfig struct {
	Target        string      // The origin server to protect (e.g., https://example.com)
	Mode          config.Mode // Enforce or Shadow
	EvidenceToken string      // Bearer token for the per-request evidence endpoint
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

// Store is a thread-safe implementation that maps hostnames and IDs to tenant environments.
type Store struct {
	mu     sync.RWMutex
	byHost map[string]*Tenant
	byID   map[string]*Tenant
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
	defer s.mu.RUnlock()

	t, ok := s.byHost[host]
	if !ok {
		// Fallback to wildcard if present
		t, ok = s.byHost["*"]
		if !ok {
			return nil, ErrTenantNotFound
		}
	}
	return t, nil
}

// GetByID looks up a tenant by their internal ID (for dashboard API use).
func (s *Store) GetByID(id string) (*Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.byID[id]
	if !ok {
		return nil, ErrTenantNotFound
	}
	return t, nil
}
