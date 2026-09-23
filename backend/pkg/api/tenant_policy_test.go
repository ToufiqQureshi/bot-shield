package api

import (
	"testing"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/evidence"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"
)

func TestActivationUsesVersionedAggregateAfterEvidenceRingWrap(t *testing.T) {
	tn := &tenant.Tenant{Trail: evidence.NewTrail(), PolicyShadow: evidence.NewShadowStats()}
	now := time.Now()
	old := &evidence.PolicyOpinion{Version: 4, Mode: "shadow", Matched: true, Action: "BLOCK", BaselineDecision: "allow", ProposedDecision: "block"}
	tn.PolicyShadow.Observe(old, now.Add(-31*time.Minute))
	for i := 0; i < 1500; i++ {
		op := &evidence.PolicyOpinion{Version: 4, Mode: "shadow", BaselineDecision: "allow", ProposedDecision: "allow"}
		tn.PolicyShadow.Observe(op, now)
		tn.Trail.Record(evidence.Evidence{Policy: op})
	}
	if len(tn.Trail.Recent(0)) != 1000 {
		t.Fatal("test did not wrap evidence ring")
	}
	if !shadowReady(tn, 4, now) {
		t.Fatal("high-volume shadow aggregate lost its first observation")
	}
	if shadowReady(tn, 5, now) {
		t.Fatal("another version's evidence was accepted")
	}
	s := tn.PolicyShadow.Snapshot(4, now)
	if s.Evaluated != 1501 || s.Matched != 1 || s.Disagreed != 1 || s.Actions["BLOCK"] != 1 {
		t.Fatalf("summary=%+v", s)
	}
	if shadowReady(tn, 4, now.Add(10*time.Minute)) {
		t.Fatal("stale-only traffic was accepted")
	}
	for i := 0; i < 200; i++ {
		tn.PolicyShadow.Observe(&evidence.PolicyOpinion{Version: 5, Mode: "shadow", SkippedReason: "challenge_solved"}, now)
	}
	if shadowReady(tn, 5, now.Add(31*time.Minute)) {
		t.Fatal("skipped decisions were counted")
	}
}
