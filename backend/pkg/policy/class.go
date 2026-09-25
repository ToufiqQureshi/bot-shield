package policy

import "github.com/ToufiqQureshi/hakaishield/pkg/signals"

// Endpoint classes, re-exported so pkg/policy callers keep their existing
// vocabulary. The definitions live in pkg/signals (see the note on the
// import direction there).
const (
	ClassLogin    = signals.ClassLogin
	ClassAPI      = signals.ClassAPI
	ClassBrowse   = signals.ClassBrowse
	ClassCheckout = signals.ClassCheckout
	ClassStatic   = signals.ClassStatic
)

// NormalizePath rejects ambiguous escape forms and removes dot segments.
// It forwards to the shared implementation in pkg/signals.
func NormalizePath(raw string) (string, bool) {
	return signals.NormalizePath(raw)
}

// Classify puts a request into one of a few endpoint classes from the
// already-normalized path and method. It forwards to the shared
// implementation in pkg/signals so the policy engine and the request-path
// velocity buckets classify every URL the same way.
func Classify(normalizedPath, method string) string {
	return signals.Classify(normalizedPath, method)
}

// ClassifyRoute applies an activated policy's exact path labels. Shadow
// drafts never alter live rate limits, and malformed paths keep the default.
func (p *Policy) ClassifyRoute(rawPath, method string) string {
	normalized, ok := signals.NormalizePath(rawPath)
	if !ok {
		return signals.ClassBrowse
	}
	defaultClass := signals.Classify(normalized, method)
	if defaultClass == ClassLogin || defaultClass == ClassCheckout {
		return defaultClass
	}
	if p != nil && p.Mode == "enforce" {
		if class := p.RouteClasses[normalized]; class == ClassLogin || class == ClassCheckout {
			return class
		}
	}
	return defaultClass
}

func validClass(class string) bool {
	switch class {
	case ClassLogin, ClassAPI, ClassBrowse, ClassCheckout, ClassStatic:
		return true
	}
	return false
}
