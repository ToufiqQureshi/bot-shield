package evidence

import (
	"testing"
	"time"
)

func TestShadowStatsBoundsVersionsAndCopiesSnapshot(t *testing.T) {
	s := NewShadowStats()
	now := time.Now()
	for version := 1; version <= 5; version++ {
		s.Observe(&PolicyOpinion{Version: version, Mode: "shadow", Matched: true, Action: "BLOCK"}, now)
	}
	if got := s.Snapshot(1, now).Evaluated; got != 0 {
		t.Fatalf("old revision retained: %d", got)
	}
	copy := s.Snapshot(5, now)
	copy.Actions["BLOCK"] = 99
	if got := s.Snapshot(5, now).Actions["BLOCK"]; got != 1 {
		t.Fatalf("caller mutated aggregate: %d", got)
	}
}
