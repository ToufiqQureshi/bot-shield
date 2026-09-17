package stats

import (
	"sync/atomic"

	"github.com/ToufiqQureshi/bot-shield/pkg/config"
	"github.com/ToufiqQureshi/bot-shield/pkg/signals"
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

	total      atomic.Int64
	passed     atomic.Int64
	challenged atomic.Int64
	blocked    atomic.Int64
}

func (s *Stats) Record(d signals.Decision) {
	s.total.Add(1)
	switch d {
	case signals.DecisionBlock:
		s.blocked.Add(1)
	case signals.DecisionChallenge:
		s.challenged.Add(1)
	default:
		s.passed.Add(1)
	}
}

func (s *Stats) Total() int64      { return s.total.Load() }
func (s *Stats) Passed() int64     { return s.passed.Load() }
func (s *Stats) Challenged() int64 { return s.challenged.Load() }
func (s *Stats) Blocked() int64    { return s.blocked.Load() }
