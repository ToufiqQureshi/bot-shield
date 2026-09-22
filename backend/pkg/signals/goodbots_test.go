package signals

import (
	"context"
	"errors"
	"net"
	"testing"
)

func TestIsGoodBotClaim(t *testing.T) {
	tests := []struct {
		ua      string
		isClaim bool
	}{
		{"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", true},
		{"Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)", true},
		{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15 (Applebot/0.1; +http://www.apple.com/bot.html)", true},
		{"Mozilla/5.0 (compatible; DuckDuckBot-Https/1.1; https://duckduckgo.com/duckduckbot)", true},
		{"Mozilla/5.0 (compatible; YandexBot/3.0; +http://yandex.com/bots)", true},
		{"Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)", true},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", false},
		{"curl/7.68.0", false},
	}

	for _, tt := range tests {
		claim, _ := IsGoodBotClaim(tt.ua)
		if claim != tt.isClaim {
			t.Errorf("IsGoodBotClaim(%q) = %v, want %v", tt.ua, claim, tt.isClaim)
		}
	}
}

func TestIsVerifiedGoodBot_Success(t *testing.T) {
	// Mock lookup funcs
	origLookupAddr := lookupAddrFunc
	origLookupIP := lookupIPFunc
	defer func() {
		lookupAddrFunc = origLookupAddr
		lookupIPFunc = origLookupIP
	}()

	testIP := "66.249.66.1"
	testHost := "crawl-66-249-66-1.googlebot.com"

	lookupAddrFunc = func(ctx context.Context, ip string) ([]string, error) {
		if ip == testIP {
			return []string{testHost}, nil
		}
		return nil, errors.New("not found")
	}

	lookupIPFunc = func(ctx context.Context, host string) ([]net.IP, error) {
		if host == testHost {
			return []net.IP{net.ParseIP(testIP)}, nil
		}
		return nil, errors.New("not found")
	}

	ua := "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
	if !IsVerifiedGoodBot(testIP, ua) {
		t.Fatalf("expected genuine Googlebot to be verified")
	}

	// Test cache hit
	lookupAddrFunc = func(ctx context.Context, ip string) ([]string, error) {
		t.Fatalf("lookupAddr called despite cached entry")
		return nil, nil
	}
	if !IsVerifiedGoodBot(testIP, ua) {
		t.Fatalf("expected cached Googlebot verification to succeed")
	}
}

func TestIsVerifiedGoodBot_Spoofed(t *testing.T) {
	origLookupAddr := lookupAddrFunc
	origLookupIP := lookupIPFunc
	defer func() {
		lookupAddrFunc = origLookupAddr
		lookupIPFunc = origLookupIP
	}()

	spoofedIP := "198.51.100.42"
	lookupAddrFunc = func(ctx context.Context, ip string) ([]string, error) {
		// Reverse DNS returns attacker host
		return []string{"attacker.example.com"}, nil
	}
	lookupIPFunc = func(ctx context.Context, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP(spoofedIP)}, nil
	}

	ua := "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
	if IsVerifiedGoodBot(spoofedIP, ua) {
		t.Fatalf("expected spoofed Googlebot to be rejected")
	}
}

func TestVerifyDNSRejectsLookalikeDomain(t *testing.T) {
	origLookupAddr := lookupAddrFunc
	origLookupIP := lookupIPFunc
	defer func() {
		lookupAddrFunc = origLookupAddr
		lookupIPFunc = origLookupIP
	}()

	const spoofedIP = "198.51.100.43"
	lookupAddrFunc = func(ctx context.Context, ip string) ([]string, error) {
		return []string{"evilgooglebot.com."}, nil
	}
	lookupIPFunc = func(ctx context.Context, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP(spoofedIP)}, nil
	}

	if verifyDNS(spoofedIP, []string{".googlebot.com"}) {
		t.Fatal("lookalike PTR domain must not pass good-bot verification")
	}
}

func TestVerifyDNSDoesNotQueueWhenLookupBudgetIsFull(t *testing.T) {
	for i := 0; i < cap(botLookupSlots); i++ {
		botLookupSlots <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(botLookupSlots); i++ {
			<-botLookupSlots
		}
	}()

	called := false
	origLookupAddr := lookupAddrFunc
	lookupAddrFunc = func(context.Context, string) ([]string, error) {
		called = true
		return nil, errors.New("lookup must not run")
	}
	defer func() { lookupAddrFunc = origLookupAddr }()

	if verifyDNS("198.51.100.44", []string{".googlebot.com"}) {
		t.Fatal("a lookup rejected by the concurrency budget must not verify")
	}
	if called {
		t.Fatal("DNS lookup ran while the bounded lookup budget was full")
	}
}

func TestHostnameMatchesDomainRequiresBoundary(t *testing.T) {
	cases := []struct {
		host, domain string
		want         bool
	}{
		{"crawl.googlebot.com", ".googlebot.com", true},
		{"googlebot.com", ".googlebot.com", true},
		{"evilgooglebot.com", ".googlebot.com", false},
		{"googlebot.com.attacker.example", ".googlebot.com", false},
	}
	for _, tc := range cases {
		if got := hostnameMatchesDomain(tc.host, tc.domain); got != tc.want {
			t.Errorf("hostnameMatchesDomain(%q, %q) = %v, want %v", tc.host, tc.domain, got, tc.want)
		}
	}
}
