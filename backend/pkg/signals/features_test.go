package signals

import (
	"net/http"
	"testing"
)

// The feature mask is a uint32, so the checks list cannot grow past 32
// entries without the 33rd check's evidence vanishing from Fired while
// still counting towards Score. That would be silent, so it is asserted
// rather than left to be noticed.
func TestChecksFitFeatureMask(t *testing.T) {
	if len(checks) > 32 {
		t.Fatalf("checks has %d entries, Evaluation.Fired only holds 32 - widen it to uint64", len(checks))
	}
}

func TestFeatureNamesMatchChecksOrder(t *testing.T) {
	names := FeatureNames()
	if len(names) != len(checks) {
		t.Fatalf("FeatureNames() has %d entries, checks has %d", len(names), len(checks))
	}
	for i, c := range checks {
		if names[i] != c.name {
			t.Errorf("FeatureNames()[%d] = %q, checks[%d] is %q", i, names[i], i, c.name)
		}
	}
}

// FeatureNames hands out a copy, so a caller that keeps and edits the
// slice cannot rename the checks this process scores.
func TestFeatureNamesIsACopy(t *testing.T) {
	names := FeatureNames()
	original := names[0]
	names[0] = "tampered"
	if FeatureNames()[0] != original {
		t.Fatalf("FeatureNames() returned the live check list: mutating it changed %q to %q", original, FeatureNames()[0])
	}
}

// The bitmask and the signal names come out of the same loop, and a
// learned model reads the bitmask while a human reads the names. If the
// two ever disagreed, the evidence shown to a customer would describe a
// different request from the one that was scored.
func TestFiredBitsAgreeWithSignalNames(t *testing.T) {
	newTestRedis(t)

	cases := []struct {
		name  string
		facts RequestFacts
	}{
		{"clean browser", facts("t13d1516h2_8daaf6152771_e5627efa2ab1", "Mozilla/5.0 Chrome/120.0")},
		{"fragmented handshake", facts(JA4Unreadable, "Mozilla/5.0 Chrome/120.0")},
		{"scripting tool", facts("t13d1516h2_8daaf6152771_e5627efa2ab1", "python-requests/2.31.0")},
		{"header anomaly", RequestFacts{JA4: "t13d1516h2_8daaf6152771_e5627efa2ab1", UA: "Mozilla/5.0 Chrome/120.0", Header: http.Header{}}},
		{"ua mismatch on unreadable handshake", facts(JA4Unreadable, "Mozilla/5.0 Safari/605.1")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := Evaluate(tc.facts)

			fromBits := []string{}
			names := FeatureNames()
			for i := range names {
				if e.Fired&(1<<uint(i)) != 0 {
					fromBits = append(fromBits, names[i])
				}
			}

			if len(fromBits) != len(e.Signals) {
				t.Fatalf("Fired names %v, Signals says %v", fromBits, e.Signals)
			}
			for i := range fromBits {
				if fromBits[i] != e.Signals[i] {
					t.Errorf("Fired[%d] = %q, Signals[%d] = %q", i, fromBits[i], i, e.Signals[i])
				}
			}
		})
	}
}

// A request that fires nothing must set no bits: a stray bit would make
// the model treat clean traffic as carrying evidence.
func TestCleanRequestFiresNoBits(t *testing.T) {
	e := Evaluate(facts("t13d1516h2_8daaf6152771_e5627efa2ab1", "Mozilla/5.0 Chrome/120.0"))
	if e.Fired != 0 {
		t.Fatalf("Evaluate(clean browser).Fired = %b, want 0 (signals: %v)", e.Fired, e.Signals)
	}
}
