package core

import (
	"net/http"
	"strings"

	"github.com/ToufiqQureshi/bot-shield/pkg/challenge"
	"github.com/ToufiqQureshi/bot-shield/pkg/config"
	"github.com/ToufiqQureshi/bot-shield/pkg/evidence"
	"github.com/ToufiqQureshi/bot-shield/pkg/signals"
	"github.com/ToufiqQureshi/bot-shield/pkg/tenant"
)

// Guard is the first thing in this codebase that actually acts on a
// signal instead of just labeling it (ROADMAP item 5). It scores each
// request and either forwards it to the origin, serves the JS
// challenge in its place, or refuses it outright.
type Guard struct {
	store     *tenant.Store
	challenge *challenge.Challenge
}

// NewGuard combines the tenant store with a challenge.Challenge instance
// into the real allow/challenge/block decision. In config.ModeShadow it
// scores and records exactly the same way but never acts (item 18).
func NewGuard(store *tenant.Store, challenge *challenge.Challenge) *Guard {
	return &Guard{store: store, challenge: challenge}
}

// ServeHTTP decides per request. A visitor who already solved a
// challenge is forwarded straight through — re-challenging someone who
// already proved they're a browser would just be a worse experience
// for no extra signal (CLAUDE.md Section 8).
func (g *Guard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Look up the tenant by the incoming Host header.
	// Strip port if present, as DNS/CNAME doesn't include it.
	host := r.Host
	if idx := strings.IndexByte(host, ':'); idx != -1 {
		host = host[:idx]
	}

	tenant, err := g.store.GetByHost(host)
	if err != nil {
		// If we don't recognize the customer domain, drop the request.
		// A 421 Misdirected Request is the most accurate HTTP status here.
		http.Error(w, "misdirected request", 421)
		return
	}

	ja4 := JA4FromContext(r.Context())
	enforced := tenant.Config.Mode == config.ModeEnforce

	if g.challenge.Passed(r) {
		tenant.Stats.Record(signals.DecisionAllow)
		// Recorded as its own reason, not as "scored zero" ?" otherwise
		// the trail would claim this visitor looked clean when really
		// they had already proven themselves.
		tenant.Trail.Record(evidence.Evidence{JA4: ja4, Signals: []string{"challenge_solved"}, Decision: signals.DecisionAllow.String(), Enforced: enforced})
		tenant.Origin.ServeHTTP(w, r)
		return
	}

	score := signals.Score(ja4, r.UserAgent())
	decision := signals.Decide(score)
	tenant.Trail.Record(evidence.Evidence{
		JA4:      ja4,
		Signals:  signals.Analyze(ja4, r.UserAgent()),
		Score:    score,
		Decision: decision.String(),
		Enforced: enforced,
	})

	// The dashboard wants to know what would have happened, but the
	// visitor is forwarded regardless. Nothing a client's real customer
	// does can be broken by a score while this is on.
	if !enforced {
		tenant.Stats.Record(decision)
		tenant.Origin.ServeHTTP(w, r)
		return
	}

	tenant.Stats.Record(decision)
	switch decision {
	case signals.DecisionBlock:
		http.Error(w, "forbidden", http.StatusForbidden)
	case signals.DecisionChallenge:
		g.challenge.Serve(w, r)
	default:
		tenant.Origin.ServeHTTP(w, r)
	}
}
