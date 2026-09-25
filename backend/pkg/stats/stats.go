package stats

import (
	"math"
	"sync/atomic"

	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
)

// Stats counts what core.Guard has decided, for the dashboard (ROADMAP
// item 12). In-memory only - resets on restart, same class of
// limitation as challenge.Challenge's in-memory secret (docs/DECISIONS.md).
// Real durable analytics need the planned Postgres store.
type Stats struct {
	// config.Mode is reported alongside the counts so nobody can read them
	// without knowing whether they describe what happened or only what
	// would have happened (docs/ROADMAP.md item 18).
	Mode config.Mode

	total       atomic.Int64
	passed      atomic.Int64
	challenged  atomic.Int64
	blocked     atomic.Int64
	deceived    atomic.Int64
	rateLimited atomic.Int64

	// P1 measurement (docs/CLIENT_READY_IMPLEMENTATION_PLAN.md):
	// egress bytes are the dominant hosting cost of an inline proxy, and
	// challenge solve/failure counts are the plan's human-burden numbers.
	egressBytes     atomic.Int64
	challengeSolved atomic.Int64
	challengeFailed atomic.Int64
}

func (s *Stats) Record(d signals.Decision) {
	s.total.Add(1)
	switch d {
	case signals.DecisionBlock:
		s.blocked.Add(1)
	case signals.DecisionDeceive:
		s.deceived.Add(1)
	case signals.DecisionChallenge:
		s.challenged.Add(1)
	case signals.DecisionRateLimit:
		s.rateLimited.Add(1)
	default:
		s.passed.Add(1)
	}
}

func (s *Stats) Total() int64       { return s.total.Load() }
func (s *Stats) Passed() int64      { return s.passed.Load() }
func (s *Stats) Challenged() int64  { return s.challenged.Load() }
func (s *Stats) Blocked() int64     { return s.blocked.Load() }
func (s *Stats) Deceived() int64    { return s.deceived.Load() }
func (s *Stats) RateLimited() int64 { return s.rateLimited.Load() }

// RecordEgressBytes adds one proxied response's body size to the
// tenant's cost meter. The add saturates at MaxInt64 instead of
// wrapping negative: an attacker driving terabytes through a counter
// must not be able to turn the cost number into garbage (or into a
// negative number the dashboard would happily render).
func (s *Stats) RecordEgressBytes(n int64) {
	if n <= 0 {
		return
	}
	for {
		cur := s.egressBytes.Load()
		if cur > math.MaxInt64-n {
			s.egressBytes.Store(math.MaxInt64)
			return
		}
		if s.egressBytes.CompareAndSwap(cur, cur+n) {
			return
		}
	}
}

func (s *Stats) EgressBytes() int64 { return s.egressBytes.Load() }

// RecordChallengeSolved/RecordChallengeFailed count verify outcomes per
// tenant, the plan's challenge-burden measure next to the global
// observability counters.
func (s *Stats) RecordChallengeSolved()   { s.challengeSolved.Add(1) }
func (s *Stats) RecordChallengeFailed()   { s.challengeFailed.Add(1) }
func (s *Stats) ChallengeSolves() int64   { return s.challengeSolved.Load() }
func (s *Stats) ChallengeFailures() int64 { return s.challengeFailed.Load() }
