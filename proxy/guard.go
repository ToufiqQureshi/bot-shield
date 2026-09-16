package proxy

import "net/http"

// Guard is the first thing in this codebase that actually acts on a
// signal instead of just labeling it (ROADMAP item 5). It scores each
// request and either forwards it to the origin, serves the JS
// challenge in its place, or refuses it outright.
type Guard struct {
	origin    http.Handler
	challenge *Challenge
	stats     *Stats
	trail     *Trail
}

// NewGuard combines an already-built proxy with a Challenge instance
// into the real allow/challenge/block decision, counting outcomes in
// stats for the dashboard (ROADMAP item 12) and recording why each one
// was made in trail (item 12a).
func NewGuard(origin http.Handler, challenge *Challenge, stats *Stats, trail *Trail) *Guard {
	return &Guard{origin: origin, challenge: challenge, stats: stats, trail: trail}
}

// ServeHTTP decides per request. A visitor who already solved a
// challenge is forwarded straight through — re-challenging someone who
// already proved they're a browser would just be a worse experience
// for no extra signal (CLAUDE.md Section 8).
func (g *Guard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ja4 := JA4FromContext(r.Context())

	if g.challenge.Passed(r) {
		g.stats.recordAllow()
		// Recorded as its own reason, not as "scored zero" — otherwise
		// the trail would claim this visitor looked clean when really
		// they had already proven themselves.
		g.trail.record(Evidence{JA4: ja4, Signals: []string{"challenge_solved"}, Decision: DecisionAllow.String()})
		g.origin.ServeHTTP(w, r)
		return
	}

	score := Score(ja4, r.UserAgent())
	decision := Decide(score)
	g.trail.record(Evidence{
		JA4:      ja4,
		Signals:  signals(ja4, r.UserAgent()),
		Score:    score,
		Decision: decision.String(),
	})

	switch decision {
	case DecisionBlock:
		g.stats.recordBlock()
		http.Error(w, "forbidden", http.StatusForbidden)
	case DecisionChallenge:
		g.stats.recordChallenge()
		g.challenge.Serve(w, r)
	default:
		g.stats.recordAllow()
		g.origin.ServeHTTP(w, r)
	}
}
