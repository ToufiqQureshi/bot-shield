// Command hakaishield-train fits a decision model to labelled traffic and
// writes it out for the proxy to load in shadow mode.
//
// Input is one JSON object per line, in the shape the evidence trail
// already records:
//
//	{"signals":["ua_mismatch","header_anomaly"],"automated":true}
//	{"signals":[],"automated":false}
//
// The label is the part that has to be earned. "automated" must come from
// something that actually knows — a solved challenge, a verified good-bot
// reverse lookup, a customer's own report. Labelling with the current
// rule score would only teach the model to repeat the guesses it exists
// to improve on, and the resulting model would look excellent against the
// very data that misled it.
//
// This runs offline. Nothing here is in the request path.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ToufiqQureshi/hakaishield/pkg/decide"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
)

// labelled is one line of the input file.
type labelled struct {
	Signals   []string `json:"signals"`
	Automated bool     `json:"automated"`
}

func main() {
	in := flag.String("in", "", "labelled traffic, one JSON object per line (default: stdin)")
	out := flag.String("out", "", "where to write the trained model (default: stdout)")
	holdout := flag.Float64("holdout", 0.2, "share of the data held back to score the model on traffic it was not trained on")
	blockAt := flag.Float64("block-at", decide.DefaultOptions().BlockAt, "probability at or above which the model would block")
	challengeAt := flag.Float64("challenge-at", decide.DefaultOptions().ChallengeAt, "probability at or above which the model would challenge")
	flag.Parse()

	if err := run(*in, *out, *holdout, *challengeAt, *blockAt); err != nil {
		fmt.Fprintf(os.Stderr, "hakaishield-train: %v\n", err)
		os.Exit(1)
	}
}

func run(inPath, outPath string, holdout, challengeAt, blockAt float64) error {
	if holdout < 0 || holdout >= 1 {
		return fmt.Errorf("holdout %v must be in [0,1)", holdout)
	}

	src := io.Reader(os.Stdin)
	if inPath != "" {
		// Operator-supplied flag; reading the file named on the command
		// line is what this command is for.
		f, err := os.Open(inPath) // #nosec G304 -- operator-supplied -in flag
		if err != nil {
			return err
		}
		// Nothing is written to it, so a close error says nothing useful.
		defer func() { _ = f.Close() }()
		src = f
	}

	features := signals.FeatureNames()
	samples, err := readSamples(src, features)
	if err != nil {
		return err
	}

	// The holdout is the tail of the file rather than a random slice, so
	// two runs on the same data split it the same way and their scores
	// can be compared. Whoever produces the file is responsible for it
	// not being ordered by label.
	split := len(samples) - int(float64(len(samples))*holdout)
	train, test := samples[:split], samples[split:]

	opts := decide.DefaultOptions()
	opts.ChallengeAt = challengeAt
	opts.BlockAt = blockAt

	model, err := decide.Train(features, train, opts)
	if err != nil {
		return err
	}

	report(os.Stderr, "training set", model.Score(train))
	if len(test) > 0 {
		// The held-out score is the one worth believing. A model that
		// does well on the data it was fitted to and poorly here has
		// memorised the sample, not learned the traffic.
		report(os.Stderr, "held-out set", model.Score(test))
	} else {
		fmt.Fprintln(os.Stderr, "no held-out data: the training score below is not evidence the model generalises")
	}

	if outPath == "" {
		return model.Save(os.Stdout)
	}

	// Operator-supplied flag; writing where the command line says is the
	// point of the command.
	f, err := os.Create(outPath) // #nosec G304 -- operator-supplied -out flag
	if err != nil {
		return err
	}
	return saveModel(model, f)
}

// saveModel writes the model and reports the close error rather than
// deferring it away. A write that only fails when the file is flushed
// would otherwise leave a truncated model on disk while this command
// exited successfully, and the next thing to read it would be the proxy
// at startup.
func saveModel(model *decide.Model, f io.WriteCloser) error {
	if err := model.Save(f); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// readSamples parses the labelled file. A bad line fails the run rather
// than being skipped: a training set silently missing its malformed
// entries is a different training set from the one the operator thinks
// they produced.
func readSamples(r io.Reader, features []string) ([]decide.Sample, error) {
	var samples []decide.Sample

	scanner := bufio.NewScanner(r)
	// Lines hold a handful of short signal names; the default 64KiB
	// buffer is already far more than one needs.
	for line := 1; scanner.Scan(); line++ {
		text := scanner.Bytes()
		if len(text) == 0 {
			continue
		}

		// Decode rather than Unmarshal, for DisallowUnknownFields: a
		// misspelled field name would otherwise be accepted silently and
		// train the model on a file that is not the one the operator wrote.
		var l labelled
		dec := json.NewDecoder(bytes.NewReader(text))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&l); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}

		fired, err := decide.Vector(features, l.Signals)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		samples = append(samples, decide.Sample{Fired: fired, Automated: l.Automated})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(samples) == 0 {
		return nil, errors.New("no labelled traffic on the input")
	}
	return samples, nil
}

func report(w io.Writer, label string, q decide.Quality) {
	// Writing the report to stderr is best-effort: a failure here must
	// not stop a model that trained correctly from being saved.
	_, _ = fmt.Fprintf(w, "%-12s  samples=%-7d logloss=%.4f  accuracy=%.3f  false-positives=%d  false-negatives=%d\n",
		label, q.Samples, q.LogLoss, q.Accuracy, q.FalsePositives, q.FalseNegatives)
}
