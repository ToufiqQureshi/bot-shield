package decide

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
)

// The model artifact must record what produced it — the check-list
// version it was trained against, the hash of the labelled data, the
// training options and the evaluation summary — so nobody loads a model
// whose origin nobody can explain (plan §decide 4).
func TestSaveEmbedsProvenance(t *testing.T) {
	m := model(-2, []float64{1, 1, 1}, 0.5, 0.9)
	m2, err := m.WithProvenance(Provenance{
		FeatureVersion: schemaVersion(testFeatures),
		DatasetHash:    "abc123",
		ApprovedBy:     "owner",
		EvalSummary:    "holdout recall 0.8, human FPR 0.0",
	})
	if err != nil {
		t.Fatalf("WithProvenance: %v", err)
	}

	var buf bytes.Buffer
	if err := m2.Save(&buf); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out := buf.String()
	for _, want := range []string{`"feature_version"`, `"dataset_hash": "abc123"`, `"approved_by": "owner"`, `"eval_summary"`, `"trained_with"`} {
		if !strings.Contains(out, want) {
			t.Errorf("saved model missing %s:\n%s", want, out)
		}
	}

	loaded, err := Load(bytes.NewReader(buf.Bytes()), testFeatures)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := loaded.Provenance(); got.DatasetHash != "abc123" || got.ApprovedBy != "owner" {
		t.Errorf("provenance came back wrong: %+v", got)
	}
	if !loaded.Approved() {
		t.Error("a model with an approver must report approved")
	}
}

// A provenance that claims a different check list than the model's own
// features is either a copy-paste or a stale artifact: refuse it rather
// than recording a lie.
func TestWithProvenanceRefusesForeignSchemaVersion(t *testing.T) {
	m := model(-2, []float64{1, 1, 1}, 0.5, 0.9)
	if _, err := m.WithProvenance(Provenance{FeatureVersion: "deadbeefdead"}); err == nil {
		t.Fatal("WithProvenance accepted a feature version that does not match the model's features")
	}
}

// Load refuses a model whose embedded feature version does not match
// this build's check list — the exact failure that made feature
// versions necessary, now closed on the artifact side too.
func TestLoadRefusesStaleFeatureVersion(t *testing.T) {
	m := model(-2, []float64{1, 1, 1}, 0.5, 0.9)
	m2, err := m.WithProvenance(Provenance{FeatureVersion: schemaVersion(testFeatures)})
	if err != nil {
		t.Fatalf("WithProvenance: %v", err)
	}
	var buf bytes.Buffer
	if err := m2.Save(&buf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Tamper the artifact's feature_version to some other build.
	tampered := strings.Replace(buf.String(), `"feature_version": "`+schemaVersion(testFeatures)+`"`, `"feature_version": "000000000000"`, 1)
	if _, err := Load(strings.NewReader(tampered), testFeatures); err == nil {
		t.Fatal("Load accepted a model trained against a different check list")
	}
}

// decide's schema version must reproduce signals.FeatureVersion for the
// live check list, or the artifact gate would disagree with the sample
// gate for no reason but a divergent hash.
func TestSchemaVersionMatchesSignals(t *testing.T) {
	if got, want := schemaVersion(signals.FeatureNames()), signals.FeatureVersion(); got != want {
		t.Errorf("schemaVersion(signals.FeatureNames()) = %q, want signals.FeatureVersion() = %q", got, want)
	}
}

// An unapproved model is allowed to load (shadow-only use is exactly the
// state before approval) but must never report approved.
func TestUnapprovedModelIsVisibleAsSuch(t *testing.T) {
	m := model(-2, []float64{1, 1, 1}, 0.5, 0.9)
	m2, err := m.WithProvenance(Provenance{FeatureVersion: schemaVersion(testFeatures)})
	if err != nil {
		t.Fatalf("WithProvenance: %v", err)
	}
	var buf bytes.Buffer
	if err := m2.Save(&buf); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(bytes.NewReader(buf.Bytes()), testFeatures)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Approved() {
		t.Error("a model saved without an approver must not report approved")
	}
}
