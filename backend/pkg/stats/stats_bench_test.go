package stats_test

import (
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
	"github.com/ToufiqQureshi/hakaishield/pkg/stats"
)

// BenchmarkStatsRecord measures atomic counter throughput under parallel load.
func BenchmarkStatsRecord(b *testing.B) {
	s := &stats.Stats{Mode: config.ModeEnforce}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			s.Record(signals.DecisionAllow)
		}
	})
}

// BenchmarkStatsRead measures read throughput under parallel load.
func BenchmarkStatsRead(b *testing.B) {
	s := &stats.Stats{Mode: config.ModeEnforce}
	for i := 0; i < 10000; i++ {
		s.Record(signals.DecisionAllow)
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = s.Total()
			_ = s.Passed()
			_ = s.Blocked()
		}
	})
}
