package decide

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
)

// testFeatures is a small stand-in for signals.FeatureNames() so these
// tests exercise the model itself rather than whatever the live check
// list happens to contain this week.
var testFeatures = []string{"handshake", "ua_lie", "headers"}

// model builds a Model directly, bypassing Load, so a test can state the
// exact weights the behaviour under test depends on.
func model(bias float64, weights []float64, challengeAt, blockAt float64) *Model {
	return &Model{
		features:    testFeatures,
		weights:     weights,
		bias:        bias,
		challengeAt: challengeAt,
		blockAt:     blockAt,
	}
}

func bit(i uint) uint32 { return 1 << i }

func TestPredictReturnsEachTypedOutcome(t *testing.T) {
	// bias -6 keeps a request that fires nothing well clear of the
	// challenge bar. Each feature is worth +6.5 log-odds, so one puts the
	// probability just over the challenge bar and two clear the block bar.
	m := model(-6, []float64{6.5, 6.5, 6.5}, 0.5, 0.9)

	cases := []struct {
		name  string
		fired uint32
		want  signals.Decision
	}{
		{"nothing fired is allowed", 0, signals.DecisionAllow},
		{"one signal is challenged", bit(0), signals.DecisionChallenge},
		{"two signals clear the block bar", bit(0) | bit(1), signals.DecisionBlock},
		{"all three signals block", bit(0) | bit(1) | bit(2), signals.DecisionBlock},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := m.Predict(tc.fired)
			if got.Decision != tc.want {
				t.Fatalf("Predict(%03b).Decision = %v (p=%.4f), want %v", tc.fired, got.Decision, got.Probability, tc.want)
			}
		})
	}
}

// CLAUDE.md Section 10: no single signal may be the only reason for a
// hard block. The model has to obey that even when it is certain, because
// one mis-weighted feature would otherwise refuse real visitors alone.
func TestPredictNeverBlocksOnALoneSignal(t *testing.T) {
	// +40 log-odds puts the probability at effectively 1.
	m := model(-6, []float64{40, 4, 4}, 0.5, 0.9)

	got := m.Predict(bit(0))
	if got.Probability < 0.999 {
		t.Fatalf("test setup is wrong: want a near-certain probability, got %.6f", got.Probability)
	}
	if got.Decision != signals.DecisionChallenge {
		t.Fatalf("Predict(lone certain signal).Decision = %v, want %v: one signal must never block alone", got.Decision, signals.DecisionChallenge)
	}
	if got.Fired != 1 {
		t.Errorf("Predict(lone signal).Fired = %d, want 1", got.Fired)
	}
}

// Confidence must track the probability, not be filled in with a
// placeholder: a policy that reads it needs it to mean something.
func TestConfidenceMeasuresDistanceFromUndecided(t *testing.T) {
	m := model(0, []float64{0, 0, 0}, 0.5, 0.9)
	if got := m.Predict(0).Confidence; got > 1e-9 {
		t.Errorf("a coin-flip prediction has Confidence %v, want ~0", got)
	}

	certain := model(-30, []float64{0, 0, 0}, 0.5, 0.9)
	if got := certain.Predict(0).Confidence; got < 0.999 {
		t.Errorf("a near-certain prediction has Confidence %v, want ~1", got)
	}
}

// Explain has to report the model's real arithmetic. If the contributions
// and the bias do not add up to the log-odds behind the probability, the
// explanation shown to a customer is a story about a different decision.
func TestExplainSumsToThePredictedLogOdds(t *testing.T) {
	m := model(-2.5, []float64{1.25, -0.75, 3.5}, 0.5, 0.9)
	fired := bit(0) | bit(2)

	z := m.Bias()
	for _, c := range m.Explain(fired) {
		z += c.Weight
	}

	wantP := m.Predict(fired).Probability
	if got := sigmoid(z); math.Abs(got-wantP) > 1e-12 {
		t.Fatalf("Explain sums to p=%.15f, Predict says p=%.15f", got, wantP)
	}
}

func TestExplainListsOnlyFiredFeaturesStrongestFirst(t *testing.T) {
	m := model(-2, []float64{0.5, -3.0, 1.5}, 0.5, 0.9)

	got := m.Explain(bit(0) | bit(1) | bit(2))
	want := []Contribution{
		{Feature: "ua_lie", Weight: -3.0},
		{Feature: "headers", Weight: 1.5},
		{Feature: "handshake", Weight: 0.5},
	}
	if len(got) != len(want) {
		t.Fatalf("Explain() returned %d contributions, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Explain()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	if only := m.Explain(bit(1)); len(only) != 1 || only[0].Feature != "ua_lie" {
		t.Errorf("Explain(one bit) = %+v, want just ua_lie", only)
	}
	if none := m.Explain(0); len(none) != 0 {
		t.Errorf("Explain(nothing fired) = %+v, want no contributions", none)
	}
}

// A bit the model has no weight for is evidence from a check this model
// was never trained on. Counting it towards the two-signal block rule
// would let an untrained signal help refuse a visitor.
func TestPredictIgnoresBitsOutsideTheModel(t *testing.T) {
	m := model(-6, []float64{40, 4, 4}, 0.5, 0.9)

	withStrayBit := m.Predict(bit(0) | bit(9))
	if withStrayBit.Fired != 1 {
		t.Errorf("Predict(lone signal + unknown bit).Fired = %d, want 1", withStrayBit.Fired)
	}
	if withStrayBit.Decision != signals.DecisionChallenge {
		t.Fatalf("an unknown bit pushed the decision to %v: it must not count as a second signal", withStrayBit.Decision)
	}
	if plain := m.Predict(bit(0)); withStrayBit.Probability != plain.Probability {
		t.Errorf("an unknown bit changed the probability: %v vs %v", withStrayBit.Probability, plain.Probability)
	}
}

// Extreme weights must saturate to a usable probability rather than
// producing a NaN that silently compares false against every threshold
// and lands every request in DecisionAllow.
func TestExtremeScoresStayInRange(t *testing.T) {
	for _, bias := range []float64{-1e308, 1e308, -745, 745} {
		m := model(bias, []float64{0, 0, 0}, 0.5, 0.9)
		p := m.Predict(0)
		if math.IsNaN(p.Probability) || p.Probability < 0 || p.Probability > 1 {
			t.Errorf("bias %v produced probability %v, want a value in [0,1]", bias, p.Probability)
		}
		if math.IsNaN(p.Confidence) {
			t.Errorf("bias %v produced NaN confidence", bias)
		}
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	original := model(-2.5, []float64{1.25, -0.75, 3.5}, 0.4, 0.95)
	original.trainedOn = 4321
	// A saved artifact must carry its provenance stamp: Load refuses one
	// without it, since v2 made the origin record part of the format.
	original, err := original.WithProvenance(Provenance{FeatureVersion: schemaVersion(testFeatures)})
	if err != nil {
		t.Fatalf("WithProvenance() error: %v", err)
	}

	var buf bytes.Buffer
	if err := original.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load(bytes.NewReader(buf.Bytes()), testFeatures)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.TrainedOn() != 4321 {
		t.Errorf("TrainedOn() = %d, want 4321", loaded.TrainedOn())
	}

	// The decision is what has to survive the round trip, not just the
	// fields: compare behaviour across every possible fired combination.
	for fired := uint32(0); fired < 8; fired++ {
		a, b := original.Predict(fired), loaded.Predict(fired)
		if a != b {
			t.Errorf("Predict(%03b): before save %+v, after load %+v", fired, a, b)
		}
	}
}

// A model file is operator input. Every one of these would otherwise
// produce a model that scores confidently and wrongly, so Load refuses
// them instead of repairing them.
func TestLoadRejectsUnusableModels(t *testing.T) {
	cases := []struct {
		name string
		json string
		want string
	}{
		{
			"a renamed feature",
			`{"version":2,"provenance":{"feature_version":"c85d7935ea0b"},"features":["handshake","ua_lie","fingerprint"],"weights":[1,1,1],"bias":-2,"challenge_at":0.5,"block_at":0.9,"trained_on":10}`,
			"position 2",
		},
		{
			"features in a different order",
			`{"version":2,"provenance":{"feature_version":"c85d7935ea0b"},"features":["ua_lie","handshake","headers"],"weights":[1,1,1],"bias":-2,"challenge_at":0.5,"block_at":0.9,"trained_on":10}`,
			"position 0",
		},
		{
			"a check this build does not have",
			`{"version":2,"provenance":{"feature_version":"c85d7935ea0b"},"features":["handshake","ua_lie","headers","extra"],"weights":[1,1,1,1],"bias":-2,"challenge_at":0.5,"block_at":0.9,"trained_on":10}`,
			"build has 3",
		},
		{
			"more features than weights",
			`{"version":2,"provenance":{"feature_version":"c85d7935ea0b"},"features":["handshake","ua_lie","headers"],"weights":[1,1],"bias":-2,"challenge_at":0.5,"block_at":0.9,"trained_on":10}`,
			"3 features but 2 weights",
		},
		{
			"a future format version",
			`{"version":3,"provenance":{"feature_version":"c85d7935ea0b"},"features":["handshake","ua_lie","headers"],"weights":[1,1,1],"bias":-2,"challenge_at":0.5,"block_at":0.9,"trained_on":10}`,
			"model version 3",
		},
		{
			"a v1 artifact without provenance",
			`{"version":1,"features":["handshake","ua_lie","headers"],"weights":[1,1,1],"bias":-2,"challenge_at":0.5,"block_at":0.9,"trained_on":10}`,
			"model version 1, want 2",
		},
		{
			"a block bar below the challenge bar",
			`{"version":2,"provenance":{"feature_version":"c85d7935ea0b"},"features":["handshake","ua_lie","headers"],"weights":[1,1,1],"bias":-2,"challenge_at":0.9,"block_at":0.5,"trained_on":10}`,
			"must be above",
		},
		{
			"a threshold outside (0,1)",
			`{"version":2,"provenance":{"feature_version":"c85d7935ea0b"},"features":["handshake","ua_lie","headers"],"weights":[1,1,1],"bias":-2,"challenge_at":0,"block_at":0.9,"trained_on":10}`,
			"not inside (0,1)",
		},
		{
			"a threshold of exactly 1",
			`{"version":2,"provenance":{"feature_version":"c85d7935ea0b"},"features":["handshake","ua_lie","headers"],"weights":[1,1,1],"bias":-2,"challenge_at":0.5,"block_at":1,"trained_on":10}`,
			"not inside (0,1)",
		},
		{
			"an unknown field",
			`{"version":2,"provenance":{"feature_version":"c85d7935ea0b"},"features":["handshake","ua_lie","headers"],"weights":[1,1,1],"bias":-2,"challenge_at":0.5,"block_at":0.9,"trained_on":10,"enforce":true}`,
			"unknown field",
		},
		{"not JSON at all", `not a model`, "read model"},
		{"an empty file", ``, "read model"},
		{"JSON that is not an object", `[1,2,3]`, "read model"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(strings.NewReader(tc.json), testFeatures)
			if err == nil {
				t.Fatalf("Load(%s) returned no error, want one mentioning %q", tc.name, tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Load(%s) error = %q, want it to mention %q", tc.name, err, tc.want)
			}
		})
	}
}

// Load has no finiteness check because it cannot need one: a model file
// is JSON, and encoding/json refuses any number it cannot hold in a
// float64. That is an assumption the request path relies on, so it is
// pinned here rather than trusted to stay true.
func TestJSONCannotCarryNonFiniteNumbers(t *testing.T) {
	for _, literal := range []string{"1e999", "-1e999"} {
		raw := `{"version":2,"provenance":{"feature_version":"c85d7935ea0b"},"features":["handshake","ua_lie","headers"],"weights":[` + literal + `,1,1],"bias":-2,"challenge_at":0.5,"block_at":0.9,"trained_on":10}`
		if _, err := Load(strings.NewReader(raw), testFeatures); err == nil {
			t.Fatalf("Load accepted an out-of-range weight %s: the finiteness guard removed from Load is needed after all", literal)
		}
	}
}

// A weight large enough to saturate the probability is still a usable
// model, not a crash or a NaN that would compare false against every
// threshold and quietly allow the request.
func TestHugeButFiniteWeightsStaySane(t *testing.T) {
	raw := `{"version":2,"provenance":{"feature_version":"c85d7935ea0b"},"features":["handshake","ua_lie","headers"],"weights":[1e308,1e308,-1e308],"bias":-1e308,"challenge_at":0.5,"block_at":0.9,"trained_on":10}`
	m, err := Load(strings.NewReader(raw), testFeatures)
	if err != nil {
		t.Fatalf("Load() rejected finite weights: %v", err)
	}
	for fired := uint32(0); fired < 8; fired++ {
		p := m.Predict(fired)
		if math.IsNaN(p.Probability) || p.Probability < 0 || p.Probability > 1 {
			t.Errorf("Predict(%03b) gave probability %v, want a value in [0,1]", fired, p.Probability)
		}
	}
}

func TestFeatureMismatchIsIdentifiable(t *testing.T) {
	raw := `{"version":2,"provenance":{"feature_version":"c85d7935ea0b"},"features":["handshake","ua_lie"],"weights":[1,1],"bias":-2,"challenge_at":0.5,"block_at":0.9,"trained_on":10}`
	_, err := Load(strings.NewReader(raw), testFeatures)
	if !errors.Is(err, ErrFeatureMismatch) {
		t.Fatalf("Load() error = %v, want it to wrap ErrFeatureMismatch so a caller can tell it from a corrupt file", err)
	}
}

// Load must not hand back a model that aliases the decoded slices, or a
// later caller could change the weights a running Guard is scoring with.
func TestLoadedModelDoesNotAliasItsInput(t *testing.T) {
	raw := `{"version":2,"provenance":{"feature_version":"c85d7935ea0b"},"features":["handshake","ua_lie","headers"],"weights":[1,1,1],"bias":-2,"challenge_at":0.5,"block_at":0.9,"trained_on":10}`
	m, err := Load(strings.NewReader(raw), testFeatures)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	names := m.Features()
	names[0] = "tampered"
	if m.Features()[0] != "handshake" {
		t.Fatal("Features() exposes the model's own slice: a caller can rename what it scores")
	}
}
