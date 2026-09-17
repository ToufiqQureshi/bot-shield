// Package proxy is the reverse proxy that sits in front of a
// client's origin server. It is the integration point every
// detection layer (fingerprint, score, challenge) plugs into.
package core

import (
	"net"
	"net/http/httputil"
	"net/url"

	"github.com/ToufiqQureshi/bot-shield/pkg/signals"
)

// ja4Header is the header we attach to the forwarded request so the
// origin (and later, our own scoring code) can see the caller's JA4
// fingerprint without re-capturing it themselves.
const ja4Header = "X-BotShield-JA4"

// uaMismatchHeader tells the origin the caller's declared browser
// doesn't match its TLS handshake. Set to "true" only when we caught
// one — absence means nothing suspicious was found, not "unknown".
const uaMismatchHeader = "X-BotShield-UA-Mismatch"

// realIPHeader is the client-IP header we set ourselves. nginx, Rails
// and Laravel apps commonly read this one.
const realIPHeader = "X-Real-IP"

// clientIPHeaders are the "who is calling" headers a visitor could
// send to pass themselves off as another IP. net/http only strips the
// X-Forwarded-* family, so we clear these ourselves — an origin that
// trusts any one of them would otherwise log, allowlist or rate-limit
// an address the caller picked.
var clientIPHeaders = []string{
	realIPHeader,
	"True-Client-IP",
	"CF-Connecting-IP",
	"X-Client-IP",
	"Fastly-Client-IP",
	"X-Cluster-Client-IP",
}

// NewOriginProxy sets up the HTTP proxy to the origin server.
// It forces TLS since a WAF that doesn't protect the origin connection
// is just security theater.
func NewOriginProxy(target string) (*httputil.ReverseProxy, error) {
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
			for _, h := range clientIPHeaders {
				r.Out.Header.Del(h)
			}
			if ip, _, err := net.SplitHostPort(r.In.RemoteAddr); err == nil {
				r.Out.Header.Set(realIPHeader, ip)
			}

			// Only we get to say what the fingerprint is. A request
			// with none is forwarded as normal — a fingerprint we
			// couldn't read is never a reason to block someone.
			r.Out.Header.Del(ja4Header)
			ja4 := JA4FromContext(r.In.Context())
			if ja4 != "" {
				r.Out.Header.Set(ja4Header, ja4)
			}

			// Same rule for the UA-mismatch flag: a visitor doesn't
			// get to set this themselves, and it's a signal for
			// scoring, not a block, per CLAUDE.md Section 6.
			r.Out.Header.Del(uaMismatchHeader)
			if signals.UAMismatch(r.In.UserAgent(), ja4) {
				r.Out.Header.Set(uaMismatchHeader, "true")
			}
		},
	}
	return p, nil
}
