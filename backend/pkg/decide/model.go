// Package decide turns the signals a request fired into one typed,
// probabilistic opinion.
//
// The rule scorer in pkg/signals adds fixed hand-chosen weights and
// compares the total to a fixed threshold. That works, but the weights are
// guesses: nobody measured that a fragmented handshake is worth exactly
// the same as a user-agent mismatch. This package answers the same
// question from the same evidence, with weights learned from labelled
// traffic (see train.go) and an output that says how sure it is.
//
// Three properties matter here, and each one is a deliberate limit:
//
//   - The output is typed. Predict returns a signals.Decision from the
//     existing enum, never a parsed string, so an invalid decision cannot
//     be produced at all.
//   - The output is explainable. Every fired feature's exact push on the
//     result is recoverable through Explain, because "why was I blocked?"
//     is a question hakaishield has to answer (CLAUDE.md Section 10).
//   - The model is linear. With around ten binary inputs that is the
//     honest amount of capacity; anything larger would fit noise and
//     would stop being explainable, which costs more than it buys.
//
// Inference is a dot product over a bitmask: no allocation, no network
// call, no external service in the request path.
package decide

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/bits"
	"sort"

	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
)

// maxModelBytes bounds a model file read. A model holds at most 32
// weights, so a real file is well under a kilobyte; the cap only exists
// so a corrupt or oversized file fails fast instead of being read into
// memory whole.
const maxModelBytes = 1 << 20

// modelVersion is the on-disk format version. Load refuses anything else
// rather than guessing at fields a future format may have moved. Version
// 2 added the provenance record (dataset hash, training options,
// evaluation summary, approval, feature-version stamp) — a v1 file has
// none of that and is refused, because an artifact nobody can explain
// must not be loaded.
const modelVersion = 2

// Model is a trained linear classifier over the pkg/signals checks.
// It is immutable once loaded, so the request path can share one across
// goroutines without locking.
type Model struct {
	features    []string
	weights     []float64
	bias        float64
	challengeAt float64
	blockAt     float64
	trainedOn   int
	provenance  Provenance
}

// Prediction is one request's typed decision. It carries no strings and
// no pointers so producing it costs nothing in the request path; call
// Explain when the evidence trail needs the reasons too.
type Prediction struct {
	Decision signals.Decision
	// Probability is the model's estimate that the request is automated,
	// in [0,1]. Calibration against representative traffic is not proven.
	Probability float64
	// Confidence is how far the estimate sits from undecided, in [0,1]:
	// 0 means the model is guessing, 1 means it is certain. It is derived
	// from Probability, not measured separately, so it is a restatement of
	// the same number in the form a policy usually wants to read.
	Confidence float64
	// Fired is how many checks fired, so a caller can apply the
	// no-lone-signal rule without re-deriving it from the bitmask.
	Fired int
}

// Contribution is one fired feature's push on the decision, in log-odds.
// Positive means the feature argued the request was automated, negative
// means it argued the opposite.
type Contribution struct {
	Feature string  `json:"feature"`
	Weight  float64 `json:"weight"`
}

// savedModel is the on-disk shape. It is separate from Model so the
// exported API does not become whatever JSON happened to be convenient.
type savedModel struct {
	Version     int        `json:"version"`
	Features    []string   `json:"features"`
	Weights     []float64  `json:"weights"`
	Bias        float64    `json:"bias"`
	ChallengeAt float64    `json:"challenge_at"`
	BlockAt     float64    `json:"block_at"`
	TrainedOn   int        `json:"trained_on"`
	Provenance  Provenance `json:"provenance"`
}

// ErrFeatureMismatch reports a model whose features are not the checks
// this binary actually runs. Scoring it anyway would apply each weight to
// the wrong signal and produce confident nonsense, so it is refused.
var ErrFeatureMismatch = errors.New("decide: model features do not match this build's checks")

// Load reads a trained model and verifies it against the checks this
// binary runs. liveFeatures is normally signals.FeatureNames().
//
// Everything here is refused rather than repaired: a model is operator
// input, and a silently-adjusted model would make every later decision
// wrong in a way no log would show.
func Load(r io.Reader, liveFeatures []string) (*Model, error) {
	var saved savedModel
	dec := json.NewDecoder(io.LimitReader(r, maxModelBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&saved); err != nil {
		return nil, fmt.Errorf("decide: read model: %w", err)
	}
	if saved.Version != modelVersion {
		return nil, fmt.Errorf("decide: model version %d, want %d", saved.Version, modelVersion)
	}
	if len(saved.Features) != len(saved.Weights) {
		return nil, fmt.Errorf("decide: %d features but %d weights", len(saved.Features), len(saved.Weights))
	}
	// Name and order both have to match: Evaluation.Fired is positional,
	// so a model trained before a check was reordered would read every bit
	// after that point as a different signal.
	if len(saved.Features) != len(liveFeatures) {
		return nil, fmt.Errorf("%w: model has %d, build has %d", ErrFeatureMismatch, len(saved.Features), len(liveFeatures))
	}
	for i, name := range saved.Features {
		if name != liveFeatures[i] {
			return nil, fmt.Errorf("%w: position %d is %q, build has %q", ErrFeatureMismatch, i, name, liveFeatures[i])
		}
	}
	if len(saved.Features) > 32 {
		return nil, fmt.Errorf("decide: %d features exceeds the 32-bit feature mask", len(saved.Features))
	}
	// There is deliberately no finiteness check on the weights here.
	// encoding/json refuses a number it cannot represent as a float64, so
	// NaN and infinity cannot survive a model file at all, and a merely
	// enormous weight is harmless: Predict only ever adds finite values,
	// which saturates the probability to 0 or 1 instead of producing a
	// NaN. TestJSONCannotCarryNonFiniteNumbers pins that assumption.
	if err := validThresholds(saved.ChallengeAt, saved.BlockAt); err != nil {
		return nil, err
	}
	// The provenance stamp is part of the artifact, not decoration: a
	// model whose embedded check-list hash disagrees with its own feature
	// list describes a build that never existed.
	if got, want := saved.Provenance.FeatureVersion, schemaVersion(saved.Features); got != want {
		return nil, fmt.Errorf("decide: model provenance names check list %q but its features hash to %q", got, want)
	}

	return &Model{
		features:    append([]string(nil), saved.Features...),
		weights:     append([]float64(nil), saved.Weights...),
		bias:        saved.Bias,
		challengeAt: saved.ChallengeAt,
		blockAt:     saved.BlockAt,
		trainedOn:   saved.TrainedOn,
		provenance:  saved.Provenance,
	}, nil
}

// Save writes the model in the format Load reads.
func (m *Model) Save(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(savedModel{
		Version:     modelVersion,
		Features:    m.features,
		Weights:     m.weights,
		Bias:        m.bias,
		ChallengeAt: m.challengeAt,
		BlockAt:     m.blockAt,
		TrainedOn:   m.trainedOn,
		Provenance:  m.provenance,
	})
}

// Predict scores one request from its fired-check bitmask
// (signals.Evaluation.Fired).
//
// The block bar is deliberately stricter than the probability alone: a
// request is only blocked when at least two checks fired. One signal is
// never enough to refuse a visitor outright, however confident a model is
// about it, because a single mis-weighted feature would then be able to
// turn away real customers on its own (CLAUDE.md Sections 10 and 14). A
// lone strong signal still reaches DecisionChallenge, which a human
// passes and a scraper does not.
func (m *Model) Predict(fired uint32) Prediction {
	z := m.bias
	for i, w := range m.weights {
		if fired&(1<<i) != 0 {
			z += w
		}
	}
	p := sigmoid(z)
	count := bits.OnesCount32(fired & m.mask())

	decision := signals.DecisionAllow
	switch {
	case p >= m.blockAt && count >= 2:
		decision = signals.DecisionBlock
	case p >= m.challengeAt:
		decision = signals.DecisionChallenge
	}

	return Prediction{
		Decision:    decision,
		Probability: p,
		Confidence:  math.Abs(2*p - 1),
		Fired:       count,
	}
}

// Explain lists what each fired check contributed to the decision,
// strongest first. This is the answer to "why was this request stopped?",
// so it reports the model's real arithmetic rather than a summary: the
// values sum with the bias to the log-odds behind Predict's probability.
//
// It is kept separate from Predict because it allocates and most requests
// never need it. Both are pure functions of the same bitmask, so calling
// them separately cannot produce an explanation that disagrees with the
// decision.
func (m *Model) Explain(fired uint32) []Contribution {
	out := make([]Contribution, 0, bits.OnesCount32(fired&m.mask()))
	for i, w := range m.weights {
		if fired&(1<<i) != 0 {
			out = append(out, Contribution{Feature: m.features[i], Weight: w})
		}
	}
	sort.SliceStable(out, func(a, b int) bool {
		return math.Abs(out[a].Weight) > math.Abs(out[b].Weight)
	})
	return out
}

// Bias is the model's score for a request that fired nothing at all: the
// base rate of automated traffic it was trained on, in log-odds.
func (m *Model) Bias() float64 { return m.bias }

// TrainedOn is how many labelled requests produced this model. A model
// built from very little traffic is not wrong, but it is not evidence
// either, so the number travels with the model.
func (m *Model) TrainedOn() int { return m.trainedOn }

// Features returns the check names this model scores, in bitmask order.
func (m *Model) Features() []string { return append([]string(nil), m.features...) }

// mask covers the bits this model has a weight for, so bits set by a
// check the model does not know about are ignored rather than counted as
// evidence.
func (m *Model) mask() uint32 {
	if len(m.weights) >= 32 {
		return math.MaxUint32
	}
	return 1<<len(m.weights) - 1
}

// sigmoid maps a log-odds score to a probability. math.Exp saturates to
// +Inf for large negative z, which divides to 0 rather than producing a
// NaN, so extreme scores stay in range instead of poisoning the decision.
func sigmoid(z float64) float64 {
	return 1 / (1 + math.Exp(-z))
}

func isFinite(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}

// validThresholds keeps the two bars in an order that can actually
// produce all three outcomes. Equal bars would make DecisionChallenge
// unreachable for a multi-signal request, which is a policy change
// disguised as a config value.
func validThresholds(challengeAt, blockAt float64) error {
	if !isFinite(challengeAt) || challengeAt <= 0 || challengeAt >= 1 {
		return fmt.Errorf("decide: challenge threshold %v is not inside (0,1)", challengeAt)
	}
	if !isFinite(blockAt) || blockAt <= 0 || blockAt >= 1 {
		return fmt.Errorf("decide: block threshold %v is not inside (0,1)", blockAt)
	}
	if blockAt <= challengeAt {
		return fmt.Errorf("decide: block threshold %v must be above challenge threshold %v", blockAt, challengeAt)
	}
	return nil
}
