package signals

import (
	"net/url"
	"path"
	"strings"
)

// Endpoint classes. Package policy re-exports these constants so callers
// keep one vocabulary; the classifier itself lives here because
// pkg/policy already imports pkg/signals and the reverse edge does not
// exist. The login/checkout/API/static rules intentionally mirror the
// original pkg/policy classifier, and policy.Classify now delegates here.
const (
	ClassLogin    = "login"
	ClassAPI      = "api"
	ClassBrowse   = "browse"
	ClassCheckout = "checkout"
	ClassStatic   = "static_asset"
)

// NormalizePath rejects ambiguous escape forms and removes dot segments.
// The result is used for classification and policy matching only, never
// proxy routing.
func NormalizePath(raw string) (string, bool) {
	if raw == "" {
		return "/", true
	}
	if !strings.HasPrefix(raw, "/") || strings.Contains(raw, "\\") {
		return "", false
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil || strings.Contains(decoded, "\\") || strings.Contains(decoded, "//") || strings.ContainsRune(decoded, 0) {
		return "", false
	}
	if strings.Contains(decoded, "%") {
		return "", false
	}
	return path.Clean(decoded), true
}

// Classify puts a request into one of a few endpoint classes from the
// already-normalized path and method. Fixed, reviewable defaults; unknown
// routes are browse.
func Classify(normalizedPath, method string) string {
	p := strings.ToLower(normalizedPath)
	method = strings.ToUpper(method)
	switch {
	case p == "/login" || p == "/signin" || p == "/sign-in" || strings.HasPrefix(p, "/auth/"):
		return ClassLogin
	case p == "/checkout" || strings.HasPrefix(p, "/checkout/") || p == "/cart" || strings.HasPrefix(p, "/payment/"):
		return ClassCheckout
	case p == "/api" || strings.HasPrefix(p, "/api/"):
		return ClassAPI
	case method == "GET" || method == "HEAD":
		if staticAssetExts[strings.ToLower(path.Ext(p))] {
			return ClassStatic
		}
	}
	return ClassBrowse
}

// ScoreFromFired sums the fixed check weights for an Evaluation.Fired
// bitmask. Training and evaluation use it as the rule scorer's baseline on
// stored samples, where re-running the live checks (Redis state, current
// connections) is impossible — it must stay in lockstep with Evaluate's
// weights, which TestScoreFromFiredMatchesEvaluateWeights pins.
func ScoreFromFired(fired uint32) int {
	score := 0
	for i, c := range checks {
		if fired&(1<<i) != 0 {
			score += c.weight
		}
	}
	return score
}
