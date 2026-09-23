package labels

import (
	"sync"
	"time"
)

// A solved challenge is a human candidate, but the
// solve arrives on a different request from the one that was scored. The
// fired checks have to wait somewhere in between, keyed by the challenge
// nonce.
//
// It is deliberately not the challenge token. The token goes to the
// client, and handing a bot a list of which checks it tripped tells it
// exactly what to fix — the same reason the evidence endpoint is
// token-gated and never gets wildcard CORS.
//
// This is per-process and in memory. Behind several nodes a visitor
// challenged on one node and verified on another simply produces no
// label, which costs a sample and nothing else. Making it shared would
// mean putting the fired mask in Redis, and that is not worth doing
// until there is evidence the loss matters.
const (
	// pendingTTL only has to outlive a human solving the puzzle. The
	// challenge itself expires on its own timer; this is the shorter
	// leash on the memory.
	pendingTTL = 10 * time.Minute

	// maxPending bounds memory under a flood of challenge issues, which
	// is exactly what an attacker hitting a protected site produces.
	maxPending = 20_000

	minPendingSweep = time.Minute
)

// pendingStore holds the sample for a challenge that has been issued but
// not yet solved.
type pendingStore struct {
	mu        sync.Mutex
	byNonce   map[string]pendingEntry
	lastSweep time.Time
}

type pendingEntry struct {
	sample Sample
	at     time.Time
}

func newPendingStore() *pendingStore {
	return &pendingStore{byNonce: make(map[string]pendingEntry)}
}

// park remembers a sample against the nonce of a challenge just issued.
func (p *pendingStore) park(nonce string, s Sample, now time.Time) {
	if nonce == "" {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.byNonce) >= maxPending && now.Sub(p.lastSweep) >= minPendingSweep {
		p.sweepLocked(now)
	}
	if len(p.byNonce) >= maxPending {
		// Full: lose the sample rather than the site.
		return
	}
	p.byNonce[nonce] = pendingEntry{sample: s, at: now}
}

// claim takes the sample back, once. A nonce is consumed on the first
// claim so one solved challenge cannot produce two labels, which is the
// cheapest way to double a sample's weight.
func (p *pendingStore) claim(nonce string, now time.Time) (Sample, bool) {
	if nonce == "" {
		return Sample{}, false
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	entry, ok := p.byNonce[nonce]
	if !ok {
		return Sample{}, false
	}
	delete(p.byNonce, nonce)

	if now.Sub(entry.at) >= pendingTTL {
		return Sample{}, false
	}
	return entry.sample, true
}

func (p *pendingStore) sweepLocked(now time.Time) {
	for k, v := range p.byNonce {
		if now.Sub(v.at) >= pendingTTL {
			delete(p.byNonce, k)
		}
	}
	p.lastSweep = now
}
