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
}

// NewGuard combines an already-built proxy with a Challenge instance
// into the real allow/challenge/block decision, counting outcomes in
// stats for the dashboard (ROADMAP item 12).
func NewGuard(origin http.Handler, challenge *Challenge, stats *Stats) *Guard {
	return &Guard{origin: origin, challenge: challenge, stats: stats}
}

// ServeHTTP decides per request. A visitor who already solved a
// challenge is forwarded straight through — re-challenging someone who
// already proved they're a browser would just be a worse experience
// for no extra signal (CLAUDE.md Section 8).
func (g *Guard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if g.challenge.Passed(r) {
		g.stats.recordAllow()
		g.origin.ServeHTTP(w, r)
		return
	}

	ja4 := JA4FromContext(r.Context())
	score := Score(ja4, r.UserAgent())

	switch Decide(score) {
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
