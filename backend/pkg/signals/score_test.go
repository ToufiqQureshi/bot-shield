package signals

import "testing"

func TestScoreNoSignals(t *testing.T) {
	// Real Chrome, modern TLS: nothing should fire.
	got := Score("t13d1516h2_8daaf6152771_e5627efa2ab1", "Mozilla/5.0 Chrome/120.0")
	if got != 0 {
		t.Fatalf("Score() = %d, want 0", got)
	}
}

func TestScoreFragmentedOnly(t *testing.T) {
	got := Score(JA4Unreadable, "curl/8.6.0")
	if got != fragmentedWeight {
		t.Fatalf("Score() = %d, want %d", got, fragmentedWeight)
	}
}

func TestScoreUAMismatchOnly(t *testing.T) {
	// Claims Firefox but negotiated TLS 1.0 - a real JA4 whose version
	// nibble is "10", not JA4Unreadable, so only UAMismatch fires.
	got := Score("t10d1516h2_8daaf6152771_e5627efa2ab1", "Mozilla/5.0 Firefox/120.0")
	if got != uaMismatchWeight {
		t.Fatalf("Score() = %d, want %d", got, uaMismatchWeight)
	}
}

func TestScoreBothSignals(t *testing.T) {
	// Fragmented handshake AND claims to be a browser - both layers fire.
	got := Score(JA4Unreadable, "Mozilla/5.0 Chrome/120.0")
	want := fragmentedWeight + uaMismatchWeight
	if got != want {
		t.Fatalf("Score() = %d, want %d", got, want)
	}
}

func TestScorePlainHTTPFailsOpen(t *testing.T) {
	// No TLS at all (ja4 == "") - can't fingerprint, must not penalize.
	got := Score("", "Mozilla/5.0 Chrome/120.0")
	if got != 0 {
		t.Fatalf("Score() = %d, want 0 (fail open, no TLS to judge)", got)
	}
}

func TestDecideThresholds(t *testing.T) {
	cases := []struct {
		score int
		want  Decision
	}{
		{0, DecisionAllow},
		{challengeThreshold - 1, DecisionAllow},
		{challengeThreshold, DecisionChallenge},
		{blockThreshold - 1, DecisionChallenge},
		{blockThreshold, DecisionBlock},
		{blockThreshold + 50, DecisionBlock},
	}
	for _, c := range cases {
		if got := Decide(c.score); got != c.want {
			t.Errorf("Decide(%d) = %v, want %v", c.score, got, c.want)
		}
	}
}

func TestDecisionString(t *testing.T) {
	cases := map[Decision]string{
		DecisionAllow:     "allow",
		DecisionChallenge: "challenge",
		DecisionBlock:     "block",
		Decision(99):      "unknown",
	}
	for d, want := range cases {
		if got := d.String(); got != want {
			t.Errorf("Decision(%d).String() = %q, want %q", d, got, want)
		}
	}
}
