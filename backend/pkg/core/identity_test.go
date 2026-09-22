package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCanonicalIP(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"ipv4 with port", "203.0.113.7:443", "203.0.113.7"},
		{"ipv6 with port", "[2001:db8::7]:443", "2001:db8::7"},
		{"bare ipv6", "2001:db8::7", "2001:db8::7"},
		{"invalid", "not-an-ip:443", ""},
	}
	for _, tc := range cases {
		if got := canonicalIP(tc.raw); got != tc.want {
			t.Errorf("canonicalIP(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestRequestIdentityCanonicalization(t *testing.T) {
	r := httptest.NewRequest("GET", "http://EXAMPLE.COM:8443/", nil)
	r.Host = "EXAMPLE.COM:8443"
	r.RemoteAddr = "[2001:db8::10]:1234"

	if got := requestClientIP(r); got != "2001:db8::10" {
		t.Fatalf("requestClientIP() = %q, want IPv6 address", got)
	}
	if got := requestHost(r); got != "example.com" {
		t.Fatalf("requestHost() = %q, want example.com", got)
	}
}

func TestRequestHostIPv6Literal(t *testing.T) {
	r := httptest.NewRequest("GET", "http://[2001:db8::1]/", nil)
	r.Host = "[2001:db8::1]"
	if got := requestHost(r); got != "2001:db8::1" {
		t.Fatalf("requestHost() = %q, want IPv6 literal", got)
	}
}

func TestClientIPResolverTrustsForwardedAddressOnlyFromConfiguredProxy(t *testing.T) {
	resolver, err := NewClientIPResolver([]string{"192.0.2.0/24", "2001:db8:feed::/48"})
	if err != nil {
		t.Fatalf("NewClientIPResolver: %v", err)
	}

	cases := []struct {
		name       string
		remoteAddr string
		forwarded  string
		want       string
	}{
		{"direct visitor ignores forged header", "198.51.100.8:443", "203.0.113.9", "198.51.100.8"},
		{"trusted proxy accepts client", "192.0.2.10:443", "203.0.113.9", "203.0.113.9"},
		{"trusted proxy chain skips proxy hops", "192.0.2.10:443", "203.0.113.9, 192.0.2.20", "203.0.113.9"},
		{"trusted ipv6 proxy accepts ipv6 client", "[2001:db8:feed::10]:443", "2001:db8:1::9", "2001:db8:1::9"},
		{"malformed forwarding falls back to direct proxy", "192.0.2.10:443", "203.0.113.9, not-an-ip", "192.0.2.10"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
			r.RemoteAddr = tc.remoteAddr
			r.Header.Set("X-Forwarded-For", tc.forwarded)
			if got := resolver.ClientIP(r); got != tc.want {
				t.Fatalf("ClientIP() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClientIPResolverRejectsInvalidCIDR(t *testing.T) {
	if _, err := NewClientIPResolver([]string{"not-a-cidr"}); err == nil {
		t.Fatal("invalid trusted proxy CIDR must be rejected")
	}
}
