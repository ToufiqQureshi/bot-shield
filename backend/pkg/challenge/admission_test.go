package challenge_test

import (
	"sync"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/challenge"
)

// The unauthenticated verify endpoint must have a concurrency ceiling:
// anyone can be issued a token (no auth on issuance), and each verify
// call decodes attacker-supplied PNGs. Without a gate, one flood turns
// into unbounded concurrent server work exactly where the work is
// expensive (plan P2 track 2, cloud-bill section).
func TestVerifyShedsLoadBeyondCeiling(t *testing.T) {
	n := challenge.MaxConcurrentVerifies()
	release := make(chan struct{})
	started := make(chan struct{}, n)
	var wg sync.WaitGroup

	// Fill every slot with a concurrent job that holds it until the
	// test releases it.
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			challenge.TryAdmitVerify(func() {
				started <- struct{}{}
				<-release
			})
		}()
	}
	for i := 0; i < n; i++ {
		<-started
	}

	// Every slot is busy, so the next call must be shed, not queued.
	if challenge.TryAdmitVerify(func() {}) {
		t.Fatal("a verify call beyond the admission ceiling must be shed")
	}

	close(release)
	wg.Wait()
}

// Admitted work must release its slot when done, or after the first
// burst every real visitor would be shed forever. Sequential rounds
// work here: each call releases its slot before the next begins.
func TestVerifySlotsAreReleasable(t *testing.T) {
	for round := 0; round < 3; round++ {
		for i := 0; i < challenge.MaxConcurrentVerifies(); i++ {
			if !challenge.TryAdmitVerify(func() {}) {
				t.Fatalf("round %d job %d: a completed call must free its slot", round, i)
			}
		}
	}
}
