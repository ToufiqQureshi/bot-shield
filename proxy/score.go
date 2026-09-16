package proxy

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
)

func (d Decision) String() string {
	switch d {
	case DecisionAllow:
		return "allow"
	case DecisionChallenge:
		return "challenge"
	case DecisionBlock:
		return "block"
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
	blockThreshold     = 100
	challengeThreshold = 50
)

// Score combines a request's known signals into one risk score. ja4
// and ua are read the same way proxy.go already reads them to set the
// label headers.
func Score(ja4, ua string) int {
	score := 0
	if ja4 == JA4Unreadable {
		score += fragmentedWeight
	}
	if UAMismatch(ua, ja4) {
		score += uaMismatchWeight
	}
	return score
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
