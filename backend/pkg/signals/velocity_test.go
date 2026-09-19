package signals

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newTestRedis points the package-level rdb at an in-memory miniredis
// instance for the duration of the test, then restores whatever was
// there before — checkVelocitySpike/checkJA4VelocitySpike read the
// package var directly, so tests can't inject a client any other way.
func newTestRedis(t *testing.T) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)

	prev := rdb
	rdb = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb = prev })
}

func TestCheckVelocitySpikeNoRedisFailsOpen(t *testing.T) {
	prev := rdb
	rdb = nil
	defer func() { rdb = prev }()

	if checkVelocitySpike("1.2.3.4") {
		t.Fatal("checkVelocitySpike with nil rdb must fail open (false)")
	}
}

func TestCheckVelocitySpikeUnderLimit(t *testing.T) {
	newTestRedis(t)
	for i := 0; i < maxRequests; i++ {
		if checkVelocitySpike("1.2.3.4") {
			t.Fatalf("request %d: spiked before exceeding maxRequests=%d", i, maxRequests)
		}
	}
}

func TestCheckVelocitySpikeOverLimit(t *testing.T) {
	newTestRedis(t)
	var lastSpiked bool
	for i := 0; i < maxRequests+1; i++ {
		lastSpiked = checkVelocitySpike("5.6.7.8")
	}
	if !lastSpiked {
		t.Fatalf("request %d: want spike after exceeding maxRequests=%d", maxRequests+1, maxRequests)
	}
}

func TestCheckVelocitySpikeIsolatedPerIP(t *testing.T) {
	newTestRedis(t)
	for i := 0; i < maxRequests+1; i++ {
		checkVelocitySpike("9.9.9.9")
	}
	// A different IP must not inherit the first IP's count.
	if checkVelocitySpike("1.1.1.1") {
		t.Fatal("a fresh IP must not be flagged by another IP's velocity")
	}
}

func TestCheckJA4VelocitySpikeExemptsCommonBrowsers(t *testing.T) {
	newTestRedis(t)
	AddCommonBrowserPrefix("t13d1516h2")
	for i := 0; i < maxJA4Requests+1; i++ {
		if checkJA4VelocitySpike("t13d1516h2_8daaf6152771_e5627efa2ab1") {
			t.Fatal("a common browser JA4 prefix must never be flagged by aggregate velocity")
		}
	}
}

func TestCheckJA4VelocitySpikeOverLimit(t *testing.T) {
	newTestRedis(t)
	var lastSpiked bool
	for i := 0; i < maxJA4Requests+1; i++ {
		lastSpiked = checkJA4VelocitySpike("t99d000000_deadbeefdead_deadbeefdead")
	}
	if !lastSpiked {
		t.Fatal("want spike after exceeding maxJA4Requests for a non-browser JA4")
	}
}

func TestVelocityExceededCombinesBothChecks(t *testing.T) {
	newTestRedis(t)
	for i := 0; i < maxRequests+1; i++ {
		VelocityExceeded("2.2.2.2", "")
	}
	if !VelocityExceeded("2.2.2.2", "") {
		t.Fatal("VelocityExceeded must reflect an IP-only spike")
	}
	if VelocityExceeded("3.3.3.3", "") {
		t.Fatal("VelocityExceeded must not flag an unrelated, low-volume IP")
	}
}
