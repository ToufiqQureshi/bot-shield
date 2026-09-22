package core

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// ClientIPResolver resolves a visitor address only through explicitly trusted
// reverse proxies. Its zero value keeps the secure default: use the direct
// peer and ignore every visitor-controlled forwarding header.
type ClientIPResolver struct {
	trusted []*net.IPNet
}

// NewClientIPResolver parses the operator-controlled CIDRs allowed to supply
// X-Forwarded-For. A trusted proxy must overwrite or safely append that header;
// no application can recover a genuine client IP from a proxy that forwards an
// attacker-provided value unchanged.
func NewClientIPResolver(cidrs []string) (*ClientIPResolver, error) {
	resolver := &ClientIPResolver{}
	for _, raw := range cidrs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		_, network, err := net.ParseCIDR(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy CIDR %q: %w", raw, err)
		}
		resolver.trusted = append(resolver.trusted, network)
	}
	return resolver, nil
}

// ClientIP returns the direct peer unless it is in the configured trusted
// proxy set. For a trusted peer, it walks X-Forwarded-For from the closest hop
// backwards and returns the first untrusted address. This supports chains of
// trusted load balancers without trusting a header from a direct visitor.
func (r *ClientIPResolver) ClientIP(req *http.Request) string {
	if req == nil {
		return ""
	}
	peer := canonicalIP(req.RemoteAddr)
	if peer == "" || r == nil || !r.isTrusted(net.ParseIP(peer)) {
		return peer
	}

	forwarded, ok := forwardedIPs(req.Header.Values("X-Forwarded-For"))
	if !ok {
		return peer
	}
	for i := len(forwarded) - 1; i >= 0; i-- {
		if !r.isTrusted(forwarded[i]) {
			return forwarded[i].String()
		}
	}
	return peer
}

func (r *ClientIPResolver) isTrusted(ip net.IP) bool {
	if ip == nil {
		return false
	}
	for _, network := range r.trusted {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func forwardedIPs(values []string) ([]net.IP, bool) {
	var ips []net.IP
	for _, value := range values {
		for _, raw := range strings.Split(value, ",") {
			ip := net.ParseIP(strings.TrimSpace(raw))
			if ip == nil {
				return nil, false
			}
			ips = append(ips, ip)
		}
	}
	return ips, len(ips) > 0
}

// requestClientIP extracts and canonicalizes the direct peer address. Forwarded
// headers are intentionally ignored here: callers that have an explicit,
// operator-configured trusted-proxy boundary must use ClientIPResolver.
func requestClientIP(r *http.Request) string {
	return (&ClientIPResolver{}).ClientIP(r)
}

func canonicalIP(raw string) string {
	raw = strings.TrimSpace(raw)
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	} else {
		raw = strings.Trim(raw, "[]")
	}

	ip := net.ParseIP(raw)
	if ip == nil {
		return ""
	}
	return ip.String()
}

// requestHost canonicalizes the HTTP Host value for tenant lookup. Hostnames
// are case-insensitive and an optional port is not part of tenant identity.
func requestHost(r *http.Request) string {
	if r == nil {
		return ""
	}
	host := strings.TrimSpace(r.Host)
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	} else {
		host = strings.Trim(host, "[]")
	}
	return strings.TrimSuffix(strings.ToLower(host), ".")
}
