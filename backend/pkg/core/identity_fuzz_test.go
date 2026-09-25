package core

import (
	"net"
	"net/http/httptest"
	"strings"
	"testing"
)

// FuzzClientIP feeds hostile X-Forwarded-For values through a trusted
// proxy. Whatever the header says, the resolved address must be a real IP
// and either the proxy itself or an untrusted hop the header actually
// named; a visitor must never be able to make us report garbage or a
// trusted proxy address as the client.
func FuzzClientIP(f *testing.F) {
	for _, seed := range []string{
		"203.0.113.9",
		"203.0.113.9, 10.0.0.2",
		"198.51.100.1, 203.0.113.9, 10.0.0.2",
		"10.0.0.2, 10.0.0.3",
		"",
		",,,",
		"not-an-ip",
		"2001:db8::1, 10.0.0.2",
		"203.0.113.9:8080",
		strings.Repeat("1.1.1.1,", 50),
	} {
		f.Add(seed)
	}
	resolver, err := NewClientIPResolver([]string{"10.0.0.0/8"})
	if err != nil {
		f.Fatal(err)
	}

	f.Fuzz(func(t *testing.T, xff string) {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "10.0.0.1:4321"
		req.Header.Set("X-Forwarded-For", xff)

		got := resolver.ClientIP(req)
		ip := net.ParseIP(got)
		if ip == nil {
			t.Fatalf("ClientIP(%q) = %q, not an IP", xff, got)
		}
		if got == "10.0.0.1" {
			return
		}
		if resolver.isTrusted(ip) {
			t.Fatalf("ClientIP(%q) = %q, a trusted proxy address", xff, got)
		}
		// A visitor can only prepend to the header; our proxies append.
		// So the answer must be the rightmost untrusted hop, never one
		// further left that the visitor wrote themselves.
		hops := strings.Split(xff, ",")
		for i := len(hops) - 1; i >= 0; i-- {
			hop := net.ParseIP(strings.TrimSpace(hops[i]))
			if hop == nil || resolver.isTrusted(hop) {
				continue
			}
			if !ip.Equal(hop) {
				t.Fatalf("ClientIP(%q) = %q, want rightmost untrusted hop %q", xff, got, hop)
			}
			return
		}
		t.Fatalf("ClientIP(%q) = %q, which the header never named", xff, got)
	})
}
