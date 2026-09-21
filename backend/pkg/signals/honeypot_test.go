package signals

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// resetHoneypot clears all recorded trips between tests.
func resetHoneypot() {
	honeypotMu.Lock()
	defer honeypotMu.Unlock()
	honeypotTrips = make(map[string]time.Time)
	lastSweep = time.Time{}
	honeypotCount.Store(0)
}

func TestHoneypotTripIsScopedToTenantIPAndJA4(t *testing.T) {
	resetHoneypot()
	t.Cleanup(resetHoneypot)

	const (
		tenant = "acme"
		ip     = "192.0.2.10"
		ja4    = "t13d1516h2_8daaf6152771_e5627efa2ab1"
	)

	RecordHoneypotTrip(tenant, ip, ja4)

	if !HoneypotTripped(tenant, ip, ja4) {
		t.Fatal("caller that tripped the trap should be recorded")
	}

	// The same TLS fingerprint is shared by every real user on that
	// browser build. If a trip carried over to them, one bot would take
	// down every Chrome visitor on the site.
	if HoneypotTripped(tenant, "198.51.100.7", ja4) {
		t.Error("a trip must not follow the JA4 to a different IP")
	}

	// A different fingerprint from the same address is a different
	// client — CGNAT and office egress put strangers on one IP.
	if HoneypotTripped(tenant, ip, "t13d1516h2_6b96b8d765a4_c9c9a1bbf8c6") {
		t.Error("a trip must not follow the IP to a different JA4")
	}

	// Tenant isolation: one customer's trap cannot affect another's
	// traffic (CLAUDE.md Section 16).
	if HoneypotTripped("globex", ip, ja4) {
		t.Error("a trip must not cross tenants")
	}
}

func TestHoneypotTripExpires(t *testing.T) {
	resetHoneypot()
	t.Cleanup(resetHoneypot)

	const tenant, ip, ja4 = "acme", "192.0.2.11", "t13d1516h2_8daaf6152771_e5627efa2ab1"

	honeypotMu.Lock()
	honeypotTrips[honeypotKey(tenant, ip, ja4)] = time.Now().Add(-honeypotTTL - time.Minute)
	honeypotMu.Unlock()
	honeypotCount.Store(1)

	if HoneypotTripped(tenant, ip, ja4) {
		t.Fatal("a trip older than the TTL must stop counting: IPs get reassigned")
	}
}

func TestHoneypotRepeatTripsDoNotGrowTheMap(t *testing.T) {
	resetHoneypot()
	t.Cleanup(resetHoneypot)

	const tenant, ip, ja4 = "acme", "192.0.2.12", "t13d1516h2_8daaf6152771_e5627efa2ab1"

	for i := 0; i < 10_000; i++ {
		RecordHoneypotTrip(tenant, ip, ja4)
	}

	honeypotMu.RLock()
	size := len(honeypotTrips)
	honeypotMu.RUnlock()

	if size != 1 {
		t.Fatalf("hammering the trap from one caller must not grow state: got %d entries, want 1", size)
	}
}

func TestHoneypotIsBounded(t *testing.T) {
	resetHoneypot()
	t.Cleanup(resetHoneypot)

	// Every trip here is a distinct caller, so nothing dedupes and
	// nothing has expired — the only thing that can stop growth is the
	// cap itself.
	for i := 0; i < maxHoneypotTrips+5_000; i++ {
		RecordHoneypotTrip("acme", fmt.Sprintf("198.51.%d.%d", i/256%256, i%256), "t13d1516h2_8daaf6152771_e5627efa2ab1")
	}

	honeypotMu.RLock()
	size := len(honeypotTrips)
	honeypotMu.RUnlock()

	if size > maxHoneypotTrips {
		t.Fatalf("honeypot state grew past its cap: %d > %d", size, maxHoneypotTrips)
	}
}

func TestHoneypotIgnoresIncompleteIdentity(t *testing.T) {
	resetHoneypot()
	t.Cleanup(resetHoneypot)

	RecordHoneypotTrip("", "192.0.2.13", "t13d1516h2_8daaf6152771_e5627efa2ab1")
	RecordHoneypotTrip("acme", "", "t13d1516h2_8daaf6152771_e5627efa2ab1")

	honeypotMu.RLock()
	size := len(honeypotTrips)
	honeypotMu.RUnlock()

	if size != 0 {
		t.Fatalf("a trip without a tenant or IP identifies nobody and must not be stored: got %d entries", size)
	}
}

func TestHoneypotConcurrentAccess(t *testing.T) {
	resetHoneypot()
	t.Cleanup(resetHoneypot)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ip := fmt.Sprintf("203.0.113.%d", i)
			RecordHoneypotTrip("acme", ip, "t13d1516h2_8daaf6152771_e5627efa2ab1")
			HoneypotTripped("acme", ip, "t13d1516h2_8daaf6152771_e5627efa2ab1")
		}(i)
	}
	wg.Wait()
}

func TestHoneypotScoresBelowBlockOnItsOwn(t *testing.T) {
	resetHoneypot()
	t.Cleanup(resetHoneypot)

	const tenant, ip, ja4 = "acme", "192.0.2.14", "t13d1516h2_8daaf6152771_e5627efa2ab1"
	RecordHoneypotTrip(tenant, ip, ja4)

	// A screen reader or a browser prefetch can reach a hidden link.
	// Those are real people, so a lone trip must not be a hard block
	// (CLAUDE.md Section 6 and 14) — it must land in challenge range.
	facts := RequestFacts{
		IP:     ip,
		JA4:    ja4,
		UA:     "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Tenant: tenant,
		Path:   "/",
	}

	score := Score(facts)
	if score == 0 {
		t.Fatal("a honeypot trip must contribute to the score")
	}
	if score >= blockThreshold {
		t.Fatalf("a lone honeypot trip must not reach the block threshold: got %d, want < %d", score, blockThreshold)
	}

	var found bool
	for _, s := range Analyze(facts) {
		if s == "honeypot_trap" {
			found = true
		}
	}
	if !found {
		t.Error("the evidence trail must name honeypot_trap so the decision can be explained")
	}
}

// BenchmarkHoneypotTrippedEmpty guards the cost this check adds to
// every single request on a node where nothing has tripped the trap —
// which is the normal state for almost every deployment. It must not
// allocate or take a lock to answer "no".
func BenchmarkHoneypotTrippedEmpty(b *testing.B) {
	resetHoneypot()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HoneypotTripped("acme", "192.0.2.1", "t13d1516h2_8daaf6152771_e5627efa2ab1")
	}
}

func TestHoneypotOnlyFirstTripIsReported(t *testing.T) {
	resetHoneypot()
	t.Cleanup(resetHoneypot)

	const tenant, ip, ja4 = "acme", "192.0.2.15", "t13d1516h2_8daaf6152771_e5627efa2ab1"

	if first := RecordHoneypotTrip(tenant, ip, ja4); !first {
		t.Fatal("the first trip from a caller should be reported so it can be logged once")
	}
	// The trap is a path an attacker can call in a loop and the
	// evidence trail is a fixed-size ring buffer: if every hit were
	// reported, one bot could evict the tenant's real history.
	for i := 0; i < 100; i++ {
		if RecordHoneypotTrip(tenant, ip, ja4) {
			t.Fatalf("repeat trip %d should not be reported again", i)
		}
	}
}
