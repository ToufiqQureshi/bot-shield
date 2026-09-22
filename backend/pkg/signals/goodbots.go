package signals

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/observability"
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
const maxBotCacheEntries = 100_000

// DNS verification is optional allowlist evidence and runs on the request
// path. A non-blocking semaphore puts a hard ceiling on simultaneous resolver
// work; under a spoofed-bot burst, excess claims are treated as unverified
// instead of queueing request handlers behind DNS.
const maxConcurrentBotLookups = 64

var botLookupSlots = make(chan struct{}, maxConcurrentBotLookups)

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

	// Cache by the claimed bot family rather than the complete User-Agent.
	// A caller can vary arbitrary UA suffixes; including them would let an
	// attacker turn a single source IP into unbounded cache keys.
	cacheKey := ip + "|" + strings.Join(validDomains, ",")
	botCacheMu.RLock()
	entry, found := botCache[cacheKey]
	botCacheMu.RUnlock()

	if found && time.Now().Before(entry.expiresAt) {
		return entry.verified
	}

	verified := verifyDNS(ip, validDomains)

	cacheBotResult(cacheKey, verified)

	return verified
}

// cacheBotResult stores a verification result without allowing visitor-
// controlled identities to grow the process heap forever. Expired entries are
// reclaimed when the cap is reached; if all entries are still live, a new
// result is simply not cached and the current request still gets its answer.
func cacheBotResult(key string, verified bool) {
	now := time.Now()
	botCacheMu.Lock()
	defer botCacheMu.Unlock()

	if len(botCache) >= maxBotCacheEntries {
		for existingKey, entry := range botCache {
			if !now.Before(entry.expiresAt) {
				delete(botCache, existingKey)
				break
			}
		}
		if len(botCache) >= maxBotCacheEntries {
			return
		}
	}
	botCache[key] = botCacheEntry{verified: verified, expiresAt: now.Add(botCacheTTL)}
}

func verifyDNS(ipStr string, validDomains []string) bool {
	parsedIP := net.ParseIP(ipStr)
	if parsedIP == nil {
		return false
	}
	select {
	case botLookupSlots <- struct{}{}:
		defer func() { <-botLookupSlots }()
	default:
		observability.Inc("goodbot_lookup_budget_reject_total")
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
			if hostnameMatchesDomain(cleanName, domain) {
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

// hostnameMatchesDomain requires either an exact hostname or a dot boundary.
// A plain suffix check would accept attacker-controlled names such as
// "evilgooglebot.com" for the allowed domain "googlebot.com".
func hostnameMatchesDomain(hostname, domain string) bool {
	hostname = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(hostname)), ".")
	domain = strings.Trim(strings.ToLower(strings.TrimSpace(domain)), ".")
	if hostname == "" || domain == "" {
		return false
	}
	return hostname == domain || strings.HasSuffix(hostname, "."+domain)
}
