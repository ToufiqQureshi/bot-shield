package labels

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeWriter stands in for the database. It records what was actually
// written, so a test can assert the sample reached storage rather than
// that a method was called.
type fakeWriter struct {
	mu     sync.Mutex
	got    []Sample
	err    error
	writes int
}

func (w *fakeWriter) WriteSamples(_ context.Context, samples []Sample) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writes++
	if w.err != nil {
		return w.err
	}
	w.got = append(w.got, samples...)
	return nil
}

func (w *fakeWriter) samples() []Sample {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]Sample(nil), w.got...)
}

func validSample() Sample {
	return Sample{
		TenantID:       "tenant-1",
		Fired:          0b101,
		FeatureVersion: "abc123",
		Source:         SourceHoneypotTrap,
		Automated:      true,
	}
}

// Close flushes, so a test can assert on what was written without
// sleeping and hoping.
func TestRecordedSamplesReachTheWriter(t *testing.T) {
	w := &fakeWriter{}
	c := NewCollector(w)

	c.Record(validSample())
	c.Close()

	got := w.samples()
	if len(got) != 1 {
		t.Fatalf("writer received %d samples, want 1", len(got))
	}
	if got[0].TenantID != "tenant-1" || got[0].Fired != 0b101 || !got[0].Automated {
		t.Errorf("writer received %+v, want the sample as recorded", got[0])
	}
}

// A sample that cannot be scoped to a tenant or interpreted later is
// worse than no sample, so it is refused rather than stored.
func TestUnattributableSamplesAreRefused(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Sample)
	}{
		{"no tenant", func(s *Sample) { s.TenantID = "" }},
		{"no feature version", func(s *Sample) { s.FeatureVersion = "" }},
		{"no source", func(s *Sample) { s.Source = "" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := &fakeWriter{}
			c := NewCollector(w)

			s := validSample()
			tc.mutate(&s)
			c.Record(s)
			c.Close()

			if got := w.samples(); len(got) != 0 {
				t.Fatalf("writer received %+v, want nothing", got)
			}
		})
	}
}

// A bot that deliberately solves challenges is injecting human labels
// for its own fingerprint. Without a cap it can do that until it owns
// the training set.
func TestOneIdentityCannotFloodTheTrainingSet(t *testing.T) {
	w := &fakeWriter{}
	c := NewCollector(w)

	for i := 0; i < maxPerIdentity*10; i++ {
		s := validSample()
		s.Identity = "tenant-1|203.0.113.5|t13d1516h2"
		c.Record(s)
	}
	c.Close()

	if got := len(w.samples()); got != maxPerIdentity {
		t.Fatalf("one identity contributed %d samples, want the cap of %d", got, maxPerIdentity)
	}
}

// The cap is per identity, not global: a busy site must still collect.
func TestDifferentIdentitiesAreNotCappedTogether(t *testing.T) {
	w := &fakeWriter{}
	c := NewCollector(w)

	for i := 0; i < 20; i++ {
		s := validSample()
		s.Identity = "tenant-1|203.0.113." + string(rune('a'+i)) + "|ja4"
		c.Record(s)
	}
	c.Close()

	if got := len(w.samples()); got != 20 {
		t.Fatalf("20 distinct identities produced %d samples, want 20", got)
	}
}

// The window expires, or a real visitor would be silently capped forever
// after one busy hour.
func TestTheCapWindowExpires(t *testing.T) {
	cap := newIdentityCap()
	start := time.Now()

	for i := 0; i < maxPerIdentity; i++ {
		if !cap.allow("client", start) {
			t.Fatalf("sample %d was capped before the limit", i)
		}
	}
	if cap.allow("client", start) {
		t.Fatal("the cap did not apply once the limit was reached")
	}
	if !cap.allow("client", start.Add(capWindow+time.Second)) {
		t.Fatal("the cap never expires: a real visitor stays capped forever")
	}
}

// A failing database must not take the process with it, and must be
// visible rather than silent.
func TestAWriteFailureDoesNotStopCollection(t *testing.T) {
	w := &fakeWriter{err: errors.New("database is down")}
	c := NewCollector(w)

	c.Record(validSample())
	c.Close()

	if w.writes == 0 {
		t.Fatal("the writer was never called")
	}
	if got := w.samples(); len(got) != 0 {
		t.Fatalf("a failed write stored %+v", got)
	}
}

// No database configured is the normal state, and must not need a nil
// check at every call site.
func TestANilCollectorIsSafe(t *testing.T) {
	var c *Collector
	c.Record(validSample())
	c.Close()

	if NewCollector(nil) != nil {
		t.Error("NewCollector(nil) returned a collector with nowhere to write")
	}
}

// Recording happens on the request path, so it must never block, even
// with the queue full and nothing draining it.
func TestRecordNeverBlocks(t *testing.T) {
	c := &Collector{
		writer: &fakeWriter{},
		queue:  make(chan Sample, 1),
		cap:    newIdentityCap(),
		done:   make(chan struct{}),
	}
	// No worker started: nothing will ever drain the queue.

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < queueSize*2; i++ {
			c.Record(validSample())
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Record blocked on a full queue: this runs in the request path")
	}
}

// http.Server.Shutdown returns once its timeout expires, but the
// handlers it gave up on keep running. Close then races them, and a
// send on a closed channel panics even inside a select with a default
// case - so the queue alone cannot make Record safe here.
func TestRecordAfterCloseDoesNotPanic(t *testing.T) {
	c := NewCollector(&fakeWriter{})
	c.Close()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Record panicked after Close: %v", r)
		}
	}()
	c.Record(validSample())
}

// Close is called from a deferred cleanup while a request that outlived
// Shutdown is still recording. Neither side may panic.
func TestCloseWhileRecordingConcurrently(t *testing.T) {
	c := NewCollector(&fakeWriter{})

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Record panicked during Close: %v", r)
				}
			}()
			for j := 0; j < 200; j++ {
				c.Record(validSample())
			}
		}()
	}
	c.Close()
	wg.Wait()
}
