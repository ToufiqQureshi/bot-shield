package evidence_test

import (
	"testing"

	"github.com/ToufiqQureshi/bot-shield/pkg/evidence"
)

// BenchmarkTrailRecord measures how fast we can write evidence under load.
func BenchmarkTrailRecord(b *testing.B) {
	tr := evidence.NewTrail()
	e := evidence.Evidence{JA4: "t13d1516h2_8daaf6152771_e5627efa2ab1", Decision: "allow", Score: 0}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			tr.Record(e)
		}
	})
}

// BenchmarkTrailRecent measures how fast we can read the evidence trail.
func BenchmarkTrailRecent(b *testing.B) {
	tr := evidence.NewTrail()
	for i := 0; i < 1000; i++ {
		tr.Record(evidence.Evidence{JA4: "fp", Decision: "allow"})
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = tr.Recent(10)
		}
	})
}

// BenchmarkTrailRecordAndRead simulates simultaneous writes and reads (real traffic scenario).
func BenchmarkTrailRecordAndRead(b *testing.B) {
	tr := evidence.NewTrail()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%3 == 0 {
				_ = tr.Recent(50)
			} else {
				tr.Record(evidence.Evidence{JA4: "fp", Decision: "block"})
			}
			i++
		}
	})
}
