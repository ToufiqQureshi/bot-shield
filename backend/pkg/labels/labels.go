// Package labels collects labelled traffic for the learned scorer
// (pkg/decide) to train on.
//
// These are candidate labels, not verified ground truth. Labelling from
// the rule score would teach the model to repeat its own guesses. Two
// independent observations are collected:
//
//   - a solved challenge, which can also be forged by a purpose-built client
//   - a honeypot hit, which can also come from prefetch or accessibility tools
//
// docs/LEARNED_SCORING.md is the full write-up: which other sources look
// obvious and are traps, the selection bias in this data, and why
// training must never run automatically.
//
// Nothing here is allowed to slow the request path down. Recording is a
// non-blocking send to a bounded queue; when the queue is full, samples
// are dropped and counted rather than making a customer's visitor wait
// on a database.
package labels

import (
	"context"
	"sync"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/observability"
)

// Sample is one candidate-labelled request.
//
// It deliberately holds no IP, user agent, path or body. A model trains
// on which checks fired, and nothing else here is worth the storage, the
// retention argument, or the risk of holding it.
type Sample struct {
	// TenantID scopes the sample. One customer's traffic must never
	// shape another's model (CLAUDE.md Section 16).
	TenantID string
	// Fired is the check bitmask, signals.Evaluation.Fired.
	Fired uint32
	// FeatureVersion names the check list that produced Fired. The mask
	// is positional, so a sample without this is not interpretable.
	FeatureVersion string
	// Automated is the observed label, not a verified classification.
	Automated bool
	// Source records provenance so candidate observations can be reviewed
	// or excluded during training.
	Source string
	// Identity is the (tenant, IP, JA4) triple this sample came from. It
	// is used to cap how much one client can contribute and is never
	// persisted: a stored sample is not meant to identify a visitor.
	Identity string
}

// Label sources.
const (
	SourceChallengeSolved = "challenge_solved"
	SourceHoneypotTrap    = "honeypot_trap"
)

// Writer persists samples. pkg/db implements it.
type Writer interface {
	WriteSamples(ctx context.Context, samples []Sample) error
}

const (
	// queueSize bounds what we hold while the database is slow. Samples
	// are worth having, not worth an unbounded queue in front of a
	// customer's site (CLAUDE.md Section 15).
	queueSize = 4096

	// batchSize and flushEvery trade write frequency against how long a
	// sample sits in memory where a restart loses it. Losing a few
	// samples on restart is fine; they are a stream, not a ledger.
	batchSize  = 64
	flushEvery = 5 * time.Second

	// writeTimeout bounds one database write. The collector is off the
	// request path, but an unbounded write would still stall the queue
	// behind it until it filled and started dropping.
	writeTimeout = 5 * time.Second
)

// Collector takes samples from the request path and writes them in the
// background. Create it with NewCollector and stop it with Close.
type Collector struct {
	writer Writer
	queue  chan Sample
	cap    *identityCap

	// closing guards queue against a send that races Close. A select
	// with a default case does not stop a send on a closed channel from
	// panicking, and the race is reachable: http.Server.Shutdown returns
	// when its timeout expires while the handlers it gave up on are
	// still running, and the deferred Close in main.go then runs
	// underneath them.
	closing sync.RWMutex
	closed  bool

	done chan struct{}
}

// NewCollector starts the background writer. A nil writer makes every
// Record a no-op, which is the normal state when no database is
// configured.
func NewCollector(w Writer) *Collector {
	if w == nil {
		return nil
	}
	c := &Collector{
		writer: w,
		queue:  make(chan Sample, queueSize),
		cap:    newIdentityCap(),
		done:   make(chan struct{}),
	}
	go c.run()
	return c
}

// Record queues one sample. It never blocks and never returns an error:
// it is called from the request path, where waiting on a database would
// make a customer's visitor wait too.
//
// A nil Collector records nothing, so callers do not need a nil check.
func (c *Collector) Record(s Sample) {
	if c == nil {
		return
	}
	if s.TenantID == "" || s.FeatureVersion == "" || s.Source == "" {
		// An unattributable sample cannot be scoped to a tenant or
		// interpreted later, so it is worse than no sample.
		observability.Inc("label_sample_invalid_total")
		return
	}
	// One client must not be able to fill the training set with its own
	// labels. A bot that deliberately solves challenges is injecting
	// "human" labels for its own fingerprint, and without a cap it can
	// do that as often as it likes (docs/LEARNED_SCORING.md).
	if !c.cap.allow(s.Identity, time.Now()) {
		observability.Inc("label_sample_capped_total")
		return
	}

	// Held across the send so Close cannot shut the queue mid-send. It
	// is a read lock, so concurrent requests still record in parallel;
	// only Close excludes them, once, at shutdown.
	c.closing.RLock()
	defer c.closing.RUnlock()
	if c.closed {
		// Shutting down. The sample is lost, which is the same outcome
		// as a sample still queued when the process exits.
		observability.Inc("label_sample_dropped_total")
		return
	}

	select {
	case c.queue <- s:
		observability.Inc("label_sample_queued_total")
	default:
		// Full. Dropping is the designed behaviour, but it is counted:
		// a queue that is always full means the writer cannot keep up,
		// and a silently shrinking training set is the kind of thing
		// nobody notices until the model is wrong.
		observability.Inc("label_sample_dropped_total")
	}
}

// Close stops the background writer and flushes what is queued. It is
// safe to call more than once.
func (c *Collector) Close() {
	if c == nil {
		return
	}
	c.closing.Lock()
	first := !c.closed
	c.closed = true
	if first {
		close(c.queue)
	}
	c.closing.Unlock()
	// Waited on outside the lock: the writer must not be able to block
	// a request that is holding the read lock on its way out.
	<-c.done
}

// run batches queued samples and writes them. It exits when the queue is
// closed, flushing whatever is left.
func (c *Collector) run() {
	defer close(c.done)

	ticker := time.NewTicker(flushEvery)
	defer ticker.Stop()

	batch := make([]Sample, 0, batchSize)
	for {
		select {
		case s, ok := <-c.queue:
			if !ok {
				c.flush(batch)
				return
			}
			batch = append(batch, s)
			if len(batch) >= batchSize {
				c.flush(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			// A partial batch must not sit in memory indefinitely on a
			// quiet site, where it would be lost on the next restart.
			if len(batch) > 0 {
				c.flush(batch)
				batch = batch[:0]
			}
		}
	}
}

// flush writes one batch. A failed write loses that batch: these are
// samples, not transactions, and retrying in front of a database that is
// already struggling would only make it worse.
func (c *Collector) flush(batch []Sample) {
	if len(batch) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()

	if err := c.writer.WriteSamples(ctx, batch); err != nil {
		observability.Inc("label_write_failed_total")
		return
	}
	observability.Add("label_written_total", len(batch))
}
