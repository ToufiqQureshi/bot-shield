package core

import (
	"net/http"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/challenge"
	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/evidence"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"
)

// Guard is the first thing in this codebase that actually acts on a
// signal instead of just labeling it (ROADMAP item 5). It scores each
// request and either forwards it to the origin, serves the JS
// challenge in its place, or refuses it outright.
type Guard struct {
	store     *tenant.Store
	challenge *challenge.Challenge
	clientIP  *ClientIPResolver
}

// NewGuard combines the tenant store with a challenge.Challenge instance
// into the real allow/challenge/block decision. In config.ModeShadow it
// scores and records exactly the same way but never acts (item 18).
func NewGuard(store *tenant.Store, challenge *challenge.Challenge) *Guard {
	return NewGuardWithClientIPResolver(store, challenge, nil)
}

// NewGuardWithClientIPResolver opts into forwarding-header client identity only
// when the caller supplies a resolver with explicit trusted proxy CIDRs.
func NewGuardWithClientIPResolver(store *tenant.Store, challenge *challenge.Challenge, clientIP *ClientIPResolver) *Guard {
	if clientIP == nil {
		clientIP = &ClientIPResolver{}
	}
	return &Guard{store: store, challenge: challenge, clientIP: clientIP}
}

// ServeHTTP decides per request. A visitor who already solved a
// challenge is forwarded straight through — re-challenging someone who
// already proved they're a browser would just be a worse experience
// for no extra signal (CLAUDE.md Section 8).
func (g *Guard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Health check endpoint for cloud load balancers and container orchestrators.
	if r.URL.Path == "/__hakaishield/healthz" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","timestamp":"` + time.Now().UTC().Format(time.RFC3339) + `"}`))
		return
	}

	ja4 := JA4FromContext(r.Context())

	ip := g.clientIP.ClientIP(r)

	// Look up the tenant by the incoming Host header.
	// Strip port if present, as DNS/CNAME doesn't include it.
	host := requestHost(r)

	tenant, err := g.store.GetByHost(host)
	if err != nil {
		// If we don't recognize the customer domain, drop the request.
		// A 421 Misdirected Request is the most accurate HTTP status here.
		http.Error(w, "misdirected request", 421)
		return
	}

	enforced := tenant.Config.Mode == config.ModeEnforce

	// Honeypot trap. The trap link is injected into deceived HTML
	// responses (pkg/deception) and is invisible to humans, so a fetch
	// of this path is evidence that something walked the DOM. It is
	// recorded against this tenant only, and scored rather than blocked
	// outright — the decision still comes from combined signals.
	//
	// This is deliberately below the tenant lookup: recording against a
	// host we don't serve would let anyone pointing a DNS record at us
	// write into detection state for free.
	if r.URL.Path == signals.HoneypotPath {
		// Only the first trip from a caller is counted. The trail is a
		// fixed-size ring buffer, so recording every hit would let one
		// bot in a loop evict this customer's real decision history.
		if firstTrip := signals.RecordHoneypotTrip(tenant.ID, ip, ja4); firstTrip {
			tenant.Stats.Record(signals.DecisionBlock)
			tenant.Trail.Record(evidence.Evidence{
				JA4:      ja4,
				Signals:  []string{"honeypot_trap"},
				Decision: signals.DecisionBlock.String(),
				Enforced: enforced,
			})
		}
		// A 404 gives the crawler nothing back: no hint the path was
		// special, and no body worth fetching again.
		http.NotFound(w, r)
		return
	}

	// SEO & Search Engine Crawler Protection:
	// Genuine verified search engine bots (Googlebot, Bingbot, Applebot) with matching
	// reverse-forward DNS are forwarded directly without friction or challenges.
	if signals.IsVerifiedGoodBot(ip, r.UserAgent()) {
		tenant.Stats.Record(signals.DecisionAllow)
		tenant.Trail.Record(evidence.Evidence{
			JA4:      ja4,
			Signals:  []string{"good_bot_verified"},
			Decision: signals.DecisionAllow.String(),
			Enforced: enforced,
		})
		tenant.Origin.ServeHTTP(w, r)
		return
	}

	if g.challenge.Passed(r) {
		// A solved challenge proves this client could run JS once; it
		// says nothing about the volume of requests after that. Without
		// this check, one solve buys unlimited-speed access to the
		// origin for the rest of passedMaxAge (CLAUDE.md Section 15/18 —
		// bounded resource use, can't let a visitor exhaust the origin).
		if signals.VelocityExceeded(ip, ja4, r.URL.Path) {
			tenant.Stats.Record(signals.DecisionBlock)
			tenant.Trail.Record(evidence.Evidence{JA4: ja4, Signals: []string{"velocity_after_pass"}, Decision: signals.DecisionBlock.String(), Enforced: enforced})
			if enforced {
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}
			tenant.Origin.ServeHTTP(w, r)
			return
		}
		tenant.Stats.Record(signals.DecisionAllow)
		// Recorded as its own reason, not as "scored zero", otherwise
		// the trail would claim this visitor looked clean when really
		// they had already proven themselves.
		tenant.Trail.Record(evidence.Evidence{JA4: ja4, Signals: []string{"challenge_solved"}, Decision: signals.DecisionAllow.String(), Enforced: enforced})
		tenant.Origin.ServeHTTP(w, r)
		return
	}

	facts := signals.RequestFacts{
		IP:     ip,
		JA4:    ja4,
		UA:     r.UserAgent(),
		Header: r.Header,
		Path:   r.URL.Path,
		Tenant: tenant.ID,
	}
	evaluation := signals.Evaluate(facts)
	score := evaluation.Score
	decision := signals.DecideWithPolicy(score, tenant.Config.Policy)
	if decision == signals.DecisionBlock && tenant.Config.Deception {
		decision = signals.DecisionDeceive
	}
	tenant.Trail.Record(evidence.Evidence{
		JA4:      ja4,
		Signals:  evaluation.Signals,
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
	case signals.DecisionDeceive:
		// ROADMAP Item 11a: Deception mode (decoy response).
		// Forward the request with X-HakaiShield-Decision: deceive so the origin
		// can serve dummy data/poisoned pricing and waste the scraper's resources.
		ctx := WithDecision(r.Context(), signals.DecisionDeceive.String(), score)
		tenant.Origin.ServeHTTP(w, r.WithContext(ctx))
	case signals.DecisionChallenge:
		g.challenge.Serve(w, r)
	default:
		tenant.Origin.ServeHTTP(w, r)
	}
}
