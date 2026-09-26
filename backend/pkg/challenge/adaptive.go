package challenge

import (
	"context"
	"time"
)

// riskKey carries the request's already-computed risk score from
// core.Guard into the challenge. It is a context value rather than a
// parameter so the challenge's public API keeps the shape every existing
// caller already uses, and a caller that supplies nothing simply gets
// the lightest puzzle.
type riskKey struct{}

// WithRisk attaches the rule score for this request so Serve can pick a
// difficulty. A missing value is treated as zero risk.
func WithRisk(ctx context.Context, score int) context.Context {
	return context.WithValue(ctx, riskKey{}, score)
}

// RiskFromContext returns the attached score, or 0.
func RiskFromContext(ctx context.Context) int {
	if v, ok := ctx.Value(riskKey{}).(int); ok {
		return v
	}
	return 0
}

// Adaptive challenge (docs/DECISIONS.md, Challenge): the challenge a visitor
// receives is not fixed. Difficulty is chosen by the server from the risk
// score it already computed for the request, and from how many times this
// client has already failed to solve one. Both inputs are bounded, so a
// visitor can never be handed a puzzle that a phone cannot finish.
//
// A difficulty is the number of leading hex zeros required in
// SHA-256(nonce + answer). Each extra zero multiplies the expected work by
// 16, so the range is deliberately tiny:
//
//	difficulty 1 =  16 average attempts   (clean/strict traffic: effectively free)
//	difficulty 2 = 256 average attempts   (the original fixed puzzle)
//	difficulty 3 = 4096 average attempts  (highest-risk traffic, still <1s)
//
// The cap is the mobile-safety boundary. Do not raise maxDifficulty without
// measuring a real mid-range phone: the page computes each hash with an
// awaited crypto.subtle.digest, which is far slower than a native loop.
const (
	minDifficulty = 1
	maxDifficulty = 3
)

// attemptsPerStep is how many failed solves bump the difficulty by one
// step. Two is intentional: one fat-fingered typo must not make a real
// visitor's next page load measurably slower.
const attemptsPerStep = 2

// maxAttempts caps the signed attempt counter. Beyond the cap the
// difficulty is already at maxDifficulty, and an unbounded counter would
// let a client inflate a cookie value for no effect.
const maxAttempts = 6

// difficultyForScore maps the request's risk score to a base difficulty.
// Clean traffic gets the lightest puzzle; anything that already fired a
// signal gets the original 8-bit puzzle; the stronger band gets 12 bits.
func difficultyForScore(score int) int {
	switch {
	case score <= 0:
		return minDifficulty
	case score < scoreMidBand:
		return minDifficulty + 1
	default:
		return maxDifficulty
	}
}

// difficultyFor combines risk with prior failures and clamps the result
// into [minDifficulty, maxDifficulty]. Clamping lives here, in one place,
// so no caller can accidentally hand a visitor a puzzle outside the
// mobile-safe range.
func difficultyFor(score, priorAttempts int) int {
	steps := priorAttempts / attemptsPerStep
	d := difficultyForScore(score) + steps
	if d > maxDifficulty {
		d = maxDifficulty
	}
	if d < minDifficulty {
		d = minDifficulty
	}
	return d
}

// passedTrustWindow is the second half of Phase 2's trust decay: one
// solve must not buy a permanent pass, and a visitor who needed the
// hardest puzzle is trusted for less time than one who sailed through.
// The window is carried inside the signed passed cookie, so it survives
// a restart and cannot be extended by the client.
func passedTrustWindow(difficulty int) time.Duration {
	switch {
	case difficulty >= maxDifficulty:
		return 5 * time.Minute
	case difficulty >= minDifficulty+1:
		return 15 * time.Minute
	default:
		return passedMaxAge
	}
}
