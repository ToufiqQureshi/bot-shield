package signals

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

// HoneypotPath is the hidden endpoint injected into challenge pages and HTML responses.
// Human visitors cannot see or click it, but headless scrapers and DOM walkers
// interact with it, immediately revealing their automation presence.
const HoneypotPath = "/__botshield/honeypot_trap"

// RecordHoneypotTrigger registers a honeypot trap activation.
// It instantly adds the offender's JA4 fingerprint to local scraper memory
// and syncs it across all nodes via Redis.
func RecordHoneypotTrigger(ctx context.Context, ja4, ip string, rdb *redis.Client) {
	if ja4 != "" && ja4 != JA4Unreadable {
		AddKnownScraperJA4(ja4, "honeypot_trap")
		log.Printf("botshield: honeypot triggered by IP %s (JA4: %s). Fingerprint added to blocklist.", ip, ja4)

		if rdb != nil {
			go func() {
				// Persist to Redis shared scraper hash
				if err := rdb.HSet(ctx, "ja4:scrapers", ja4, "honeypot_trap").Err(); err != nil {
					log.Printf("botshield: error persisting honeypot JA4 to redis: %v", err)
				}
				// Publish event to swarm channel for instant multi-node sync
				if err := rdb.Publish(ctx, "ja4:updates", ja4).Err(); err != nil {
					log.Printf("botshield: error publishing honeypot JA4 update to redis pubsub: %v", err)
				}
			}()
		}
	}
}
