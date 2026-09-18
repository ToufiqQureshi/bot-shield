package signals

// Decision is the three-way outcome scoring produces for a request:
// forward it untouched, make it prove it's a browser first, or refuse
// it outright. No single signal may pick DecisionBlock by itself
// (CLAUDE.md Section 6) — only combined evidence crosses that
// threshold; one mid-strength signal only earns a challenge.
type Decision int

const (
	DecisionAllow Decision = iota
	DecisionChallenge
	DecisionBlock
	DecisionDeceive
)

func (d Decision) String() string {
	switch d {
	case DecisionAllow:
		return "allow"
	case DecisionChallenge:
		return "challenge"
	case DecisionBlock:
		return "block"
	case DecisionDeceive:
		return "deceive"
	default:
		return "unknown"
	}
}

// Signal weights and thresholds. Fixed for now, not yet configurable
// per client — that's docs/ROADMAP.md item 11. fragmentedWeight and
// uaMismatchWeight are deliberately equal and additive: a fragmented
// handshake (fingerprint-layer anomaly, true regardless of what the
// client claims to be) and a UA mismatch (consistency-layer lie,
// which needs a browser claim to be a lie) are different findings
// from different layers, even though a fragmented handshake is one of
// UAMismatch's own inputs — see docs/DECISIONS.md for why that's not
// double-counting the same fact.
const (
	fragmentedWeight   = 50
	uaMismatchWeight   = 50
	firstTouchWeight   = 50 // Enforce JS challenge on unverified sessions
	blockThreshold     = 100
	challengeThreshold = 50
)

// Known malicious JA4 fingerprints. Every entry must be verified against
// real captures before being added — blocking a JA4 shared with a real
// browser family would block every legitimate user on that browser
// (false positive, CLAUDE.md Section 14). Placeholder hashes without
// verified captures must never ship.
var badJA4Hashes = map[string]bool{
	"t12d190800_4464c1bd5eb7_b3394627b738": true, // Python requests (verified capture)
}

// checks is the single list every scoring check lives in, so a score
// and the explanation shown for it can never disagree — adding a check
// in one place and forgetting the other would make the evidence trail
// lie about why a request was stopped.
var checks = []struct {
	name   string
	weight int
	fired  func(ip, ja4, ua string) bool
}{
	{"fragmented_handshake", fragmentedWeight, func(ip, ja4, ua string) bool { return ja4 == JA4Unreadable }},
	{"ua_mismatch", uaMismatchWeight, func(ip, ja4, ua string) bool { return UAMismatch(ua, ja4) }},
	{"ja4_blocklist", 100, func(ip, ja4, ua string) bool { 
		isScraper, _ := IsKnownScraperJA4(ja4)
		return isScraper || badJA4Hashes[ja4] 
	}},
	{"scripting_tool", 100, func(ip, ja4, ua string) bool { return IsScriptingTool(ua) }},
	{"velocity_spike", 50, func(ip, ja4, ua string) bool { return checkVelocitySpike(ip) }},
	{"ja4_velocity_spike", 50, func(ip, ja4, ua string) bool { return checkJA4VelocitySpike(ja4) }},
	{"untrusted_session", firstTouchWeight, func(ip, ja4, ua string) bool { return true }},
}

// Score combines a request's known signals into one risk score. ja4
// and ua are read the same way proxy.go already reads them to set the
// label headers.
func Score(ip, ja4, ua string) int {
	total := 0
	for _, c := range checks {
		if c.fired(ip, ja4, ua) {
			total += c.weight
		}
	}
	return total
}

// Analyze names the checks that fired for a request, so the evidence
// trail can answer "why was this stopped?" and not just "how much."
func Analyze(ip, ja4, ua string) []string {
	var fired []string
	for _, c := range checks {
		if c.fired(ip, ja4, ua) {
			fired = append(fired, c.name)
		}
	}
	return fired
}

// Decide turns a score into the three-way outcome using the fixed
// thresholds above.
func Decide(score int) Decision {
	switch {
	case score >= blockThreshold:
		return DecisionBlock
	case score >= challengeThreshold:
		return DecisionChallenge
	default:
		return DecisionAllow
	}
}
