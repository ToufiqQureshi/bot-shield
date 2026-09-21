package settings

import (
	"context"
	"errors"
	"testing"
)

func TestUpsert_RejectsBlockNotGreaterThanChallenge(t *testing.T) {
	s := NewStore(nil)
	ctx := context.Background()

	cases := []Protection{
		{BlockThreshold: 50, ChallengeThreshold: 50}, // equal
		{BlockThreshold: 40, ChallengeThreshold: 50}, // block below challenge
	}
	for _, p := range cases {
		if err := s.Upsert(ctx, "usr_x", p); !errors.Is(err, ErrInvalid) {
			t.Errorf("Upsert(%+v) = %v, want ErrInvalid", p, err)
		}
	}
}

func TestUpsert_ValidThresholdsReachDatabaseCheck(t *testing.T) {
	// Proves the threshold rule runs before the database is touched:
	// a valid combination against a nil pool must fail with the
	// "not configured" error, not ErrInvalid.
	s := NewStore(nil)
	err := s.Upsert(context.Background(), "usr_x", Protection{
		BlockThreshold: 90, ChallengeThreshold: 50, ChallengeType: "pow", HoneypotEnabled: true,
	})
	if errors.Is(err, ErrInvalid) {
		t.Fatalf("Upsert(valid thresholds, nil pool) = %v, should not be ErrInvalid", err)
	}
	if err == nil {
		t.Fatal("Upsert(valid thresholds, nil pool) = nil, want a database-not-configured error")
	}
}

func TestGet_NilPoolFailsClearly(t *testing.T) {
	s := NewStore(nil)
	if _, err := s.Get(context.Background(), "usr_x"); err == nil {
		t.Fatal("Get(nil pool) = nil, want an error")
	}
}
