package labels

import (
	"sync"
	"time"
)

// A bot that deliberately solves challenges is injecting "human" labels
// for its own fingerprint. Uncapped, it can do that as often as it likes
// and eventually own the training set. The cap makes that expensive:
// contributing more labels requires more distinct (IP, JA4) identities,
// which is the same cost as evading the rest of the detection.
//
// The numbers are deliberately generous. A real visitor solving a
// challenge a handful of times an hour is normal; a client producing
// hundreds is not a customer.
const (
	maxPerIdentity = 5
	capWindow      = time.Hour

	// maxTrackedIdentities bounds memory. Each entry is small, and once
	// full we stop admitting new identities rather than grow without
	// limit (CLAUDE.md Section 15) — the same trade the honeypot store
	// makes, for the same reason.
	maxTrackedIdentities = 50_000

	// minCapSweep keeps expiry sweeps off the hot path even when labels
	// are arriving constantly.
	minCapSweep = time.Minute
)

// identityCap limits how many samples one client can contribute inside
// capWindow.
type identityCap struct {
	mu        sync.Mutex
	seen      map[string]capEntry
	lastSweep time.Time
}

type capEntry struct {
	count int
	first time.Time
}

func newIdentityCap() *identityCap {
	return &identityCap{seen: make(map[string]capEntry)}
}

// allow reports whether this identity may contribute another sample.
//
// An empty identity is allowed through uncapped: it means the caller had
// no client identity to attribute the sample to, which today only
// happens in tests. Callers on the request path always supply one.
func (c *identityCap) allow(identity string, now time.Time) bool {
	if identity == "" {
		return true
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, seen := c.seen[identity]
	if seen && now.Sub(entry.first) < capWindow {
		if entry.count >= maxPerIdentity {
			return false
		}
		entry.count++
		c.seen[identity] = entry
		return true
	}

	// New identity, or one whose window has expired and starts again.
	if !seen {
		if len(c.seen) >= maxTrackedIdentities && now.Sub(c.lastSweep) >= minCapSweep {
			c.sweepLocked(now)
		}
		if len(c.seen) >= maxTrackedIdentities {
			// Full and nothing expired. Refusing the sample is the safe
			// direction: an uncapped sample is exactly what the cap
			// exists to prevent.
			return false
		}
	}

	c.seen[identity] = capEntry{count: 1, first: now}
	return true
}

// sweepLocked drops entries whose window has passed. The caller holds
// the lock.
func (c *identityCap) sweepLocked(now time.Time) {
	for k, v := range c.seen {
		if now.Sub(v.first) >= capWindow {
			delete(c.seen, k)
		}
	}
	c.lastSweep = now
}
