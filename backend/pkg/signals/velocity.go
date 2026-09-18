package signals

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	rdb            *redis.Client
	rateLimitMs    = 1000 // Time window in milliseconds
	maxRequests    = 5    // Max requests per IP per window before it's considered a spike
	maxJA4Requests = 50   // Max requests across all IPs for a single non-browser JA4 per window
)

// InitRedis sets the package-level Redis client used for distributed rate limiting.
func InitRedis(client *redis.Client) {
	rdb = client
}

// checkVelocitySpike returns true if the IP has made more than maxRequests in the last rateLimitMs.
// It uses a simple fixed-window counter in Redis to allow horizontal scaling.
func checkVelocitySpike(ip string) bool {
	if ip == "" || rdb == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Fixed window approach: key changes every rateLimitMs
	window := time.Now().UnixMilli() / int64(rateLimitMs)
	key := fmt.Sprintf("vel:ip:%s:%d", ip, window)

	// Increment and set expiry in a single pipeline
	pipe := rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, time.Duration(rateLimitMs*2)*time.Millisecond)

	_, err := pipe.Exec(ctx)
	if err != nil {
		// Fail open on Redis errors to prevent blocking legit traffic if Redis is down
		return false
	}

	return incr.Val() > int64(maxRequests)
}

// checkJA4VelocitySpike returns true if a single non-standard JA4 fingerprint exceeds maxJA4Requests
// across all IPs within the current rateLimitMs window. This neutralizes residential proxy networks
// where bots rotate IP on every request but keep the same underlying scraper client TLS profile.
func checkJA4VelocitySpike(ja4 string) bool {
	if ja4 == "" || ja4 == JA4Unreadable || rdb == nil {
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
		return false
	}

	return incr.Val() > int64(maxJA4Requests)
}
