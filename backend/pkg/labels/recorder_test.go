package labels

import (
	"context"
	"testing"
	"time"
)

func TestChallengeSolvedLabelsTheRequestThatWasScored(t *testing.T) {
	w := &fakeWriter{}
	r := NewRecorder(w)

	r.ChallengeIssued("nonce-1", Sample{
		TenantID:       "tenant-1",
		Fired:          0b1010,
		FeatureVersion: "abc123",
	})
	r.ChallengeSolved("nonce-1")
	r.Close()

	got := w.samples()
	if len(got) != 1 {
		t.Fatalf("a solved challenge produced %d samples, want 1", len(got))
	}
	if got[0].Automated {
		t.Error("a solved challenge was labelled automated")
	}
	if got[0].Fired != 0b1010 {
		t.Errorf("Fired = %04b, want the mask from the request that was challenged", got[0].Fired)
	}
	if got[0].Source != SourceChallengeSolved {
		t.Errorf("Source = %q, want %q", got[0].Source, SourceChallengeSolved)
	}
}

// An unsolved challenge is not evidence of anything. A real person on a
// slow phone, with JavaScript off, or who closed the tab produces the
// same silence as a scraper.
func TestAnUnsolvedChallengeLabelsNothing(t *testing.T) {
	w := &fakeWriter{}
	r := NewRecorder(w)

	r.ChallengeIssued("nonce-1", Sample{TenantID: "tenant-1", Fired: 1, FeatureVersion: "abc123"})
	r.Close()

	if got := w.samples(); len(got) != 0 {
		t.Fatalf("an unsolved challenge produced %+v, want nothing", got)
	}
}

// One solve must not produce two labels: replaying it is the cheapest
// way to double a sample's weight in the training set.
func TestASolveCanOnlyBeClaimedOnce(t *testing.T) {
	w := &fakeWriter{}
	r := NewRecorder(w)

	r.ChallengeIssued("nonce-1", Sample{TenantID: "tenant-1", Fired: 1, FeatureVersion: "abc123"})
	r.ChallengeSolved("nonce-1")
	r.ChallengeSolved("nonce-1")
	r.ChallengeSolved("nonce-1")
	r.Close()

	if got := w.samples(); len(got) != 1 {
		t.Fatalf("a replayed solve produced %d samples, want 1", len(got))
	}
}

// A solve for a challenge this process never issued (another node, or a
// made-up nonce) has no sample behind it and must label nothing.
func TestAnUnknownNonceLabelsNothing(t *testing.T) {
	w := &fakeWriter{}
	r := NewRecorder(w)

	r.ChallengeSolved("a-nonce-nobody-issued")
	r.Close()

	if got := w.samples(); len(got) != 0 {
		t.Fatalf("an unknown nonce produced %+v, want nothing", got)
	}
}

// A sample parked long enough to expire is not labelled: the solve is no
// longer good evidence about a request from that long ago.
func TestAParkedSampleExpires(t *testing.T) {
	w := &fakeWriter{}
	r := NewRecorder(w)

	now := time.Now()
	r.now = func() time.Time { return now }
	r.ChallengeIssued("nonce-1", Sample{TenantID: "tenant-1", Fired: 1, FeatureVersion: "abc123"})

	r.now = func() time.Time { return now.Add(pendingTTL + time.Second) }
	r.ChallengeSolved("nonce-1")
	r.Close()

	if got := w.samples(); len(got) != 0 {
		t.Fatalf("an expired sample produced %+v, want nothing", got)
	}
}

func TestHoneypotTripLabelsAutomated(t *testing.T) {
	w := &fakeWriter{}
	r := NewRecorder(w)

	r.HoneypotTripped(Sample{TenantID: "tenant-1", Fired: 0b0110, FeatureVersion: "abc123"})
	r.Close()

	got := w.samples()
	if len(got) != 1 {
		t.Fatalf("a honeypot trip produced %d samples, want 1", len(got))
	}
	if !got[0].Automated {
		t.Error("a honeypot trip was labelled human")
	}
	if got[0].Source != SourceHoneypotTrap {
		t.Errorf("Source = %q, want %q", got[0].Source, SourceHoneypotTrap)
	}
}

// No database configured is the normal state.
func TestANilRecorderIsSafe(t *testing.T) {
	var r *Recorder
	r.ChallengeIssued("n", Sample{})
	r.ChallengeSolved("n")
	r.HoneypotTripped(Sample{})
	r.Close()

	if NewRecorder(nil) != nil {
		t.Error("NewRecorder(nil) returned a recorder with nowhere to write")
	}
}

func TestSampleSurvivesTheRequestContext(t *testing.T) {
	want := Sample{TenantID: "tenant-1", Fired: 7, FeatureVersion: "abc123"}

	got, ok := SampleFrom(WithSample(context.Background(), want))
	if !ok {
		t.Fatal("SampleFrom did not find the sample WithSample stored")
	}
	if got != want {
		t.Errorf("SampleFrom() = %+v, want %+v", got, want)
	}

	if _, ok := SampleFrom(context.Background()); ok {
		t.Error("SampleFrom found a sample in a context that has none")
	}
}
