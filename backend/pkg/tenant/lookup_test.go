package tenant

import (
	"context"
	"errors"
	"fmt"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// A visitor controls the Host header, so a flood of random hostnames must
// not turn into an unbounded number of simultaneous Postgres queries.
func TestGetByHostBoundsConcurrentDatabaseLookups(t *testing.T) {
	store := NewStore()
	store.ProxyFactory = func(string) (*httputil.ReverseProxy, error) {
		t.Fatal("proxy factory should not run when every lookup misses")
		return nil, nil
	}

	release := make(chan struct{})
	var inFlight, peak, calls atomic.Int64
	store.TenantLoader = func(context.Context, string) (string, string, string, string, string, string, error) {
		calls.Add(1)
		n := inFlight.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		<-release
		inFlight.Add(-1)
		return "", "", "", "", "", "", errors.New("missing")
	}

	const visitors = 200
	var wg sync.WaitGroup
	for i := 0; i < visitors; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := store.GetByHost(fmt.Sprintf("random-%d.example.com", i)); !errors.Is(err, ErrTenantNotFound) {
				t.Errorf("random host resolved: err=%v", err)
			}
		}(i)
	}

	// Let every goroutine either take a lookup slot or be turned away.
	deadline := time.Now().Add(2 * time.Second)
	for inFlight.Load() < maxConcurrentHostLookups && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	if got := peak.Load(); got != maxConcurrentHostLookups {
		t.Fatalf("peak concurrent lookups = %d, want exactly the cap %d", got, maxConcurrentHostLookups)
	}
	if got := calls.Load(); got >= visitors {
		t.Fatalf("every random host reached the database (%d calls); overflow must be turned away", got)
	}
}

// When a newly onboarded domain receives its first burst of traffic, the
// burst must share one database lookup rather than each request querying
// and replacing the tenant (which would split its stats and evidence).
func TestGetByHostSharesOneLookupForTheSameHost(t *testing.T) {
	store := NewStore()
	origin, _ := url.Parse("http://origin.invalid")
	store.ProxyFactory = func(string) (*httputil.ReverseProxy, error) {
		return httputil.NewSingleHostReverseProxy(origin), nil
	}

	release := make(chan struct{})
	var calls atomic.Int64
	store.TenantLoader = func(_ context.Context, host string) (string, string, string, string, string, string, error) {
		calls.Add(1)
		<-release
		return "t1", "http://origin.invalid", "shadow", "", StatusActive, "owner", nil
	}

	const visitors = 50
	results := make([]*Tenant, visitors)
	var wg sync.WaitGroup
	for i := 0; i < visitors; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ten, err := store.GetByHost("new.example.com")
			if err != nil {
				t.Errorf("new host did not resolve: %v", err)
			}
			results[i] = ten
		}(i)
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("same host burst made %d database lookups, want 1", got)
	}
	for i, ten := range results {
		if ten == nil || ten != results[0] {
			t.Fatalf("visitor %d got a different tenant object; stats would be split", i)
		}
	}
}
