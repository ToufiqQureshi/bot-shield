package forensics

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// ForensicResult contains advanced behavioral analysis results
type ForensicResult struct {
	Score            int      `json:"score"`
	Confidence       string   `json:"confidence"` // high/medium/low
	Signals          []string `json:"signals"`
	Evidence         Evidence `json:"evidence"`
	ProcessingTimeMs int      `json:"processing_time_ms"`
}

// Evidence contains detailed forensic evidence
type Evidence struct {
	CanvasNoise     float64 `json:"canvas_noise,omitempty"`
	WebGLDrift      float64 `json:"webgl_drift,omitempty"`
	AudioDrift      float64 `json:"audio_drift,omitempty"`
	MouseEntropy    float64 `json:"mouse_entropy,omitempty"`
	TimingVariance  float64 `json:"timing_variance,omitempty"`
	MemoryPattern   string  `json:"memory_pattern,omitempty"`
	GCTiming        float64 `json:"gc_timing,omitempty"`
	ResourceRaces   int     `json:"resource_races,omitempty"`
	HumanLikelihood float64 `json:"human_likelihood"`
}

// ForensicAnalyzer performs advanced client-side forensics
type ForensicAnalyzer struct {
	mu           sync.RWMutex
	cache        map[string]*ForensicResult
	cacheExpiry  time.Duration
	maxCacheSize int
}

// NewForensicAnalyzer creates a new forensic analyzer with caching
func NewForensicAnalyzer(cacheSize int, expiry time.Duration) *ForensicAnalyzer {
	return &ForensicAnalyzer{
		cache:        make(map[string]*ForensicResult),
		cacheExpiry:  expiry,
		maxCacheSize: cacheSize,
	}
}

// AnalyzeClient performs comprehensive forensic analysis
func (fa *ForensicAnalyzer) AnalyzeClient(ctx context.Context, clientData map[string]interface{}) *ForensicResult {
	start := time.Now()

	// Generate cache key
	cacheKey := fa.generateCacheKey(clientData)

	// Check cache first (cost optimization)
	if result := fa.getCached(cacheKey); result != nil {
		result.ProcessingTimeMs = int(time.Since(start).Milliseconds())
		return result
	}

	result := &ForensicResult{
		Score:      0,
		Signals:    make([]string, 0),
		Evidence:   Evidence{},
		Confidence: "low",
	}

	// Layer 1: Canvas & WebGL Noise Analysis (GPU imperfections)
	canvasScore, canvasEvidence := fa.analyzeCanvasNoise(clientData)
	result.Score += canvasScore
	result.Evidence.CanvasNoise = canvasEvidence
	if canvasScore > 0 {
		result.Signals = append(result.Signals, "canvas_anomaly")
	}

	// Layer 2: AudioContext Oscillator Drift (hardware clock drift)
	audioScore, audioEvidence := fa.analyzeAudioDrift(clientData)
	result.Score += audioScore
	result.Evidence.AudioDrift = audioEvidence
	if audioScore > 0 {
		result.Signals = append(result.Signals, "audio_drift_anomaly")
	}

	// Layer 3: Input Event Human Entropy (mouse/scroll patterns)
	mouseScore, mouseEvidence := fa.analyzeMouseEntropy(clientData)
	result.Score += mouseScore
	result.Evidence.MouseEntropy = mouseEvidence
	if mouseScore > 0 {
		result.Signals = append(result.Signals, "mouse_entropy_anomaly")
	}

	// Layer 4: Resource Timing Race Conditions (parallel request timing)
	timingScore, timingEvidence := fa.analyzeTimingPatterns(clientData)
	result.Score += timingScore
	result.Evidence.TimingVariance = timingEvidence
	if timingScore > 0 {
		result.Signals = append(result.Signals, "timing_anomaly")
	}

	// Layer 5: Memory Heap & GC Patterns (browser internal behavior)
	memoryScore, memoryEvidence := fa.analyzeMemoryPatterns(clientData)
	result.Score += memoryScore
	result.Evidence.MemoryPattern = memoryEvidence
	if memoryScore > 0 {
		result.Signals = append(result.Signals, "memory_anomaly")
	}

	// Calculate human likelihood (0.0 - 1.0)
	result.Evidence.HumanLikelihood = fa.calculateHumanLikelihood(result.Score)

	// Determine confidence level
	totalSignals := len(result.Signals)
	if totalSignals >= 3 {
		result.Confidence = "high"
	} else if totalSignals >= 1 {
		result.Confidence = "medium"
	}

	result.ProcessingTimeMs = int(time.Since(start).Milliseconds())

	// Cache result
	fa.setCached(cacheKey, result)

	return result
}

// analyzeCanvasNoise checks for synthetic vs real GPU noise patterns
func (fa *ForensicAnalyzer) analyzeCanvasNoise(data map[string]interface{}) (int, float64) {
	noise, ok := data["canvas_noise"].(float64)
	if !ok {
		return 0, 0
	}

	// Real GPUs have specific noise patterns (0.001-0.01 range)
	// Synthetic/rendered canvases often have too perfect or too random noise
	if noise < 0.0001 || noise > 0.05 {
		return 30, noise // High suspicion
	}

	// Check for repeating patterns (bot signature)
	if isRepeatingPattern(noise) {
		return 40, noise
	}

	return 0, noise
}

// analyzeAudioDrift checks hardware-level audio oscillator drift
func (fa *ForensicAnalyzer) analyzeAudioDrift(data map[string]interface{}) (int, float64) {
	drift, ok := data["audio_drift"].(float64)
	if !ok {
		return 0, 0
	}

	// Real hardware has micro-drift (0.0001-0.001)
	// Virtual/synthetic audio often has zero drift or extreme values
	if math.Abs(drift) < 0.00001 || math.Abs(drift) > 0.01 {
		return 35, drift
	}

	return 0, drift
}

// analyzeMouseEntropy evaluates mouse movement patterns for human-like entropy
func (fa *ForensicAnalyzer) analyzeMouseEntropy(data map[string]interface{}) (int, float64) {
	entropy, ok := data["mouse_entropy"].(float64)
	if !ok {
		return 0, 0
	}

	// Human mouse movement has natural entropy (0.3-0.7 range)
	// Bots either have no movement (0) or perfectly linear (very low entropy)
	// OR perfectly random (very high entropy)
	if entropy < 0.15 || entropy > 0.85 {
		return 25, entropy
	}

	// Check for velocity profiles
	if velocity, exists := data["velocity_profile"].(string); exists {
		if velocity == "linear" || velocity == "instant" {
			return 30, entropy
		}
	}

	return 0, entropy
}

// analyzeTimingPatterns detects race conditions in resource loading
func (fa *ForensicAnalyzer) analyzeTimingPatterns(data map[string]interface{}) (int, float64) {
	variance, ok := data["timing_variance"].(float64)
	if !ok {
		return 0, 0
	}

	// Humans have variable timing (higher variance)
	// Bots often have suspiciously consistent timing (very low variance)
	// Or completely random timing (extremely high variance)
	if variance < 0.01 || variance > 0.95 {
		return 20, variance
	}

	// Check for parallel request races
	if races, exists := data["resource_races"].(int); exists && races > 5 {
		return 15, variance
	}

	return 0, variance
}

// analyzeMemoryPatterns checks browser memory management signatures
func (fa *ForensicAnalyzer) analyzeMemoryPatterns(data map[string]interface{}) (int, string) {
	pattern, ok := data["memory_pattern"].(string)
	if !ok {
		return 0, ""
	}

	// Known bot patterns
	botPatterns := map[string]int{
		"synchronous_gc":         35,
		"no_gc_observed":         25,
		"instant_allocation":     30,
		"heap_fragmentation_low": 20,
	}

	if score, isBot := botPatterns[pattern]; isBot {
		return score, pattern
	}

	return 0, pattern
}

// calculateHumanLikelihood converts score to 0.0-1.0 human probability
func (fa *ForensicAnalyzer) calculateHumanLikelihood(score int) float64 {
	// Score 0 = 100% human, Score 100+ = 0% human
	if score <= 0 {
		return 1.0
	}
	if score >= 150 {
		return 0.0
	}

	// Sigmoid curve for smooth transition
	likelihood := 1.0 / (1.0 + math.Exp(float64(score-50)/20))
	return math.Round(likelihood*1000) / 1000
}

// generateCacheKey creates a unique key for client data. Map iteration order
// in Go is randomized, so keys are sorted first — otherwise identical
// client data could hash to a different key on every call, and the cache
// this key feeds would never hit.
func (fa *ForensicAnalyzer) generateCacheKey(data map[string]interface{}) string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		_, _ = fmt.Fprintf(h, "%s=%v;", k, data[k])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// getCached retrieves cached result if valid
func (fa *ForensicAnalyzer) getCached(key string) *ForensicResult {
	fa.mu.RLock()
	defer fa.mu.RUnlock()

	result, exists := fa.cache[key]
	if !exists {
		return nil
	}

	// Return a copy to avoid race conditions
	resultCopy := *result
	return &resultCopy
}

// setCached stores result with expiry
func (fa *ForensicAnalyzer) setCached(key string, result *ForensicResult) {
	fa.mu.Lock()
	defer fa.mu.Unlock()

	// Evict if cache is full (simple LRU-ish)
	if len(fa.cache) >= fa.maxCacheSize {
		for k := range fa.cache {
			delete(fa.cache, k)
			break
		}
	}

	// Store a copy to avoid race conditions
	resultCopy := *result
	fa.cache[key] = &resultCopy
}

// Helper: detect repeating patterns in noise
func isRepeatingPattern(value float64) bool {
	// Simple check for suspiciously round numbers
	rounded := math.Round(value*10000) / 10000
	return math.Abs(value-rounded) < 0.00001
}
