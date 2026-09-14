// Package proxy is the reverse proxy that sits in front of a
// client's origin server. It is the integration point every
// detection layer (fingerprint, score, challenge) plugs into.
package proxy

import (
	"net/http/httputil"
	"net/url"
)

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
	return httputil.NewSingleHostReverseProxy(u), nil
}
