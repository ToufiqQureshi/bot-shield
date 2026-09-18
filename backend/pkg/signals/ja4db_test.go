package signals

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestJA4DynamicDatabase(t *testing.T) {
	// Reset the DB for this test
	ja4Mu.Lock()
	scraperJA4s = make(map[string]string)
	browserPrefixes = make([]string, 0)
	ja4Mu.Unlock()

	AddKnownScraperJA4("t12d190800_4464c1bd5eb7_b3394627b738", "python-requests")
	AddCommonBrowserPrefix("t13d1516h2_")

	// Python requests verified capture
	isScraper, tool := IsKnownScraperJA4("t12d190800_4464c1bd5eb7_b3394627b738")
	if !isScraper || tool != "python-requests" {
		t.Errorf("expected python-requests, got (%v, %s)", isScraper, tool)
	}

	// Real Chrome JA4
	if !isCommonBrowserJA4("t13d1516h2_8daaf6152771_e5627efa2ab1") {
		t.Errorf("expected t13d1516h2_... to be a common browser JA4")
	}

	// Concurrency and race testing
	// We read and write simultaneously to ensure no data races.
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			IsKnownScraperJA4("t12d190800_4464c1bd5eb7_b3394627b738")
			isCommonBrowserJA4("t13d1516h2_8daaf6152771_e5627efa2ab1")

			if i%10 == 0 {
				AddKnownScraperJA4("fake_bot", "fake")
			}
		}(i)
	}
	wg.Wait()
}

func TestJA4SyncFromRedisFailures(t *testing.T) {
	// Ensure that syncing with a nil redis client doesn't panic and fails open/gracefully
	StartJA4Sync(context.Background(), nil)

	// Ensure syncing with a bad redis client fails gracefully without clearing the list
	opt, _ := redis.ParseURL("redis://invalid-host:1234")
	rdb := redis.NewClient(opt)

	AddKnownScraperJA4("t12_survive", "survivor")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	syncJA4FromRedis(ctx, rdb)

	// Should survive
	isScraper, tool := IsKnownScraperJA4("t12_survive")
	if !isScraper || tool != "survivor" {
		t.Errorf("expected existing records to survive a failed Redis sync, got %v", isScraper)
	}
}
