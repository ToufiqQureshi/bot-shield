package signals

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ja4Mu           sync.RWMutex
	scraperJA4s     map[string]string // Map of JA4 -> Tool Name
	browserPrefixes []string
)

func init() {
	scraperJA4s = make(map[string]string)
	browserPrefixes = make([]string, 0)
}

// StartJA4Sync initializes the JA4 database from Redis and starts a background
// goroutine to keep the local in-memory cache synchronized, ensuring zero-latency
// lookups during actual request processing.
func StartJA4Sync(ctx context.Context, rdb *redis.Client) {
	if rdb == nil {
		log.Println("botshield: warning: StartJA4Sync called with nil Redis client. JA4 DB will remain empty/default.")
		return
	}

	// Initial load
	syncJA4FromRedis(ctx, rdb)

	// Background polling to keep nodes in sync
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				syncJA4FromRedis(ctx, rdb)
			}
		}
	}()
}

func syncJA4FromRedis(ctx context.Context, rdb *redis.Client) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	scrapers, err := rdb.HGetAll(timeoutCtx, "ja4:scrapers").Result()
	if err != nil && err != redis.Nil {
		log.Printf("botshield: error syncing ja4 scrapers from redis: %v", err)
		return
	}

	browsers, err := rdb.SMembers(timeoutCtx, "ja4:browsers").Result()
	if err != nil && err != redis.Nil {
		log.Printf("botshield: error syncing ja4 browsers from redis: %v", err)
		return
	}

	ja4Mu.Lock()
	defer ja4Mu.Unlock()

	// Only overwrite if we didn't error (to survive transient Redis failures)
	if scrapers != nil {
		scraperJA4s = scrapers
	}
	if browsers != nil {
		browserPrefixes = browsers
	}
}

// AddKnownScraperJA4 adds a scraper to the local cache (primarily for testing or manual overrides)
func AddKnownScraperJA4(ja4, tool string) {
	ja4Mu.Lock()
	defer ja4Mu.Unlock()
	scraperJA4s[ja4] = tool
}

// AddCommonBrowserPrefix adds a browser prefix to the local cache
func AddCommonBrowserPrefix(prefix string) {
	ja4Mu.Lock()
	defer ja4Mu.Unlock()
	browserPrefixes = append(browserPrefixes, prefix)
}

// isCommonBrowserJA4 checks if a JA4 signature matches genuine modern browser profiles.
// Used to exempt legitimate browser traffic from aggressive cross-IP aggregate rate limits.
func isCommonBrowserJA4(ja4 string) bool {
	if ja4 == "" || ja4 == JA4Unreadable {
		return false
	}
	ja4Mu.RLock()
	defer ja4Mu.RUnlock()
	for _, p := range browserPrefixes {
		if strings.HasPrefix(ja4, p) {
			return true
		}
	}
	return false
}

// IsKnownScraperJA4 returns whether the given fingerprint matches a known scraper or automation library.
func IsKnownScraperJA4(ja4 string) (bool, string) {
	if ja4 == "" || ja4 == JA4Unreadable {
		return false, ""
	}
	ja4Mu.RLock()
	defer ja4Mu.RUnlock()
	tool, ok := scraperJA4s[ja4]
	return ok, tool
}
