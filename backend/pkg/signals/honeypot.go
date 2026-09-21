package signals

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// HoneypotPath is a hidden endpoint linked from deceived HTML responses
// (see pkg/deception). The link is invisible to humans and marked
// aria-hidden + rel=nofollow, so a fetch of this path means something
// walked the DOM and followed a link no person and no well-behaved
// crawler would follow.
const HoneypotPath = "/__hakaishield/trap"

const (
	// honeypotTTL is how long a trip keeps counting against a caller.
	// Bounded on purpose: an IP that tripped the trap months ago is
	// very likely a different machine today (DHCP, CGNAT, cloud IP
	// reuse), and holding it forever would eventually block a stranger.
	honeypotTTL = 6 * time.Hour

	// maxHoneypotTrips caps memory. Each entry is roughly 100 bytes, so
	// the ceiling is a few MB. A visitor cannot grow this without also
	// varying their source IP, and once it is full we stop recording
	// rather than grow without limit (CLAUDE.md Section 15).
	maxHoneypotTrips = 50_000

	// minSweepInterval keeps expiry sweeps off the per-request path
	// even if the trap is being hammered.
	minSweepInterval = time.Minute
)

var (
	honeypotMu    sync.RWMutex
	honeypotTrips = make(map[string]time.Time)
	lastSweep     time.Time

	// honeypotCount mirrors len(honeypotTrips) so the lookup on the
	// request path can return without taking a lock or building a key
	// at all — which is the case for every tenant that has never had
	// the trap tripped.
	honeypotCount atomic.Int64
)

// honeypotKey scopes a trip to one tenant and to the pair of
// (source IP, TLS fingerprint).
//
// Both halves matter. JA4 alone is far too coarse to punish: it
// identifies a browser build, not a machine, so thousands of real
// Chrome users share one fingerprint and blocking on it would take
// them all down. IP alone is too coarse the other way: CGNAT and
// corporate egress put many unrelated people behind one address.
// Requiring both means a trip only counts against the caller that
// actually walked into the trap.
//
// The tenant prefix keeps one customer's trap from affecting traffic
// to another customer's site (CLAUDE.md Section 16).
func honeypotKey(tenantID, ip, ja4 string) string {
	var b strings.Builder
	b.Grow(len(tenantID) + len(ip) + len(ja4) + 2)
	b.WriteString(tenantID)
	b.WriteByte('|')
	b.WriteString(ip)
	b.WriteByte('|')
	b.WriteString(ja4)
	return b.String()
}

// RecordHoneypotTrip marks that this caller fetched the hidden trap
// path. It is deliberately not a blocklist write: the trip becomes
// evidence that scoring reads (see the honeypot_trap check in
// score.go), so the decision still comes from combined signals rather
// than from one lone rule (CLAUDE.md Section 6).
//
// It reports whether this was the first trip from this caller within
// the TTL. Callers use that to log and record evidence once per caller
// instead of once per request: the trap is a path an attacker can call
// in a loop, and the evidence trail is a fixed-size ring buffer, so
// recording every hit would let one bot evict the tenant's real
// decision history.
func RecordHoneypotTrip(tenantID, ip, ja4 string) bool {
	if tenantID == "" || ip == "" {
		return false
	}

	key := honeypotKey(tenantID, ip, ja4)
	now := time.Now()

	honeypotMu.Lock()
	defer honeypotMu.Unlock()

	// A repeat trip from a caller we already know refreshes the entry
	// and adds nothing to the map. This is the path a bot hammering
	// the trap takes, so hammering it costs us no memory.
	if _, seen := honeypotTrips[key]; seen {
		honeypotTrips[key] = now
		return false
	}

	if len(honeypotTrips) >= maxHoneypotTrips && now.Sub(lastSweep) >= minSweepInterval {
		sweepHoneypotLocked(now)
	}
	if len(honeypotTrips) >= maxHoneypotTrips {
		// Full and nothing expired: drop this trip rather than grow.
		// Losing a detection is survivable; running the process out of
		// memory in front of a customer's site is not.
		return false
	}

	honeypotTrips[key] = now
	honeypotCount.Store(int64(len(honeypotTrips)))
	return true
}

// HoneypotTripped reports whether this caller has tripped the trap
// within honeypotTTL.
func HoneypotTripped(tenantID, ip, ja4 string) bool {
	// Fast path: nothing has ever tripped the trap on this node, so
	// there is nothing to look up. No lock, no key allocation.
	if honeypotCount.Load() == 0 {
		return false
	}

	key := honeypotKey(tenantID, ip, ja4)

	honeypotMu.RLock()
	at, ok := honeypotTrips[key]
	honeypotMu.RUnlock()

	return ok && time.Since(at) < honeypotTTL
}

// sweepHoneypotLocked drops expired entries. Caller must hold the write lock.
func sweepHoneypotLocked(now time.Time) {
	for k, at := range honeypotTrips {
		if now.Sub(at) >= honeypotTTL {
			delete(honeypotTrips, k)
		}
	}
	lastSweep = now
	honeypotCount.Store(int64(len(honeypotTrips)))
}
