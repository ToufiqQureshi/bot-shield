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
func VelocityExceeded(ip, ja4, path string) bool {
	return checkVelocitySpike(ip, path) || checkJA4VelocitySpike(ja4)
}

// checkVelocitySpike returns true if the IP has exceeded the rate limit
// for its request class in the current window. Navigations and assets
// have separate counters and limits, so a browser loading a page's
// subresources is never mistaken for a crawler hitting many pages.
func checkVelocitySpike(ip, path string) bool {
	if ip == "" || !redisRequestAllowed() {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	window := time.Now().UnixMilli() / int64(rateLimitMs)
	key, limit := velocityBucket(ip, path, window)

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

// velocityBucket picks the counter key and its limit for a request.
func velocityBucket(ip, path string, window int64) (string, int64) {
	if isStaticAsset(path) {
		return fmt.Sprintf("vel:ip:%s:asset:%d", ip, window), maxAssetPerWindow
	}
	return fmt.Sprintf("vel:ip:%s:nav:%d", ip, window), maxNavPerWindow
}

// checkJA4VelocitySpike returns true if a single non-standard JA4 fingerprint exceeds maxJA4Requests
// across all IPs within the current rateLimitMs window. This neutralises residential proxy networks
// where bots rotate IP on every request but keep the same underlying scraper client TLS profile.
func checkJA4VelocitySpike(ja4 string) bool {
	if ja4 == "" || ja4 == JA4Unreadable || !redisRequestAllowed() {
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
	key := fmt.Sprintf("vel:ja4:%s:%d", ja4, window)

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
