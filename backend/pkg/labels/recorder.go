package labels

import (
	"context"
	"time"
)

// Recorder is what the request path talks to. It owns the queue, the
// per-identity cap and the parked challenge samples, so callers hold one
// thing and a nil one records nothing.
type Recorder struct {
	collector *Collector
	pending   *pendingStore
	now       func() time.Time
}

// NewRecorder starts collecting into w. A nil writer returns nil, which
// every method accepts, so "no database configured" needs no special
// case at the call sites.
func NewRecorder(w Writer) *Recorder {
	c := NewCollector(w)
	if c == nil {
		return nil
	}
	return &Recorder{collector: c, pending: newPendingStore(), now: time.Now}
}

// Close stops collecting and flushes what is queued.
func (r *Recorder) Close() {
	if r == nil {
		return
	}
	r.collector.Close()
}

// ChallengeIssued remembers what a request looked like when it was sent
// to the challenge, so a later solve can label it. It records nothing on
// its own: an unsolved challenge is not evidence of anything, since a
// real person on a slow phone, with JavaScript off, or who just closed
// the tab produces the same silence as a scraper.
func (r *Recorder) ChallengeIssued(nonce string, s Sample) {
	if r == nil {
		return
	}
	s.Automated = false
	s.Source = SourceChallengeSolved
	r.pending.park(nonce, s, r.now())
}

// ChallengeSolved labels the request that led to this challenge as human.
// The solve is independent of anything our own scoring decided, which is
// what makes it a label rather than our own guess repeated back.
//
// It is not proof that a browser ran. The verify handler checks the
// proof-of-work hash, that the canvas string has a PNG data-URL prefix
// and some length, and that the client did not itself report being
// automated (pkg/challenge, validCanvasProof). A script that reproduces
// those three passes without rendering anything, so this source can be
// poisoned by an attacker willing to spend one nonce per sample. The
// per-identity cap bounds the rate, not the ceiling for someone rotating
// addresses. docs/LEARNED_SCORING.md 2.1 has the full caveat; it is one
// reason no model trained on this data may decide anything yet.
func (r *Recorder) ChallengeSolved(nonce string) {
	if r == nil {
		return
	}
	s, ok := r.pending.claim(nonce, r.now())
	if !ok {
		// Expired, already claimed, or issued by a different node.
		return
	}
	r.collector.Record(s)
}

// HoneypotTripped labels a request from a caller that followed the
// invisible trap link as automated.
//
// fired must already have the honeypot_trap bit cleared. Leaving it in
// would make the model learn "honeypot_trap means automated", which is
// the label rather than a finding — the other checks on the same request
// are the part worth learning from (docs/LEARNED_SCORING.md).
func (r *Recorder) HoneypotTripped(s Sample) {
	if r == nil {
		return
	}
	s.Automated = true
	s.Source = SourceHoneypotTrap
	r.collector.Record(s)
}

// contextKey is unexported so nothing outside this package can put a
// value under it.
type contextKey struct{}

// WithSample carries the sample for a request being challenged, so
// pkg/challenge can pair it with the nonce it is about to generate.
// pkg/core sets it; nothing reads it except pkg/challenge.
func WithSample(ctx context.Context, s Sample) context.Context {
	return context.WithValue(ctx, contextKey{}, s)
}

// SampleFrom returns the sample WithSample stored, if any.
func SampleFrom(ctx context.Context) (Sample, bool) {
	s, ok := ctx.Value(contextKey{}).(Sample)
	return s, ok
}
