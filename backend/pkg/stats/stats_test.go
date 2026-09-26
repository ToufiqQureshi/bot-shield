package stats_test

import (
	"math"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
	"github.com/ToufiqQureshi/hakaishield/pkg/stats"
)

func newStats(mode config.Mode) *stats.Stats {
	return &stats.Stats{Mode: mode}
}

func TestStats_Record_AllDecisions(t *testing.T) {
	s := newStats(config.ModeEnforce)

	s.Record(signals.DecisionAllow)
	s.Record(signals.DecisionAllow)
	s.Record(signals.DecisionChallenge)
	s.Record(signals.DecisionBlock)
	s.Record(signals.DecisionDeceive)

	if got := s.Total(); got != 5 {
		t.Errorf("Total: want 5, got %d", got)
	}
	if got := s.Passed(); got != 2 {
		t.Errorf("Passed: want 2, got %d", got)
	}
	if got := s.Challenged(); got != 1 {
		t.Errorf("Challenged: want 1, got %d", got)
	}
	if got := s.Blocked(); got != 1 {
		t.Errorf("Blocked: want 1, got %d", got)
	}
	if got := s.Deceived(); got != 1 {
		t.Errorf("Deceived: want 1, got %d", got)
	}
}

func TestStats_ZeroValues(t *testing.T) {
	s := newStats(config.ModeShadow)
	if s.Total() != 0 || s.Passed() != 0 || s.Challenged() != 0 || s.Blocked() != 0 {
		t.Error("fresh Stats must start at zero")
	}
}

func TestStats_ModeReported(t *testing.T) {
	s := newStats(config.ModeShadow)
	if s.Mode != config.ModeShadow {
		t.Errorf("mode: want shadow, got %v", s.Mode)
	}
}

// P1 measurement (docs/STATUS.md): the plan's
// cost numbers are egress bytes and challenge outcomes, so Stats must
// carry them alongside the decision counts.
func TestStats_RecordsBytesAndChallengeOutcomes(t *testing.T) {
	s := newStats(config.ModeEnforce)

	s.RecordEgressBytes(1024)
	s.RecordEgressBytes(512)
	s.RecordChallengeSolved()
	s.RecordChallengeSolved()
	s.RecordChallengeFailed()

	if got := s.EgressBytes(); got != 1536 {
		t.Errorf("EgressBytes: want 1536, got %d", got)
	}
	if got := s.ChallengeSolves(); got != 2 {
		t.Errorf("ChallengeSolves: want 2, got %d", got)
	}
	if got := s.ChallengeFailures(); got != 1 {
		t.Errorf("ChallengeFailures: want 1, got %d", got)
	}
}

// Egress bytes are the biggest hosting cost line (plan, cloud-bill
// section). A hostile client must not be able to overflow the counter
// into negative territory by wrapping it, so the increment saturates.
func TestStats_EgressBytesSaturatesNotOverflows(t *testing.T) {
	s := newStats(config.ModeEnforce)
	s.RecordEgressBytes(math.MaxInt64)
	s.RecordEgressBytes(1)
	if got := s.EgressBytes(); got != math.MaxInt64 {
		t.Errorf("EgressBytes after overflow: want %d, got %d", int64(math.MaxInt64), got)
	}
	if got := s.EgressBytes(); got < 0 {
		t.Errorf("EgressBytes went negative: %d", got)
	}
}

func TestStats_ConcurrentRecord(t *testing.T) {
	s := newStats(config.ModeEnforce)
	done := make(chan struct{})
	const goroutines = 100
	for i := 0; i < goroutines; i++ {
		go func() {
			s.Record(signals.DecisionAllow)
			done <- struct{}{}
		}()
	}
	for i := 0; i < goroutines; i++ {
		<-done
	}
	if got := s.Total(); got != goroutines {
		t.Errorf("concurrent Total: want %d, got %d", goroutines, got)
	}
}
