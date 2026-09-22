package signals

import (
	"sync"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/observability"
)

// redisCircuitOpenFor is deliberately short: Redis rate signals are helpful,
// but they must never add repeated timeout latency or take customer traffic
// down while Redis is unhealthy.
const redisCircuitOpenFor = time.Second

// redisCircuit stops an outage from multiplying request-path Redis attempts.
// After an error, callers fail open until the cooldown expires. Exactly one
// caller may then probe Redis; a success closes the circuit and another error
// starts the cooldown again.
type redisCircuit struct {
	mu            sync.Mutex
	openUntil     time.Time
	probeInFlight bool
}

func (c *redisCircuit) reset() {
	c.mu.Lock()
	c.openUntil = time.Time{}
	c.probeInFlight = false
	c.mu.Unlock()
}

func (c *redisCircuit) allow(now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.openUntil.IsZero() {
		return true
	}
	if now.Before(c.openUntil) || c.probeInFlight {
		observability.Inc("redis_circuit_skip_total")
		return false
	}
	c.probeInFlight = true
	observability.Inc("redis_circuit_probe_total")
	return true
}

func (c *redisCircuit) success() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.openUntil = time.Time{}
	c.probeInFlight = false
}

func (c *redisCircuit) failure(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.openUntil = now.Add(redisCircuitOpenFor)
	c.probeInFlight = false
	observability.Inc("redis_circuit_open_total")
}
