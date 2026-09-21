package stats_test

import (
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
