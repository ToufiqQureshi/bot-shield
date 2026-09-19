package signals

import "testing"

func TestScoreNoSignals(t *testing.T) {
	// Real Chrome, modern TLS: nothing should fire.
	got := Score("", "t13d1516h2_8daaf6152771_e5627efa2ab1", "Mozilla/5.0 Chrome/120.0")
	if got != 0 {
		t.Fatalf("Score() = %d, want 0", got)
	}
}

func TestScoreFragmentedOnly(t *testing.T) {
	// curl also trips scripting_tool, so isolate fragmented_handshake
	// with a UA that isn't a known scripting tool or browser claim.
	got := Score("", JA4Unreadable, "SomeUnknownClient/1.0")
	want := fragmentedWeight
	if got != want {
		t.Fatalf("Score() = %d, want %d", got, want)
	}
}

func TestScoreUAMismatchOnly(t *testing.T) {
	// Claims Firefox but negotiated TLS 1.0 - a real JA4 whose version
	// nibble is "10", not JA4Unreadable, so only UAMismatch fires.
	got := Score("", "t10d1516h2_8daaf6152771_e5627efa2ab1", "Mozilla/5.0 Firefox/120.0")
	want := uaMismatchWeight
	if got != want {
		t.Fatalf("Score() = %d, want %d", got, want)
	}
}

func TestScoreBothSignals(t *testing.T) {
	// Fragmented handshake AND claims to be a browser - both layers fire.
	got := Score("", JA4Unreadable, "Mozilla/5.0 Chrome/120.0")
	want := fragmentedWeight + uaMismatchWeight
	if got != want {
		t.Fatalf("Score() = %d, want %d", got, want)
	}
}

func TestScorePlainHTTPFailsOpen(t *testing.T) {
	// No TLS at all (ja4 == "") - can't fingerprint, must not penalize.
	got := Score("", "", "Mozilla/5.0 Chrome/120.0")
	if got != 0 {
		t.Fatalf("Score() = %d, want 0", got)
	}
}

func TestScoreJA4Blocklist(t *testing.T) {
	// Known malicious JA4 with a non-browser, non-scripting-tool client
	// should add 100 points from ja4_blocklist alone.
	got := Score("", "t12d190800_4464c1bd5eb7_b3394627b738", "SomeUnknownClient/1.0")
	want := 100
	if got != want {
		t.Fatalf("Score() = %d, want %d", got, want)
	}
}

func TestScoreJA4BlocklistWithUAMismatch(t *testing.T) {
	// Known malicious JA4 that also claims to be Chrome fires both blocklist and UA mismatch
	got := Score("", "t12d190800_4464c1bd5eb7_b3394627b738", "Mozilla/5.0 Chrome/120.0")
	want := 100 + uaMismatchWeight
	if got != want {
		t.Fatalf("Score() = %d, want %d", got, want)
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
		DecisionDeceive:   "deceive",
		Decision(99):      "unknown",
	}
	for d, want := range cases {
		if got := d.String(); got != want {
			t.Errorf("Decision(%d).String() = %q, want %q", d, got, want)
		}
	}
}
