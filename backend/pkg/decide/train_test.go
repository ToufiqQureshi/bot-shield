package decide

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
)

// samples repeats one labelled pattern n times, which is how the real
// training set will look: a handful of distinct signal combinations seen
// many times each.
func samples(fired uint32, automated bool, n int) []Sample {
	out := make([]Sample, n)
	for i := range out {
		out[i] = Sample{Fired: fired, Automated: automated}
	}
	return out
}

// trainingSet is deliberately not separable. Real traffic is not: some
// humans trip a signal and some bots trip nothing, and a model trained
// only on tidy data would learn a certainty the traffic does not support.
func trainingSet() []Sample {
	var s []Sample
	s = append(s, samples(0, false, 800)...)                   // clean humans
	s = append(s, samples(bit(2), false, 120)...)              // humans with odd headers
	s = append(s, samples(bit(0)|bit(1), true, 400)...)        // obvious bots
	s = append(s, samples(bit(1), true, 150)...)               // bots that only lie about the UA
	s = append(s, samples(0, true, 20)...)                     // bots that trip nothing
	s = append(s, samples(bit(0)|bit(1)|bit(2), true, 200)...) // loud bots
	s = append(s, samples(bit(0), false, 40)...)               // humans on a broken middlebox
	return s
}

func TestTrainLearnsTheDirectionOfEachSignal(t *testing.T) {
	m, err := Train(testFeatures, trainingSet(), DefaultOptions())
	if err != nil {
		t.Fatalf("Train() error: %v", err)
	}

	// ua_lie appears almost only on automated traffic, so it must end up
	// arguing for "automated". headers appears mostly on humans, so it
	// must argue the other way. A model that got these backwards would
	// still train and still report a decent accuracy.
	byName := map[string]float64{}
	for _, c := range m.Explain(bit(0) | bit(1) | bit(2)) {
		byName[c.Feature] = c.Weight
	}
	if byName["ua_lie"] <= 0 {
		t.Errorf("ua_lie weight = %.4f, want positive: it appears almost only on bots", byName["ua_lie"])
	}
	if byName["headers"] >= 0 {
		t.Errorf("headers weight = %.4f, want negative: it appears mostly on humans", byName["headers"])
	}

	// The base rate has to be learned too: most traffic here is human, so
	// a request that fires nothing must not look automated.
	if p := m.Predict(0); p.Decision != signals.DecisionAllow {
		t.Errorf("a request that fired nothing was decided %v (p=%.4f), want allow", p.Decision, p.Probability)
	}
	if m.TrainedOn() != len(trainingSet()) {
		t.Errorf("TrainedOn() = %d, want %d", m.TrainedOn(), len(trainingSet()))
	}
}

// The point of learning weights is to beat the guess. A model that scores
// no better than answering "no idea" to everything is not worth the code,
// so that bar is asserted rather than assumed.
func TestTrainedModelBeatsAnUninformedGuess(t *testing.T) {
	set := trainingSet()
	m, err := Train(testFeatures, set, DefaultOptions())
	if err != nil {
		t.Fatalf("Train() error: %v", err)
	}

	q := m.Score(set)
	// 0.693 is ln(2): the log loss of always answering 0.5.
	if q.LogLoss >= 0.693 {
		t.Fatalf("LogLoss = %.4f, want below 0.693 (an uninformed guess)", q.LogLoss)
	}
	if q.Accuracy <= 0.5 {
		t.Errorf("Accuracy = %.4f, want above 0.5", q.Accuracy)
	}
	if q.Samples != len(set) {
		t.Errorf("Quality.Samples = %d, want %d", q.Samples, len(set))
	}
}

// Same samples in, same weights out. Without this a drop in detection
// quality could never be told apart from training noise.
func TestTrainIsDeterministic(t *testing.T) {
	a, err := Train(testFeatures, trainingSet(), DefaultOptions())
	if err != nil {
		t.Fatalf("Train() error: %v", err)
	}
	b, err := Train(testFeatures, trainingSet(), DefaultOptions())
	if err != nil {
		t.Fatalf("Train() error: %v", err)
	}

	if a.Bias() != b.Bias() {
		t.Errorf("bias differs between runs: %v vs %v", a.Bias(), b.Bias())
	}
	for fired := uint32(0); fired < 8; fired++ {
		if a.Predict(fired) != b.Predict(fired) {
			t.Errorf("Predict(%03b) differs between runs: %+v vs %+v", fired, a.Predict(fired), b.Predict(fired))
		}
	}
}

// One-class data trains without complaint and scores 100% accurate. It is
// also worthless, and shipping it would mean enforcing a model that has
// never seen a human request.
func TestTrainRefusesOneClassData(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  []Sample
	}{
		{"only automated", samples(bit(0), true, 50)},
		{"only human", samples(bit(0), false, 50)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Train(testFeatures, tc.set, DefaultOptions())
			if !errors.Is(err, ErrOneClass) {
				t.Fatalf("Train(%s) error = %v, want ErrOneClass", tc.name, err)
			}
		})
	}
}

// A sample carrying a bit no feature owns means the labelled traffic was
// captured against a different check list. Training on it would silently
// drop that evidence and mislabel everything else.
func TestTrainRefusesSamplesFromADifferentCheckList(t *testing.T) {
	set := append(trainingSet(), Sample{Fired: bit(7), Automated: true})
	_, err := Train(testFeatures, set, DefaultOptions())
	if err == nil {
		t.Fatal("Train() accepted a sample with a bit outside the known features")
	}
}

func TestTrainRefusesUnusableInput(t *testing.T) {
	good := trainingSet()
	cases := []struct {
		name     string
		features []string
		set      []Sample
		opts     Options
		want     string
	}{
		{"no features", nil, good, DefaultOptions(), "no features"},
		{"no samples", testFeatures, nil, DefaultOptions(), "no training samples"},
		{"block bar below challenge bar", testFeatures, good, Options{ChallengeAt: 0.9, BlockAt: 0.5}, "must be above"},
		{"NaN threshold", testFeatures, good, Options{ChallengeAt: math.NaN(), BlockAt: 0.9}, "not inside (0,1)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Train(tc.features, tc.set, tc.opts); err == nil {
				t.Fatalf("Train(%s) returned no error, want one mentioning %q", tc.name, tc.want)
			}
		})
	}
}

// A learning rate high enough to diverge produces infinite weights. They
// would save as an unusable model, so training reports the divergence
// instead of returning one. The L2 term is what actually overflows: the
// plain gradient is bounded by 1 per sample, so it can only ever move a
// weight by the learning rate, while the penalty scales with the weight
// already reached and compounds.
func TestTrainReportsDivergenceInsteadOfReturningIt(t *testing.T) {
	opts := DefaultOptions()
	opts.LearningRate = 1e300
	opts.Iterations = 50

	m, err := Train(testFeatures, trainingSet(), opts)
	if err == nil {
		t.Fatalf("Train() with a runaway learning rate returned a model (bias %v), want a divergence error", m.Bias())
	}
	if !strings.Contains(err.Error(), "diverged") {
		t.Errorf("Train() error = %q, want it to say the training diverged", err)
	}
}

// The zero Options value has to be usable, or every caller has to know
// the defaults to get a model at all.
func TestZeroOptionsTrainsWithDefaults(t *testing.T) {
	m, err := Train(testFeatures, trainingSet(), Options{})
	if err != nil {
		t.Fatalf("Train() with zero Options error: %v", err)
	}
	if got := m.Predict(bit(0) | bit(1)); got.Decision == signals.DecisionAllow {
		t.Errorf("a two-bot-signal request was allowed by a default-trained model (p=%.4f)", got.Probability)
	}
}

// False positives are the number that decides whether a model may ever
// enforce, so Score has to count them, not just report an accuracy that
// hides them.
func TestScoreSeparatesFalsePositivesFromFalseNegatives(t *testing.T) {
	// Always-block: every human is a false positive, no bot is missed.
	blocker := model(20, []float64{0, 0, 0}, 0.5, 0.9)
	set := append(samples(bit(0)|bit(1), false, 7), samples(bit(0)|bit(1), true, 3)...)

	q := blocker.Score(set)
	if q.FalsePositives != 7 {
		t.Errorf("FalsePositives = %d, want 7", q.FalsePositives)
	}
	if q.FalseNegatives != 0 {
		t.Errorf("FalseNegatives = %d, want 0", q.FalseNegatives)
	}

	// Always-allow: the mirror image.
	allower := model(-20, []float64{0, 0, 0}, 0.5, 0.9)
	q = allower.Score(set)
	if q.FalsePositives != 0 {
		t.Errorf("FalsePositives = %d, want 0", q.FalsePositives)
	}
	if q.FalseNegatives != 3 {
		t.Errorf("FalseNegatives = %d, want 3", q.FalseNegatives)
	}
}

// A saturated probability makes the logarithm infinite. Without clamping,
// one such sample would set the whole average to +Inf and hide every
// other sample's contribution.
func TestScoreHandlesSaturatedProbabilities(t *testing.T) {
	certain := model(-800, []float64{0, 0, 0}, 0.5, 0.9)
	q := certain.Score(samples(0, true, 5))
	if math.IsInf(q.LogLoss, 0) || math.IsNaN(q.LogLoss) {
		t.Fatalf("LogLoss = %v, want a finite number", q.LogLoss)
	}
}

func TestScoreOfNothingIsEmptyNotADivideByZero(t *testing.T) {
	m := model(-2, []float64{1, 1, 1}, 0.5, 0.9)
	q := m.Score(nil)
	if q.Samples != 0 || math.IsNaN(q.Accuracy) || math.IsNaN(q.LogLoss) {
		t.Fatalf("Score(nil) = %+v, want a zero Quality", q)
	}
}

func TestVectorMapsNamesToBits(t *testing.T) {
	got, err := Vector(testFeatures, []string{"headers", "handshake"})
	if err != nil {
		t.Fatalf("Vector() error: %v", err)
	}
	if want := bit(0) | bit(2); got != want {
		t.Errorf("Vector() = %03b, want %03b", got, want)
	}

	if got, err := Vector(testFeatures, nil); err != nil || got != 0 {
		t.Errorf("Vector(nothing fired) = %03b, %v; want 0, nil", got, err)
	}
}

// Labels captured against a different check list must be refused. Reading
// them anyway would train on requests that look cleaner than they were.
func TestVectorRefusesAnUnknownCheck(t *testing.T) {
	_, err := Vector(testFeatures, []string{"handshake", "signal_from_another_build"})
	if err == nil {
		t.Fatal("Vector() accepted a check name this build does not run")
	}
	if !strings.Contains(err.Error(), "signal_from_another_build") {
		t.Errorf("Vector() error = %q, want it to name the unknown check", err)
	}
}
