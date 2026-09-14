// Package proxy is the reverse proxy that sits in front of a
// client's origin server. It is the integration point every
// detection layer (fingerprint, score, challenge) plugs into.
package proxy

import (
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

	// Rewrite, not the older Director: net/http strips the client's
	// own X-Forwarded-* headers before calling it, so a visitor can't
	// fake the IP that our rate limiting and geo checks will rely on.
	// Director leaves those headers untouched.
	p := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(u)
			// SetURL would point Host at the origin; the origin serves
			// the visitor's domain, so it needs the original Host.
			r.Out.Host = r.In.Host
			// Fills in the real client IP for the origin's logs.
			r.SetXForwarded()

			// Only we get to say what the fingerprint is. A request
			// with none is forwarded as normal — a fingerprint we
			// couldn't read is never a reason to block someone.
			r.Out.Header.Del(ja4Header)
			if ja4 := JA4FromContext(r.In.Context()); ja4 != "" {
				r.Out.Header.Set(ja4Header, ja4)
			}
		},
	}
	return p, nil
}
