package signals

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	rdb *redis.Client
	// redisHealth is shared by every request-path Redis signal. One failed
	// Redis call opens it briefly, so an outage cannot turn into several
	// timeout-bound calls for every proxied request.
	redisHealth redisCircuit
	// rateLimitMs is the fixed-window size for the per-IP counters.
	rateLimitMs = 1000
)

const (
	// maxNavPerWindow caps *navigation* requests per IP per window. A
	// person cannot open 20 pages in one second; a crawler easily does.
	maxNavPerWindow = 20
	// maxAssetPerWindow is a far looser cap for subresources, because one
	// real page load is dozens of them and blocking those would break
	// every real visitor (CLAUDE.md Section 14).
	maxAssetPerWindow = 300
	// maxLoginPerWindow and the other endpoint caps make the plan's
	// endpoint-aware velocity (P2 track 4) concrete: credential stuffing
	// is a login-shaped flood, scraping is a browse-shaped flood, and a
	// person does none of them fast.
	maxLoginPerWindow = 10
	// maxAPIPerWindow stays loose: dashboards and mobile apps legitimately
	// poll APIs hard, and API false positives take down integrations.
	maxAPIPerWindow = 100
	// maxCheckoutPerWindow sits below nav because a checkout flow is a
	// handful of requests, but above login because carts and payment
	// steps are normal to repeat.
	maxCheckoutPerWindow = 20
	// maxJA4Requests caps one non-browser fingerprint across all IPs, to
	// neutralise residential-proxy rotation.
	maxJA4Requests = 50
)

// InitRedis sets the package-level Redis client used for distributed rate limiting.
func InitRedis(client *redis.Client) {
	rdb = client
	// A new client is a new health boundary. Do not carry an outage state from
	// a previous client into startup or a controlled client replacement.
	redisHealth.reset()
}

// VelocityExceeded reports whether ip or ja4 has tripped its rate
// limit in the current window. Exported so a visitor who already
// passed the JS challenge can still be rate-limited on later requests
// (guard.go) — a solved challenge proves the client can run JS once,
// not that every request after it is legitimate at any volume.
func VelocityExceeded(tenant, ip, ja4, path, method string) bool {
	return checkVelocitySpike(tenant, ip, path, method) || checkJA4VelocitySpike(tenant, ja4)
}

// checkVelocitySpike returns true if the IP has exceeded the rate limit
// for its endpoint class in the current window. Logins, API calls,
// checkout steps, page navigations and assets each get their own counter
// and limit, so a credential-stuffing run is visible against its own
// bar while a browser clicking around and loading subresources is never
// mistaken for any of it.
func checkVelocitySpike(tenant, ip, path, method string) bool {
	if tenant == "" || ip == "" || !redisRequestAllowed() {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	window := time.Now().UnixMilli() / int64(rateLimitMs)
	key, limit := velocityBucket(tenant, ip, path, method, window)

	pipe := rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, time.Duration(rateLimitMs*2)*time.Millisecond)
	if _, err := pipe.Exec(ctx); err != nil {
		// Fail open on Redis errors: a Redis outage must never block
		// legitimate traffic.
		redisHealth.failure(time.Now())
		return false
	}
	redisHealth.success()
	return incr.Val() > limit
}

// velocityBucket picks the counter key and its limit for a request from
// its endpoint class. The class comes from the normalized path and the
// method; an unparseable path still counts, as a navigation.
func velocityBucket(tenant, ip, path, method string, window int64) (string, int64) {
	bucket := "nav"
	var limit int64 = maxNavPerWindow
	if normalized, ok := NormalizePath(path); ok {
		switch Classify(normalized, method) {
		case ClassStatic:
			bucket, limit = "asset", maxAssetPerWindow
		case ClassLogin:
			bucket, limit = "login", maxLoginPerWindow
		case ClassAPI:
			bucket, limit = "api", maxAPIPerWindow
		case ClassCheckout:
			bucket, limit = "checkout", maxCheckoutPerWindow
		}
	}
	return fmt.Sprintf("vel:t:%d:%s:ip:%s:%s:%d", len(tenant), tenant, ip, bucket, window), limit
}

// checkJA4VelocitySpike returns true if a single non-standard JA4 fingerprint exceeds maxJA4Requests
// across all IPs within the current rateLimitMs window. This neutralises residential proxy networks
// where bots rotate IP on every request but keep the same underlying scraper client TLS profile.
func checkJA4VelocitySpike(tenant, ja4 string) bool {
	if tenant == "" || ja4 == "" || ja4 == JA4Unreadable || !redisRequestAllowed() {
		return false
	}
	if !hasCommonBrowserPrefixes() {
		return false
	}
	// Common desktop/mobile browsers are exempt from raw aggregate count to protect genuine traffic
	if isCommonBrowserJA4(ja4) {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	window := time.Now().UnixMilli() / int64(rateLimitMs)
	key := fmt.Sprintf("vel:t:%d:%s:ja4:%s:%d", len(tenant), tenant, ja4, window)

	pipe := rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, time.Duration(rateLimitMs*2)*time.Millisecond)

	_, err := pipe.Exec(ctx)
	if err != nil {
		redisHealth.failure(time.Now())
		return false
	}
	redisHealth.success()

	return incr.Val() > int64(maxJA4Requests)
}

// redisRequestAllowed returns false while Redis is unhealthy. Rate and crawl
// signals are optional evidence, so callers deliberately fail open instead of
// spending the request budget retrying a known-down dependency.
func redisRequestAllowed() bool {
	return rdb != nil && redisHealth.allow(time.Now())
}
