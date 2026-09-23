package evidence

import (
	"sync"
	"time"
)

// ShadowStats keeps bounded per-revision rollout measurements even when the
// detailed 1000-record evidence ring wraps on a busy site. It is node-local.
type ShadowStats struct {
	mu       sync.Mutex
	versions map[int]*ShadowSnapshot
}

type ShadowSnapshot struct {
	Version   int            `json:"version"`
	Evaluated int            `json:"evaluated"`
	Matched   int            `json:"matched"`
	Disagreed int            `json:"disagreed"`
	Skipped   int            `json:"skipped"`
	Actions   map[string]int `json:"actions"`
	First     time.Time      `json:"first,omitempty"`
	Last      time.Time      `json:"last,omitempty"`
	Ready     bool           `json:"ready"`
}

func NewShadowStats() *ShadowStats { return &ShadowStats{versions: make(map[int]*ShadowSnapshot)} }

// Observe records one immutable opinion. Only a few recent versions are kept.
func (s *ShadowStats) Observe(op *PolicyOpinion, at time.Time) {
	if s == nil || op == nil || op.Version <= 0 || op.Mode != "shadow" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.versions == nil {
		s.versions = make(map[int]*ShadowSnapshot)
	}
	v := s.versions[op.Version]
	if v == nil {
		if len(s.versions) >= 4 {
			oldest := op.Version
			for version := range s.versions {
				if version < oldest {
					oldest = version
				}
			}
			if oldest != op.Version {
				delete(s.versions, oldest)
			} else {
				return
			}
		}
		v = &ShadowSnapshot{Version: op.Version, Actions: map[string]int{}}
		s.versions[op.Version] = v
	}
	if op.SkippedReason != "" {
		v.Skipped++
		return
	}
	v.Evaluated++
	if v.First.IsZero() || at.Before(v.First) {
		v.First = at
	}
	if at.After(v.Last) {
		v.Last = at
	}
	if op.Matched {
		v.Matched++
		v.Actions[op.Action]++
	}
	if op.ProposedDecision != "" && op.ProposedDecision != op.BaselineDecision {
		v.Disagreed++
	}
}

func (s *ShadowStats) Snapshot(version int, now time.Time) ShadowSnapshot {
	empty := ShadowSnapshot{Version: version, Actions: map[string]int{}}
	if s == nil {
		return empty
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.versions[version]
	if v == nil {
		return empty
	}
	copySnapshot := *v
	copySnapshot.Actions = make(map[string]int, len(v.Actions))
	for action, n := range v.Actions {
		copySnapshot.Actions[action] = n
	}
	copySnapshot.Ready = v.Evaluated >= 100 && !v.First.IsZero() && now.Sub(v.First) >= 30*time.Minute && now.Sub(v.Last) <= 5*time.Minute && !v.Last.After(now)
	return copySnapshot
}
