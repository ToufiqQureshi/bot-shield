package signals

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"
)

// GoodBotFamily defines an allowed search engine crawler family and its valid PTR domains.
type GoodBotFamily struct {
	UAPattern string
	Domains   []string
}

var knownGoodBots = []GoodBotFamily{
	{
		UAPattern: "googlebot",
		Domains:   []string{".googlebot.com", ".google.com"},
	},
	{
		UAPattern: "bingbot",
		Domains:   []string{".search.msn.com", ".bing.com"},
	},
	{
		UAPattern: "applebot",
		Domains:   []string{".applebot.apple.com", ".apple.com"},
	},
	{
		UAPattern: "duckduckbot",
		Domains:   []string{".duckduckgo.com"},
	},
	{
		UAPattern: "yandexbot",
		Domains:   []string{".yandex.ru", ".yandex.net", ".yandex.com"},
	},
	{
		UAPattern: "baiduspider",
		Domains:   []string{".baidu.com", ".baidu.jp"},
	},
}

type botCacheEntry struct {
	verified  bool
	expiresAt time.Time
}

var (
	botCacheMu sync.RWMutex
	botCache   = make(map[string]botCacheEntry)
)

const botCacheTTL = 6 * time.Hour

// DNSLookupFuncs allow dependency injection for deterministic testing without external network dependencies.
var (
	lookupAddrFunc = func(ctx context.Context, ip string) ([]string, error) {
		var r net.Resolver
		return r.LookupAddr(ctx, ip)
	}
	lookupIPFunc = func(ctx context.Context, host string) ([]net.IP, error) {
		var r net.Resolver
		return r.LookupIP(ctx, "ip", host)
	}
)

// IsGoodBotClaim checks if the User-Agent claims to be a major search engine crawler.
func IsGoodBotClaim(ua string) (isClaim bool, domains []string) {
	lowerUA := strings.ToLower(ua)
	for _, bot := range knownGoodBots {
		if strings.Contains(lowerUA, bot.UAPattern) {
			return true, bot.Domains
		}
	}
	return false, nil
}

// IsVerifiedGoodBot verifies whether a client IP claiming to be a search engine
// is genuinely operated by that search engine via reverse DNS + forward DNS verification.
// Results are cached for 6 hours to avoid DNS latency on repeated requests.
func IsVerifiedGoodBot(ip, ua string) bool {
	if ip == "" || ua == "" {
		return false
	}

	isClaim, validDomains := IsGoodBotClaim(ua)
	if !isClaim {
		return false
	}

	cacheKey := ip + "|" + ua
	botCacheMu.RLock()
	entry, found := botCache[cacheKey]
	botCacheMu.RUnlock()

	if found && time.Now().Before(entry.expiresAt) {
		return entry.verified
	}

	verified := verifyDNS(ip, validDomains)

	botCacheMu.Lock()
	botCache[cacheKey] = botCacheEntry{
		verified:  verified,
		expiresAt: time.Now().Add(botCacheTTL),
	}
	botCacheMu.Unlock()

	return verified
}

func verifyDNS(ipStr string, validDomains []string) bool {
	parsedIP := net.ParseIP(ipStr)
	if parsedIP == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	names, err := lookupAddrFunc(ctx, ipStr)
	if err != nil || len(names) == 0 {
		return false
	}

	for _, name := range names {
		// Clean trailing dot in FQDN
		cleanName := strings.TrimSuffix(strings.ToLower(name), ".")
		
		matchedDomain := false
		for _, domain := range validDomains {
			cleanDomain := strings.TrimPrefix(domain, ".")
			if strings.HasSuffix(cleanName, cleanDomain) {
				matchedDomain = true
				break
			}
		}

		if !matchedDomain {
			continue
		}

		// Forward DNS verification
		resolvedIPs, err := lookupIPFunc(ctx, cleanName)
		if err != nil {
			continue
		}

		for _, rip := range resolvedIPs {
			if rip.Equal(parsedIP) {
				return true
			}
		}
	}

	return false
}
