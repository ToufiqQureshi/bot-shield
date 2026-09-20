package forensics

import (
	"context"
	"testing"
	"time"
)

func TestForensicAnalyzer_HumanTraffic(t *testing.T) {
	analyzer := NewForensicAnalyzer(100, 5*time.Minute)

	// Simulate real human traffic data
	humanData := map[string]interface{}{
		"canvas_noise":     0.0035,
		"audio_drift":      0.0005,
		"mouse_entropy":    0.45,
		"velocity_profile": "natural",
		"timing_variance":  0.35,
		"resource_races":   2,
		"memory_pattern":   "normal",
	}

	ctx := context.Background()
	result := analyzer.AnalyzeClient(ctx, humanData)

	if result.Score > 50 {
		t.Errorf("Human traffic scored too high: %d", result.Score)
	}

	if result.Confidence != "low" && result.Confidence != "medium" {
		t.Errorf("Expected low/medium confidence for human, got: %s", result.Confidence)
	}

	if result.Evidence.HumanLikelihood < 0.5 {
		t.Errorf("Human likelihood too low: %f", result.Evidence.HumanLikelihood)
	}

	if len(result.Signals) > 2 {
		t.Errorf("Too many signals for human: %v", result.Signals)
	}
}

func TestForensicAnalyzer_BotTraffic(t *testing.T) {
	analyzer := NewForensicAnalyzer(100, 5*time.Minute)

	// Simulate bot traffic with multiple anomalies
	botData := map[string]interface{}{
		"canvas_noise":     0.00001,  // Too perfect
		"audio_drift":      0.0,      // No drift (synthetic)
		"mouse_entropy":    0.05,     // Linear movement
		"velocity_profile": "linear", // Bot signature
		"timing_variance":  0.001,    // Too consistent
		"resource_races":   8,        // Parallel requests
		"memory_pattern":   "synchronous_gc",
	}

	ctx := context.Background()
	result := analyzer.AnalyzeClient(ctx, botData)

	if result.Score < 100 {
		t.Errorf("Bot traffic scored too low: %d", result.Score)
	}

	if result.Confidence != "high" {
		t.Errorf("Expected high confidence for bot, got: %s", result.Confidence)
	}

	if result.Evidence.HumanLikelihood > 0.3 {
		t.Errorf("Human likelihood too high for bot: %f", result.Evidence.HumanLikelihood)
	}

	if len(result.Signals) < 3 {
		t.Errorf("Expected multiple signals for bot, got: %v", result.Signals)
	}
}

func TestForensicAnalyzer_PatchrightLike(t *testing.T) {
	analyzer := NewForensicAnalyzer(100, 5*time.Minute)

	// Patchright-like: good TLS/fingerprint but behavioral anomalies
	patchrightData := map[string]interface{}{
		"canvas_noise":     0.004,     // Good (real GPU)
		"audio_drift":      0.0003,    // Good (real hardware)
		"mouse_entropy":    0.08,      // BAD (automated movement)
		"velocity_profile": "instant", // BAD (bot signature)
		"timing_variance":  0.02,      // Suspicious (too consistent)
		"resource_races":   3,
		"memory_pattern":   "normal",
	}

	ctx := context.Background()
	result := analyzer.AnalyzeClient(ctx, patchrightData)

	// Should detect behavioral anomalies despite good fingerprints
	if result.Score < 50 {
		t.Errorf("Patchright-like traffic should score higher: %d", result.Score)
	}

	// Check that mouse/timing signals were detected
	foundMouse := false
	for _, signal := range result.Signals {
		if signal == "mouse_entropy_anomaly" {
			foundMouse = true
		}
	}

	if !foundMouse {
		t.Error("Failed to detect mouse entropy anomaly")
	}
}

func TestForensicAnalyzer_ScraplingLike(t *testing.T) {
	analyzer := NewForensicAnalyzer(100, 5*time.Minute)

	// Scrapling-like: randomized fingerprints but timing issues
	scraplingData := map[string]interface{}{
		"canvas_noise":     0.06, // BAD (too random - synthetic)
		"audio_drift":      0.02, // BAD (extreme drift)
		"mouse_entropy":    0.5,  // Good (simulated)
		"velocity_profile": "natural",
		"timing_variance":  0.98, // BAD (completely random timing)
		"resource_races":   1,
		"memory_pattern":   "no_gc_observed",
	}

	ctx := context.Background()
	result := analyzer.AnalyzeClient(ctx, scraplingData)

	if result.Score < 80 {
		t.Errorf("Scrapling-like traffic should score higher: %d", result.Score)
	}

	// Should detect canvas, audio, timing, and memory anomalies
	if len(result.Signals) < 3 {
		t.Errorf("Expected multiple signals for Scrapling, got: %v", result.Signals)
	}
}

func TestForensicAnalyzer_Caching(t *testing.T) {
	analyzer := NewForensicAnalyzer(100, 5*time.Minute)

	data := map[string]interface{}{
		"canvas_noise":  0.003,
		"audio_drift":   0.0004,
		"mouse_entropy": 0.4,
	}

	ctx := context.Background()

	// First call
	result1 := analyzer.AnalyzeClient(ctx, data)

	// Second call (should be cached)
	result2 := analyzer.AnalyzeClient(ctx, data)

	if result1.Score != result2.Score {
		t.Error("Cached result differs from original")
	}

	// Processing time should be minimal for cached result
	if result2.ProcessingTimeMs > result1.ProcessingTimeMs {
		t.Logf("Note: Cached result took %dms vs original %dms",
			result2.ProcessingTimeMs, result1.ProcessingTimeMs)
	}
}

func TestForensicAnalyzer_CacheKeyIsDeterministic(t *testing.T) {
	analyzer := NewForensicAnalyzer(100, 5*time.Minute)

	data := map[string]interface{}{
		"canvas_noise":   0.003,
		"audio_drift":    0.0004,
		"mouse_entropy":  0.4,
		"gc_timing":      1.2,
		"resource_races": 0,
	}

	// Go randomizes map iteration order on every range, including two
	// ranges over the exact same map in the same process. Without
	// sorting keys first, generateCacheKey would produce a different
	// string most of the time, and AnalyzeClient's cache would miss on
	// every call even for identical input.
	first := analyzer.generateCacheKey(data)
	for i := 0; i < 20; i++ {
		if got := analyzer.generateCacheKey(data); got != first {
			t.Fatalf("generateCacheKey is non-deterministic: call 1 = %q, call %d = %q", first, i+2, got)
		}
	}
}

func TestForensicAnalyzer_EmptyData(t *testing.T) {
	analyzer := NewForensicAnalyzer(100, 5*time.Minute)

	ctx := context.Background()
	result := analyzer.AnalyzeClient(ctx, map[string]interface{}{})

	if result.Score != 0 {
		t.Errorf("Empty data should score 0, got: %d", result.Score)
	}

	if result.Confidence != "low" {
		t.Errorf("Empty data should have low confidence, got: %s", result.Confidence)
	}

	if result.Evidence.HumanLikelihood != 1.0 {
		t.Errorf("Empty data should be 100%% human, got: %f", result.Evidence.HumanLikelihood)
	}
}

func TestForensicAnalyzer_CacheEviction(t *testing.T) {
	analyzer := NewForensicAnalyzer(10, 5*time.Minute) // Small cache

	ctx := context.Background()

	// Fill cache
	for i := 0; i < 15; i++ {
		data := map[string]interface{}{
			"canvas_noise": float64(i) * 0.001,
		}
		analyzer.AnalyzeClient(ctx, data)
	}

	// Cache should not exceed max size
	analyzer.mu.RLock()
	cacheSize := len(analyzer.cache)
	analyzer.mu.RUnlock()

	if cacheSize > 10 {
		t.Errorf("Cache exceeded max size: %d > 10", cacheSize)
	}
}

func TestForensicAnalyzer_HumanLikelihoodCalculation(t *testing.T) {
	analyzer := NewForensicAnalyzer(100, 5*time.Minute)

	testCases := []struct {
		score       int
		expectedMin float64
		expectedMax float64
	}{
		{0, 0.99, 1.0},
		{25, 0.7, 0.9},
		{50, 0.45, 0.55},
		{75, 0.1, 0.3},
		{100, 0.0, 0.1},
		{150, 0.0, 0.01},
	}

	for _, tc := range testCases {
		likelihood := analyzer.calculateHumanLikelihood(tc.score)
		if likelihood < tc.expectedMin || likelihood > tc.expectedMax {
			t.Errorf("Score %d: likelihood %f outside range [%f, %f]",
				tc.score, likelihood, tc.expectedMin, tc.expectedMax)
		}
	}
}

func TestForensicAnalyzer_ConcurrentAccess(t *testing.T) {
	analyzer := NewForensicAnalyzer(1000, 5*time.Minute)

	ctx := context.Background()
	done := make(chan bool, 10)

	// Launch concurrent analyzers
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				data := map[string]interface{}{
					"canvas_noise":  float64(id+j) * 0.001,
					"mouse_entropy": 0.5,
				}
				analyzer.AnalyzeClient(ctx, data)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// If we get here without panic, test passed
}
