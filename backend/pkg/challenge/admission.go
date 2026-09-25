package challenge

import (
	"sync"

	"github.com/ToufiqQureshi/hakaishield/pkg/observability"
)

// verifyAdmissions bounds how many verify calls run at once. Issuing a
// challenge needs no authentication, and every verify call parses
// attacker-supplied form data and PNGs, so an attacker able to start
// unbounded verifies concurrently is able to spend unbounded server CPU.
// The gate is a ceiling, not a queue: work beyond it is refused with 503
// (a real retry-able overload answer) and counted, which is the bounded
// overload behavior the plan's track 2 asks for. The number is small on
// purpose — a genuine page load holds one slot for the few milliseconds
// its canvas decode takes.
const verifyAdmissions = 64

// verifyMu guards the running counter. It is package-level, not per
// challenge instance: the ceiling protects the process's CPU, which all
// tenants' verify calls share. The critical section is two
// instructions, far cheaper than the PNG decode the slot protects.
var (
	verifyMu      sync.Mutex
	verifyRunning int
)

// MaxConcurrentVerifies reports the verify-path admission ceiling, for
// tests and operators reading the code.
func MaxConcurrentVerifies() int { return verifyAdmissions }

// TryAdmitVerify runs job when a verify slot is free, and reports false
// — without running anything — when the path is already at its ceiling.
// The slot is released when job returns, including on its panic.
func TryAdmitVerify(job func()) bool {
	verifyMu.Lock()
	if verifyRunning >= verifyAdmissions {
		verifyMu.Unlock()
		observability.Inc(counterVerifyOverload)
		return false
	}
	verifyRunning++
	verifyMu.Unlock()

	defer func() {
		verifyMu.Lock()
		verifyRunning--
		verifyMu.Unlock()
	}()
	job()
	return true
}
