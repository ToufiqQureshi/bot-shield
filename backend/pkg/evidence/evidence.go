package evidence

import (
	"sync"
	"time"
)

// How much history the trail keeps. Both limits exist because this
// runs against live adversarial traffic: the size cap stops memory
// growing with request volume, and the age cap stops us holding
// visitor records longer than answering a complaint needs.
const (
	trailSize   = 1000
	trailMaxAge = 24 * time.Hour
)

// Evidence is the record of one decision: enough to answer "why was
// this request stopped?" days later, and nothing more about the
// visitor than the decision itself already used.
type Evidence struct {
	Time     time.Time `json:"time"`
	JA4      string    `json:"ja4"`
	Signals  []string  `json:"signals"`
	Score    int       `json:"score"`
	Decision string    `json:"decision"`
	// Enforced is false when the decision was only recorded, not acted
	// on (shadow mode). Without it a reader cannot tell a real block
	// from one that never happened.
	Enforced bool `json:"enforced"`
	// Model is what the learned model (pkg/decide) would have decided,
	// present only when one is loaded. It never affects Decision: the
	// rule scorer above is what actually ran. Recording both is how a
	// model earns the right to enforce, by being compared against the
	// rules on real traffic first.
	Model *ModelOpinion `json:"model,omitempty"`
	// Policy is what the account's dashboard-authored mitigation rules
	// (pkg/policy) would have decided, present only when a policy
	// provider is attached. Like Model, it never affects Decision — see
	// docs/BACKEND_IMPLEMENTATION_PLAN.md Phase 1: policy output only
	// starts driving enforcement in a later, separately reviewed change.
	Policy *PolicyOpinion `json:"policy,omitempty"`
}

// PolicyOpinion is what one account's mitigation rules would have done
// with a request, per pkg/policy.Evaluate. A nil RuleID means no rule
// matched.
type PolicyOpinion struct {
	Matched  bool   `json:"matched"`
	RuleID   string `json:"ruleId,omitempty"`
	RuleName string `json:"ruleName,omitempty"`
	Action   string `json:"action,omitempty"`
}

// ModelOpinion is the learned model's view of one request. It is a plain
// record rather than the decide.Prediction itself so the evidence trail
// stays a description of what happened and does not depend on the
// scoring package.
type ModelOpinion struct {
	Decision    string  `json:"decision"`
	Probability float64 `json:"probability"`
	Confidence  float64 `json:"confidence"`
	// Reasons is each fired check's push on the decision, strongest
	// first. This is what answers "why" for a model decision, the same
	// way Signals does for the rule score.
	Reasons []ModelReason `json:"reasons,omitempty"`
}

// ModelReason is one check's contribution to a model decision, in
// log-odds. Positive argued the request was automated.
type ModelReason struct {
	Feature string  `json:"feature"`
	Weight  float64 `json:"weight"`
}

// Trail holds the most recent decisions in a fixed-size ring buffer,
// oldest overwritten first. In-memory only, so it resets on restart —
// same limitation as stats.Stats, and durable history needs the planned
// store (docs/ROADMAP.md item 12).
type Trail struct {
	mu     sync.Mutex
	buf    []Evidence
	next   int
	n      int
	maxAge time.Duration
	now    func() time.Time
}

func NewTrail() *Trail {
	return newTrail(trailSize, trailMaxAge)
}

// newTrail refuses a zero size rather than handing back a Trail that
// divides by zero on its first record — a panic here would be in the
// request path, so it fails at construction instead.
func newTrail(size int, maxAge time.Duration) *Trail {
	if size < 1 {
		size = 1
	}
	return &Trail{buf: make([]Evidence, size), maxAge: maxAge, now: time.Now}
}

// Record stamps a decision with the time it happened and stores it,
// dropping the oldest record once the buffer is full.
func (t *Trail) Record(e Evidence) {
	t.mu.Lock()
	defer t.mu.Unlock()

	e.Time = t.now()
	t.buf[t.next] = e
	t.next = (t.next + 1) % len(t.buf)
	if t.n < len(t.buf) {
		t.n++
	}
}

// Recent returns the newest records first, skipping any past the
// retention window. limit <= 0 means "everything still held".
func (t *Trail) Recent(limit int) []Evidence {
	t.mu.Lock()
	defer t.mu.Unlock()

	if limit <= 0 || limit > t.n {
		limit = t.n
	}
	cutoff := t.now().Add(-t.maxAge)

	out := make([]Evidence, 0, limit)
	for i := 0; i < t.n && len(out) < limit; i++ {
		e := t.buf[(t.next-1-i+2*len(t.buf))%len(t.buf)]
		// Records sit in time order, so the first one past the window
		// means every older one is too.
		if e.Time.Before(cutoff) {
			break
		}
		out = append(out, e)
	}
	return out
}
