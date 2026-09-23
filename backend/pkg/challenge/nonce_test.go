package challenge

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func TestLocalNonceStoreExpiryAndBound(t *testing.T) {
	s := newLocalNonceStore()
	now := time.Now()
	for i := 0; i < maxUsedChallenges; i++ {
		if !s.Consume(context.Background(), strconv.Itoa(i), now.Add(time.Duration(i)*time.Nanosecond), challengeMaxAge) {
			t.Fatalf("new nonce %d rejected", i)
		}
	}
	if s.Consume(context.Background(), "0", now.Add(time.Second), challengeMaxAge) {
		t.Fatal("recent nonce replay accepted")
	}
	if !s.Consume(context.Background(), "next", now.Add(time.Second), challengeMaxAge) {
		t.Fatal("capacity blocked a new solve")
	}
	if len(s.used) != maxUsedChallenges {
		t.Fatalf("nonce count = %d, want %d", len(s.used), maxUsedChallenges)
	}
	if !s.Consume(context.Background(), "0", now.Add(time.Second), challengeMaxAge) {
		t.Fatal("oldest nonce should have been evicted at capacity")
	}
	if !s.Consume(context.Background(), "expired", now.Add(challengeMaxAge+time.Second), challengeMaxAge) {
		t.Fatal("new nonce rejected after expiry")
	}
	if len(s.used) != 1 {
		t.Fatalf("expired nonces retained: %d", len(s.used))
	}
}

func BenchmarkLocalNonceStoreAtCapacity(b *testing.B) {
	s := newLocalNonceStore()
	now := time.Now()
	for i := 0; i < maxUsedChallenges; i++ {
		s.Consume(context.Background(), strconv.Itoa(i), now, challengeMaxAge)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !s.Consume(context.Background(), "new-"+strconv.Itoa(i), now, challengeMaxAge) {
			b.Fatal("new nonce rejected")
		}
	}
}
