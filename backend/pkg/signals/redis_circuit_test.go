package signals

import (
	"testing"
	"time"
)

func TestRedisCircuitSkipsRequestsUntilSingleRecoveryProbe(t *testing.T) {
	var circuit redisCircuit
	now := time.Unix(1_700_000_000, 0)

	if !circuit.allow(now) {
		t.Fatal("healthy circuit must allow Redis work")
	}
	circuit.failure(now)

	if circuit.allow(now.Add(redisCircuitOpenFor - time.Nanosecond)) {
		t.Fatal("open circuit must fail open without attempting Redis")
	}

	recoveryAt := now.Add(redisCircuitOpenFor)
	if !circuit.allow(recoveryAt) {
		t.Fatal("circuit must permit one recovery probe after the cooldown")
	}
	if circuit.allow(recoveryAt) {
		t.Fatal("only one recovery probe may run at a time")
	}

	circuit.success()
	if !circuit.allow(recoveryAt) {
		t.Fatal("a successful probe must close the circuit")
	}
}

func TestRedisCircuitExtendsCooldownAfterFailedProbe(t *testing.T) {
	var circuit redisCircuit
	now := time.Unix(1_700_000_000, 0)
	circuit.failure(now)

	probeAt := now.Add(redisCircuitOpenFor)
	if !circuit.allow(probeAt) {
		t.Fatal("expected recovery probe")
	}
	circuit.failure(probeAt)

	if circuit.allow(probeAt.Add(redisCircuitOpenFor - time.Nanosecond)) {
		t.Fatal("failed recovery probe must reopen the circuit")
	}
}
