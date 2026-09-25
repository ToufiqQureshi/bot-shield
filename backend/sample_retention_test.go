package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPruneSamplesUsesBoundedBatchesAndConfiguredCutoff(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	calls := 0
	deleted, err := pruneSamples(context.Background(), now, 30, func(_ context.Context, cutoff time.Time) (int64, error) {
		calls++
		if want := now.AddDate(0, 0, -30); !cutoff.Equal(want) {
			t.Fatalf("cutoff=%v, want %v", cutoff, want)
		}
		if calls == 1 {
			return sampleRetentionBatch, nil
		}
		return 7, nil
	})
	if err != nil || deleted != sampleRetentionBatch+7 || calls != 2 {
		t.Fatalf("deleted=%d calls=%d err=%v", deleted, calls, err)
	}
}

func TestPruneSamplesStopsOnErrorAndCancellation(t *testing.T) {
	boom := errors.New("database down")
	deleted, err := pruneSamples(context.Background(), time.Now(), 30, func(context.Context, time.Time) (int64, error) {
		return 0, boom
	})
	if deleted != 0 || !errors.Is(err, boom) {
		t.Fatalf("deleted=%d err=%v, want database error", deleted, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	deleted, err = pruneSamples(ctx, time.Now(), 30, func(context.Context, time.Time) (int64, error) {
		calls++
		cancel()
		return sampleRetentionBatch, nil
	})
	if deleted != sampleRetentionBatch || !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("cancel: deleted=%d calls=%d err=%v", deleted, calls, err)
	}
}

func TestPruneSamplesCapsBacklogWorkPerRun(t *testing.T) {
	calls := 0
	deleted, err := pruneSamples(context.Background(), time.Now(), 30, func(context.Context, time.Time) (int64, error) {
		calls++
		return sampleRetentionBatch, nil
	})
	if err != nil || calls != maxRetentionBatches || deleted != int64(maxRetentionBatches)*sampleRetentionBatch {
		t.Fatalf("deleted=%d calls=%d err=%v", deleted, calls, err)
	}
}

func TestPruneSamplesRejectsInvalidRetention(t *testing.T) {
	called := false
	_, err := pruneSamples(context.Background(), time.Now(), 0, func(context.Context, time.Time) (int64, error) {
		called = true
		return 0, nil
	})
	if err == nil || called {
		t.Fatalf("invalid retention err=%v called=%v", err, called)
	}
}
