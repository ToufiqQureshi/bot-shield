package core

import (
	"net/http"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/challenge"
	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/decide"
	"github.com/ToufiqQureshi/hakaishield/pkg/evidence"
	"github.com/ToufiqQureshi/hakaishield/pkg/labels"
	"github.com/ToufiqQureshi/hakaishield/pkg/observability"
	"github.com/ToufiqQureshi/hakaishield/pkg/policy"
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
	// labels, when set, collects labelled traffic for the learned scorer
	// to train on. Like model, it observes and never decides.
	labels *labels.Recorder
	// model, when set, scores every request alongside the rule scorer and
	// records what it would have done. It never decides anything: a model
	// is allowed to enforce only after its recorded disagreements have
	// been looked at on real traffic. Nil is the normal state and costs
	// nothing.
	model *decide.Model
	// policyProvider, when set, is asked for the requesting tenant's
	// dashboard-authored mitigation policy so it can be evaluated
	// alongside the rule scorer, the same shadow-only pattern as model.
	// It never picks Decision (docs/BACKEND_IMPLEMENTATION_PLAN.md Phase
	// 1 guardrail: policy output earns enforcement in a later, separate
	// change). Nil is the normal state and costs nothing.
	policyProvider PolicyProvider
}

// PolicyProvider returns the current policy for one tenant, or nil when
// that tenant has none configured. It is a function rather than a
// concrete store so core.Guard never has to import the storage/database
// details of how a policy is assembled from an account's rules — see
// pkg/policy's doc comment on why that package stays storage-agnostic.
type PolicyProvider func(tenantID string) *policy.Policy

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

// WithLabelRecorder attaches label collection for the learned scorer
// (pkg/decide). Passing nil turns it off, which is the default.
//
// Call it during setup, before the guard serves traffic: the recorder is
// read without locking on the request path.
func (g *Guard) WithLabelRecorder(r *labels.Recorder) *Guard {
	g.labels = r
	if g.challenge != nil {
		// The solve lands on the challenge handler, not here, so it
		// needs the same recorder to pair the nonce with the sample.
		g.challenge.SetLabelRecorder(r)
	}
	return g
}

// WithShadowModel attaches a trained model that scores alongside the rule
// scorer without affecting any decision. Passing nil turns it off again.
//
// Call it during setup, before the guard serves traffic. The model is
// read-only once attached, so requests share it without locking, but
// swapping it on a guard that is already serving would be a data race.
func (g *Guard) WithShadowModel(m *decide.Model) *Guard {
	g.model = m
	return g
}

// WithPolicyProvider attaches a lookup for each tenant's dashboard rules
// so they can be evaluated in shadow mode. Passing nil turns it off,
// which is the default.
//
// Call it during setup, before the guard serves traffic, for the same
// reason as WithShadowModel: the field is read without locking on the
// request path.
func (g *Guard) WithPolicyProvider(p PolicyProvider) *Guard {
	g.policyProvider = p
	return g
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
	r = r.WithContext(WithClientIP(r.Context(), ip))

	// Look up the tenant by the incoming Host header.
	// Strip port if present, as DNS/CNAME doesn't include it.
	host, ok := validatedRequestHost(r)
	if !ok {
		observability.Inc("request_host_malformed_total")
		http.Error(w, "bad host", http.StatusBadRequest)
		return
	}
	if !hostMatchesTLS(r, host) {
		observability.Inc("request_host_sni_mismatch_total")
		http.Error(w, "misdirected request", http.StatusMisdirectedRequest)
		return
	}

	tenant, err := g.store.GetByHost(host)
	if err != nil {
		// If we don't recognize the customer domain, drop the request.
		// A 421 Misdirected Request is the most accurate HTTP status here.
		observability.Inc("request_unknown_host_total")
		http.Error(w, "misdirected request", http.StatusMisdirectedRequest)
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
			if g.labels != nil {
				// Capture the trap request itself. A crawler may leave after
				// fetching this URL, so waiting for a later request loses it.
				fired := signals.Evaluate(signals.RequestFacts{
					IP: ip, JA4: ja4, UA: r.UserAgent(), Header: r.Header,
					Path: r.URL.Path, Tenant: tenant.ID,
				}).Fired
				g.labels.HoneypotTripped(labels.Sample{
					TenantID: tenant.ID, Fired: fired &^ honeypotBit,
					FeatureVersion: signals.FeatureVersion(),
					Identity:       labelIdentity(tenant.ID, ip, ja4),
				})
			}
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
		Model:    g.shadowOpinion(evaluation.Fired, decision),
		Policy:   g.shadowPolicyOpinion(tenant.ID, facts, score, r.Method),
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
		// Carry what this request looked like into the challenge, so a
		// solve can label it human. Nothing about the challenge itself
		// changes.
		g.challenge.Serve(w, r.WithContext(labels.WithSample(r.Context(), labels.Sample{
			TenantID:       tenant.ID,
			Fired:          evaluation.Fired,
			FeatureVersion: signals.FeatureVersion(),
			Identity:       labelIdentity(tenant.ID, ip, ja4),
		})))
	default:
		tenant.Origin.ServeHTTP(w, r)
	}
}

// shadowOpinion scores a request with the learned model, if one is
// loaded, and returns what it would have decided. It returns nil when no
// model is configured, which is the normal case and the reason this costs
// nothing by default.
//
// Disagreements are counted, not just recorded: the trail is a bounded
// ring buffer that a busy tenant overwrites within minutes, so a counter
// is the only thing that survives long enough to answer "how often does
// the model differ from the rules?" — which is the question that decides
// whether a model may ever enforce.
func (g *Guard) shadowOpinion(fired uint32, ruleDecision signals.Decision) *evidence.ModelOpinion {
	if g.model == nil {
		return nil
	}

	p := g.model.Predict(fired)
	if p.Decision == ruleDecision {
		observability.Inc("model_shadow_agree_total")
	} else {
		observability.Inc("model_shadow_disagree_total")
	}

	contributions := g.model.Explain(fired)
	reasons := make([]evidence.ModelReason, len(contributions))
	for i, c := range contributions {
		reasons[i] = evidence.ModelReason{Feature: c.Feature, Weight: c.Weight}
	}

	return &evidence.ModelOpinion{
		Decision:    p.Decision.String(),
		Probability: p.Probability,
		Confidence:  p.Confidence,
		Reasons:     reasons,
	}
}

// shadowPolicyOpinion evaluates the tenant's dashboard rules against
// this request, if a provider is attached, and returns what they would
// have done. It returns nil when no provider is configured (the normal
// case) or when the tenant has no policy — both cost nothing beyond the
// provider lookup itself.
//
// Like shadowOpinion, this never influences ruleDecision: pkg/policy is
// evaluated purely for the evidence trail until a later, separately
// reviewed change lets it drive enforcement.
func (g *Guard) shadowPolicyOpinion(tenantID string, facts signals.RequestFacts, score int, method string) *evidence.PolicyOpinion {
	if g.policyProvider == nil {
		return nil
	}
	p := g.policyProvider(tenantID)
	if p == nil {
		return nil
	}

	result := policy.Evaluate(p, policy.Facts{
		IP:     facts.IP,
		JA4:    facts.JA4,
		UA:     facts.UA,
		Path:   facts.Path,
		Method: method,
		Score:  score,
	})
	if result.Matched {
		observability.Inc("policy_shadow_match_total")
	}
	return &evidence.PolicyOpinion{
		Matched:  result.Matched,
		RuleID:   result.RuleID,
		RuleName: result.RuleName,
		Action:   string(result.Action),
	}
}

// honeypotBit is resolved once so the trap cannot teach the model its
// own label. A zero bit is safe: the trap cannot appear in a sample.
var honeypotBit = func() uint32 {
	bit, _ := signals.FeatureBit(signals.FeatureHoneypotTrap)
	return bit
}()

// labelIdentity is the key the per-identity sample cap counts against.
// It is never stored with the sample: it exists to stop one client
// filling the training set, not to identify a visitor afterwards.
func labelIdentity(tenantID, ip, ja4 string) string {
	return tenantID + "|" + ip + "|" + ja4
}
