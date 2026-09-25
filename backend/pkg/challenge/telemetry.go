package challenge

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/ToufiqQureshi/hakaishield/pkg/observability"
)

// The client-reported telemetry (automation, headless, elapsed) can never
// be a trusted cryptographic statement — only the client's own JavaScript
// computes it. The honest treatment is:
//
//  1. bound and shape-check every field, so malformed input is rejected
//     rather than silently read;
//  2. treat the values as evidence with a recorded outcome, not as a
//     freely editable form field that alone decides a hard block;
//  3. measure the results by browser class, so a challenge that fails
//     real phones shows up as a number instead of a support ticket.
//
// The value is still bound to the issued challenge through the signed
// token (nonce, host, issue time, difficulty), which is what keeps one
// challenge's telemetry from being replayed against another.

// Difficulty bands, keyed off the rule score core.Guard already computed.
// The thresholds mirror signals.DecideWithPolicy (challenge at >0, the
// stronger block bar at 100), kept local so pkg/challenge stays free of a
// dependency on the whole signals package for two numbers.
const scoreMidBand = 50

const (
	// maxElapsedMillis bounds the self-reported solve time. A real solve
	// is well under a second; the ceiling only exists so an absurd value
	// is rejected instead of stored.
	maxElapsedMillis = 10 * 60 * 1000

	// fastSolveMillis is a reported time no scripted or human solve should
	// manage for a real puzzle. It is measured, never enforced: a fast
	// number on its own is not proof of anything (a cached page, a
	// paused VM, or a clock skew all produce it), so it must not be the
	// only reason a client is refused.
	fastSolveMillis = 25
)

// browserClass buckets the User-Agent just enough to see whether
// challenges are failing real phones. Cardinality is deliberately tiny
// (three labels) so it can be a bounded counter, not a metric explosion.
func browserClass(ua string) string {
	switch {
	case ua == "":
		return "unknown"
	case containsAny(ua, "Mobile", "Android", "iPhone", "iPad", "iPod"):
		return "mobile"
	default:
		return "desktop"
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// solveTelemetry is the validated, bounded view of what the challenge page
// reported. Anything malformed makes parseTelemetry report false, so the
// caller treats it as a failed solve rather than guessing a default.
type solveTelemetry struct {
	Automation           bool
	Headless             bool
	ElapsedMS            int
	FastSolve            bool
	ShaderF16Unsupported bool
	LegacyAutomation     bool
	PointerActive        bool
	PointerReported      bool
	CanvasDuplicate      bool
}

// A pointer absence is only worth counting after a full second of solve time.
// Shorter solves leave little opportunity for a person to interact. This is
// still observational: a real user can leave the pointer motionless for longer.
const pointerObservationMillis = 1000

func (t solveTelemetry) shadowSignals() []string {
	var signals []string
	if t.ShaderF16Unsupported {
		signals = append(signals, "challenge_shader_f16_unsupported")
	}
	if t.CanvasDuplicate {
		signals = append(signals, "challenge_canvas_duplicate")
	}
	if t.PointerReported && !t.PointerActive && t.ElapsedMS >= pointerObservationMillis {
		signals = append(signals, "challenge_pointer_activity_absent")
	}
	if t.LegacyAutomation {
		signals = append(signals, "challenge_legacy_automation_key")
	}
	return signals
}

// parseTelemetry validates the client's form fields. Booleans must be the
// exact strings "true"/"false" (not "", not "1", not "TRUE"), and elapsed
// must be a plain non-negative integer inside the bound. An empty elapsed
// is allowed for backwards compatibility with an older cached page that
// did not send it, and is recorded as zero.
func parseTelemetry(form url.Values) (solveTelemetry, bool) {
	var t solveTelemetry

	automation, ok := parseBoolField(form.Get("automation"))
	if !ok {
		return solveTelemetry{}, false
	}
	t.Automation = automation

	headless, ok := parseBoolField(form.Get("headless"))
	if !ok {
		return solveTelemetry{}, false
	}
	t.Headless = headless

	elapsed := form.Get("elapsed")
	if elapsed != "" {
		ms, err := strconv.Atoi(elapsed)
		if err != nil || ms < 0 || ms > maxElapsedMillis {
			return solveTelemetry{}, false
		}
		t.ElapsedMS = ms
	}
	t.FastSolve = elapsed != "" && t.ElapsedMS < fastSolveMillis
	if t.ShaderF16Unsupported, _, ok = optionalBool(form, "shaderF16Unsupported"); !ok {
		return solveTelemetry{}, false
	}
	if t.LegacyAutomation, _, ok = optionalBool(form, "legacyAutomation"); !ok {
		return solveTelemetry{}, false
	}
	if t.PointerActive, t.PointerReported, ok = optionalBool(form, "pointerActive"); !ok {
		return solveTelemetry{}, false
	}
	if second, exists := form["canvas2"]; exists {
		if len(second) != 1 || !strings.HasPrefix(second[0], "data:image/png;base64,") || len(second[0]) > maxCanvasProofLen {
			return solveTelemetry{}, false
		}
		t.CanvasDuplicate = second[0] == form.Get("canvas")
	}
	return t, true
}

func optionalBool(form url.Values, field string) (value, reported, valid bool) {
	values, exists := form[field]
	if !exists {
		return false, false, true
	}
	if len(values) != 1 || values[0] == "" {
		return false, true, false
	}
	value, valid = parseBoolField(values[0])
	return value, true, valid
}

func parseBoolField(v string) (bool, bool) {
	switch v {
	case "true":
		return true, true
	case "false", "":
		return false, true
	default:
		return false, false
	}
}

// Challenge outcome counters, split by browser class. Names are
// code-owned constants, so a visitor-controlled value can never become a
// counter name (CLAUDE.md Sections 16/19).
const (
	counterIssued          = "challenge_issued_total"
	counterSolved          = "challenge_solved_total"
	counterFailed          = "challenge_failed_total"
	counterSolvedMobile    = "challenge_solved_mobile_total"
	counterSolvedDesktop   = "challenge_solved_desktop_total"
	counterFailedMobile    = "challenge_failed_mobile_total"
	counterFailedDesktop   = "challenge_failed_desktop_total"
	counterFastSolve       = "challenge_solved_fast_total"
	counterTelemetryBad    = "challenge_telemetry_malformed_total"
	counterEscalatedIssued = "challenge_issued_escalated_total"
	counterVerifyOverload  = "challenge_verify_overload_total"
)

// recordChallengeOutcome adds one solve outcome to the aggregate counters,
// overall and split by browser class. It is the single place a counter
// name is chosen, so no caller can accidentally pass a visitor-controlled
// string as a counter name (CLAUDE.md Sections 16/19).
func recordChallengeOutcome(solved bool, ua string) {
	if solved {
		observability.Inc(counterSolved)
	} else {
		observability.Inc(counterFailed)
	}
	switch browserClass(ua) {
	case "mobile":
		if solved {
			observability.Inc(counterSolvedMobile)
		} else {
			observability.Inc(counterFailedMobile)
		}
	case "desktop":
		if solved {
			observability.Inc(counterSolvedDesktop)
		} else {
			observability.Inc(counterFailedDesktop)
		}
	}
}
