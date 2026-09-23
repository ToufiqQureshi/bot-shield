package evidence

import "testing"

func TestTrailCopiesDecisionOnWriteAndRead(t *testing.T) {
	trail := NewTrail()
	input := Evidence{Signals: []string{"velocity_spike"}, Policy: &PolicyOpinion{Version: 2, RuleID: "safe"}, Model: &ModelOpinion{Reasons: []ModelReason{{Feature: "velocity_spike"}}}}
	trail.Record(input)
	input.Signals[0] = "forged"
	input.Policy.RuleID = "forged"
	input.Model.Reasons[0].Feature = "forged"
	first := trail.Recent(1)[0]
	if first.Signals[0] != "velocity_spike" || first.Policy.RuleID != "safe" || first.Model.Reasons[0].Feature != "velocity_spike" {
		t.Fatalf("input mutated stored decision: %+v", first)
	}
	first.Signals[0] = "changed"
	first.Policy.RuleID = "changed"
	first.Model.Reasons[0].Feature = "changed"
	second := trail.Recent(1)[0]
	if second.Signals[0] != "velocity_spike" || second.Policy.RuleID != "safe" || second.Model.Reasons[0].Feature != "velocity_spike" {
		t.Fatalf("reader mutated stored decision: %+v", second)
	}
}
