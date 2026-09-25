package decide

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// Provenance is the record of how a model artifact came to exist (plan,
// decide §4): which check list produced its training masks, the hash of
// the labelled dataset, the training options, what evaluation said, and
// who approved it. It travels inside the model file, so an artifact
// always carries its own explanation and Load can refuse one whose
// origin does not match this build.
type Provenance struct {
	// FeatureVersion is the signals check-list hash the training masks
	// were captured against. It must equal the hash of the model's own
	// feature list.
	FeatureVersion string `json:"feature_version"`
	// DatasetHash is a hash of the labelled training data, so two
	// artifacts claiming the same data can be told apart.
	DatasetHash string `json:"dataset_hash"`
	// TrainedWith records the training options actually used.
	TrainedWith string `json:"trained_with"`
	// EvalSummary is the held-out evaluation report verbatim.
	EvalSummary string `json:"eval_summary"`
	// ApprovedBy names the person who accepted the evaluation. Empty
	// means the model is unapproved and may be loaded only for shadow
	// use.
	ApprovedBy string `json:"approved_by,omitempty"`
}

// Approved reports whether a person has accepted this artifact. The
// proxy records opinions either way; enforcement decisions are a human
// sign-off away, not a code path away.
func (p Provenance) Approved() bool { return p.ApprovedBy != "" }

// schemaVersion reproduces signals.FeatureVersion's hash for any check
// list. It exists because a Model stores its feature names, not the
// live list, and the artifact's version must be checked against the
// model's own features — the live list can drift after training.
func schemaVersion(features []string) string {
	sum := sha256.Sum256([]byte(strings.Join(features, "\n")))
	return hex.EncodeToString(sum[:6])
}

// WithProvenance returns a copy of the model stamped with prov. It
// refuses a provenance whose feature version does not match the model's
// own feature list: a stamp claiming another check list is either a
// stale artifact or a copy-paste, and recording the lie would be worse
// than refusing it.
func (m *Model) WithProvenance(prov Provenance) (*Model, error) {
	want := schemaVersion(m.features)
	if prov.FeatureVersion != want {
		return nil, fmt.Errorf("decide: provenance feature version %q does not match the model's check list (%q)", prov.FeatureVersion, want)
	}
	out := *m
	out.provenance = prov
	return &out, nil
}

// Provenance returns the artifact's origin record.
func (m *Model) Provenance() Provenance { return m.provenance }

// Approved reports whether a person accepted this artifact. It is a
// method on Model because the load gate and the operator's deploy
// checklist both want to ask it of the loaded model, not of a struct
// they would have to keep separately.
func (m *Model) Approved() bool { return m.provenance.Approved() }
