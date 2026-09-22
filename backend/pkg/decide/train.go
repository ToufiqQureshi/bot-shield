package decide

import (
	"errors"
	"fmt"
	"math"

	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
)

// Training is deliberately plain batch gradient descent on logistic
// regression. With around ten binary features and labelled traffic
// measured in thousands of requests, a full pass is microseconds and the
// result is reproducible: same samples in, same weights out, every time.
// Anything stochastic would make two runs on the same data disagree,
// which would make a regression in detection quality impossible to tell
// apart from training noise.
//
// Training runs offline, never in the request path.

// Sample is one labelled request: the checks it fired, and whether it was
// in fact automated.
//
// The label is the hard part, not the arithmetic. It has to come from
// something that actually knows the answer — a solved challenge, a
// verified good-bot reverse lookup, a customer's own report — and a model
// trained on labels guessed by the current rule scorer would only learn
// to copy the guesses it was meant to improve on.
type Sample struct {
	Fired     uint32
	Automated bool
}

// Options controls training. The zero value is usable: DefaultOptions
// fills anything left unset.
type Options struct {
	// Iterations is how many full passes over the samples to run.
	Iterations int
	// LearningRate scales each step.
	LearningRate float64
	// L2 penalises large weights. It matters more than it looks: without
	// it, a feature that happens to be perfectly separating in the
	// training set drives its weight towards infinity, and the model
	// becomes certain about a pattern it saw a handful of times.
	L2 float64
	// ChallengeAt and BlockAt are the probability bars the trained model
	// is saved with. BlockAt is high by default because a false block is
	// a customer's real visitor turned away (CLAUDE.md Section 14).
	ChallengeAt float64
	BlockAt     float64
}

// DefaultOptions returns the training settings used when a field is left
// at zero.
func DefaultOptions() Options {
	return Options{
		Iterations:   2000,
		LearningRate: 0.5,
		L2:           1e-4,
		ChallengeAt:  0.5,
		BlockAt:      0.9,
	}
}

func (o Options) withDefaults() Options {
	d := DefaultOptions()
	if o.Iterations <= 0 {
		o.Iterations = d.Iterations
	}
	if o.LearningRate <= 0 {
		o.LearningRate = d.LearningRate
	}
	if o.L2 < 0 {
		o.L2 = d.L2
	}
	if o.ChallengeAt == 0 {
		o.ChallengeAt = d.ChallengeAt
	}
	if o.BlockAt == 0 {
		o.BlockAt = d.BlockAt
	}
	return o
}

// ErrOneClass reports training data that contains only automated or only
// human requests. Such a model scores every request the same way and
// learns nothing, but it trains without complaint and its accuracy looks
// perfect, so it is refused at the source rather than shipped.
var ErrOneClass = errors.New("decide: training data has only one label")

// Train fits a model to labelled requests. features must be the check
// names in bitmask order, normally signals.FeatureNames().
func Train(features []string, samples []Sample, opts Options) (*Model, error) {
	opts = opts.withDefaults()

	if len(features) == 0 {
		return nil, errors.New("decide: no features to train on")
	}
	if len(features) > 32 {
		return nil, fmt.Errorf("decide: %d features exceeds the 32-bit feature mask", len(features))
	}
	if len(samples) == 0 {
		return nil, errors.New("decide: no training samples")
	}
	if err := validThresholds(opts.ChallengeAt, opts.BlockAt); err != nil {
		return nil, err
	}

	mask := featureMask(len(features))
	automated := 0
	for _, s := range samples {
		if s.Fired&^mask != 0 {
			return nil, fmt.Errorf("decide: sample fired bit outside the %d known features", len(features))
		}
		if s.Automated {
			automated++
		}
	}
	if automated == 0 || automated == len(samples) {
		return nil, fmt.Errorf("%w: %d automated of %d", ErrOneClass, automated, len(samples))
	}

	weights := make([]float64, len(features))
	bias := 0.0
	n := float64(len(samples))

	for iter := 0; iter < opts.Iterations; iter++ {
		gradW := make([]float64, len(features))
		gradB := 0.0

		for _, s := range samples {
			z := bias
			for i := range weights {
				if s.Fired&(1<<uint(i)) != 0 {
					z += weights[i]
				}
			}
			// err is the signed distance between what the model believes
			// and what actually happened; every weight moves against it.
			err := sigmoid(z) - label(s.Automated)
			gradB += err
			for i := range gradW {
				if s.Fired&(1<<uint(i)) != 0 {
					gradW[i] += err
				}
			}
		}

		bias -= opts.LearningRate * gradB / n
		for i := range weights {
			// The L2 term is applied to the weights only. Penalising the
			// bias too would pull the model away from the real base rate
			// of automated traffic, which is a fact worth keeping.
			weights[i] -= opts.LearningRate * (gradW[i]/n + opts.L2*weights[i])
		}
	}

	for i, w := range weights {
		if !isFinite(w) {
			return nil, fmt.Errorf("decide: training diverged: weight for %q is %v (lower the learning rate)", features[i], w)
		}
	}
	if !isFinite(bias) {
		return nil, fmt.Errorf("decide: training diverged: bias is %v (lower the learning rate)", bias)
	}

	return &Model{
		features:    append([]string(nil), features...),
		weights:     weights,
		bias:        bias,
		challengeAt: opts.ChallengeAt,
		blockAt:     opts.BlockAt,
		trainedOn:   len(samples),
	}, nil
}

// Quality is how a model performed against a set of labelled requests.
// A model is not worth enforcing on the strength of its training score
// alone: score it on traffic it was not trained on.
type Quality struct {
	// LogLoss is the model's average surprise at the true labels. Lower
	// is better; 0.693 is what always answering "no idea" scores, so a
	// model above that is worse than useless.
	LogLoss float64
	// Accuracy is the share of requests whose decision was not
	// DecisionAllow when the request really was automated, and was
	// DecisionAllow when it was not.
	Accuracy float64
	// FalsePositives counts human requests the model did not allow. This
	// is the number that matters most: each one is a real visitor a
	// customer would have lost (CLAUDE.md Section 14).
	FalsePositives int
	// FalseNegatives counts automated requests the model allowed.
	FalseNegatives int
	Samples        int
}

// Score measures a model against labelled requests.
func (m *Model) Score(samples []Sample) Quality {
	q := Quality{Samples: len(samples)}
	if len(samples) == 0 {
		return q
	}

	correct := 0
	total := 0.0
	for _, s := range samples {
		p := m.Predict(s.Fired)
		y := label(s.Automated)
		// Clamp before the logarithm: a saturated probability of exactly
		// 0 or 1 would otherwise make the average infinite and hide every
		// other sample's contribution.
		clamped := math.Min(math.Max(p.Probability, 1e-15), 1-1e-15)
		total += -(y*math.Log(clamped) + (1-y)*math.Log(1-clamped))

		stopped := p.Decision != signals.DecisionAllow
		switch {
		case stopped == s.Automated:
			correct++
		case stopped:
			q.FalsePositives++
		default:
			q.FalseNegatives++
		}
	}

	q.LogLoss = total / float64(len(samples))
	q.Accuracy = float64(correct) / float64(len(samples))
	return q
}

func label(automated bool) float64 {
	if automated {
		return 1
	}
	return 0
}

// featureMask is the set of bits a model with n features can legitimately
// see set.
func featureMask(n int) uint32 {
	if n >= 32 {
		return math.MaxUint32
	}
	return 1<<uint(n) - 1
}

// Vector turns the check names recorded for a request into the bitmask
// Train and Predict use. features is the check list in bitmask order,
// normally signals.FeatureNames().
//
// It takes names rather than a number because that is the form labelled
// traffic actually arrives in: evidence.Evidence records which checks
// fired by name. An unrecognised name is an error, not a skipped entry —
// it means the labels were captured against a different build, and
// quietly dropping the signal would train the model on a request that
// looks cleaner than the one that really happened.
func Vector(features []string, fired []string) (uint32, error) {
	index := make(map[string]int, len(features))
	for i, name := range features {
		index[name] = i
	}

	var mask uint32
	for _, name := range fired {
		i, ok := index[name]
		if !ok {
			return 0, fmt.Errorf("decide: %q is not a check this build runs", name)
		}
		mask |= 1 << uint(i)
	}
	return mask, nil
}
