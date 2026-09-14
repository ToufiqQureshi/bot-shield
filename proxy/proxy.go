// Package proxy is the reverse proxy that sits in front of a
// client's origin server. It is the integration point every
// detection layer (fingerprint, score, challenge) plugs into.
package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// ja4Header is the header we attach to the forwarded request so the
// origin (and later, our own scoring code) can see the caller's JA4
// fingerprint without re-capturing it themselves.
const ja4Header = "X-BotShield-JA4"

// New builds a reverse proxy that forwards every request to target.
// It fails immediately on a bad target instead of at the first
// request, so a broken config can't reach production traffic.
func New(target string) (*httputil.ReverseProxy, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, &url.Error{Op: "parse", URL: target, Err: errInvalidTarget}
	}

	p := httputil.NewSingleHostReverseProxy(u)
	director := p.Director
	// Passes along the JA4 fingerprint captured for this connection,
	// if there was one. A request with no fingerprint (plain HTTP, or
	// TLS capture wasn't set up) is forwarded as normal — bot-shield
	// never blocks traffic just because fingerprinting didn't run.
	p.Director = func(r *http.Request) {
		director(r)
		if ja4 := JA4FromContext(r.Context()); ja4 != "" {
			r.Header.Set(ja4Header, ja4)
		}
	}
	return p, nil
}
