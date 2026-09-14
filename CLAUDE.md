# CLAUDE.md — bot-shield Development Rules

This file is engineering rules only: coding standards, workflow, and
production-safety rules — **how** we work.

## 0. Documentation Map — Read In This Order

Before writing any code, read these files in order. Each answers a
different question; do not skip ahead.

1. **`docs/AGENT.md`** — *why* this project exists, what success looks
   like, and the standard to hold every change to. Read this first,
   always.
2. **`docs/ARCHITECTURE.md`** — *what* tech is used (proxy, storage,
   dashboard, deployment) and why each choice was made.
3. **`docs/ROADMAP.md`** — *what* to build and in what order (P0/P1/P2
   feature list, current status — check the "Done" section to know
   what already exists before building it again).
4. **`docs/DECISIONS.md`** — *why we chose what we chose*: a dated log
   of real decisions and rejected alternatives, so you don't re-derive
   or accidentally reverse reasoning that already happened.
5. **`docs/RESEARCH.md`** — background research this project is built
   on (how real bot-management vendors detect bots, what stealth
   tools defeat by default, open-source pieces already identified).
   Read before proposing a "new" detection idea — check it isn't
   already researched here.
6. **`docs/PROGRESS.md`** — chronological work log: what changed, in
   which file/function, why, and how it was tested — for every
   session, even a small change. Read the most recent entries to know
   exactly where the last session left off, in detail `ROADMAP.md`'s
   checklist doesn't carry.
7. **`CLAUDE.md`** (this file) — *how* to write the code: standards,
   naming, testing, safety rules, workflow.

If any of these is unclear or missing something you need, resolve
that before coding — do not guess and proceed.

### Mandatory: leave the docs current before you stop

A future session — yours or a different agent's — starts from these
files with zero memory of this conversation. Before ending any work
session (not just when a feature is "done"):

- [ ] Update `docs/ROADMAP.md`'s "Done" checklist for anything you
      finished, so nobody re-builds it.
- [ ] Add an entry to `docs/DECISIONS.md` for any real decision you
      made (tech choice, scope cut, rejected alternative, priority
      change) — see that file's format.
- [ ] Add to `docs/RESEARCH.md` if you researched a new threat,
      technique, or open-source tool — even if you didn't act on it
      yet, so the next session doesn't re-research it from scratch.
- [ ] If you changed the product's behavior/API, update
      `docs/ARCHITECTURE.md` or `README.md` to match.
- [ ] Append an entry to `docs/PROGRESS.md` — every change, even a
      single function or a small fix, with what changed, why, and how
      it was tested. This is mandatory for every session, not just
      "big" ones — see that file's format and why it matters more than
      usual for this project.

Skipping this is the same failure mode as Section 16 (Self-Report
Gaps): work that isn't written down doesn't exist for whoever picks
this up next.

---

## 1. Project Goal

bot-shield is a **simple, affordable, production-grade bot detection
and mitigation service** in Go. The goal is NOT to out-feature
Akamai/DataDome — it's to give small/mid-size companies a "good
enough" defense they can actually afford and self-host. See
`docs/ROADMAP.md` for product direction and priorities.

Do not add a feature just because a big vendor has it. Every feature
must earn its place against real bot traffic patterns (see Section
20, Threat-Driven Development).

---

## 2. Read Docs First — Always

Before writing a line of code: read `docs/ROADMAP.md`, the relevant
package's existing code and tests, and any related docs. Do not
blindly add or change code.

---

## 3. Code Must Stay Short and Simple

Prefer:

**simple code > clever code**

**short code > unnecessary abstraction**

**readable code > technically fancy code**

If something can be done correctly in 10 lines, do NOT make it 20.
Do not add unnecessary helper functions, interfaces, structs,
wrappers, error layers, comments, or configuration. Every line must
have a reason to exist. Do not build a scoring engine, cache, or queue
more complex than the current, real need — measure before optimizing.

### 3a. Comments: Beginner-Friendly, What/Why/Need — Not How

This is a solo-dev project — the code has to explain itself to
whoever (or whichever future agent) opens the file next, with zero
memory of this conversation. Every non-trivial function (detection
logic, fingerprint/TLS parsing, scoring, anything not a one-line
getter) gets **one short comment**, max 2-3 lines, right above it.

**How to write it:**

- Plain, everyday words. Write it like you're explaining it to a
  junior dev or the client's ops engineer, not another engineer who
  already knows TLS/JA4/scoring internals.
- One line for **what** it does, one line for **why** (what it
  catches / what problem it solves), if a "why" isn't obvious skip
  it rather than stretch for one.
- No jargon dump, no citing three doc files in one comment, no
  restating the code line by line. If you can't say it in 2-3 short
  lines, the explanation is too long — cut it, don't wrap it.
- One comment per function, not one comment per test case / per
  struct field / per line inside the function.

Example — good:
```go
// checkUserAgent flags a request when its claimed browser doesn't
// match how it actually behaves. Bots often lie about this.
```
Example — bad (too long, too technical, restates the code):
```go
// checkUserAgent implements a User-Agent consistency check by
// parsing the UA string, comparing it against the TLS/HTTP2
// fingerprint's inferred client family per RFC..., iterating over
// known browser signatures, and returning a mismatch score based on
// docs/RESEARCH.md section 3 combined with docs/DECISIONS.md's
// scoring-weight rationale from 2026-09-14...
```

This does not contradict Section 3's "no unnecessary comments" rule:
a comment that just restates the code ("// loop over items") is
still banned. A short, plain comment that explains why the code
exists is not decoration — it earns its place the same way a line of
code does.

---

## 4. Function Names Must Be Simple

Function names must be extremely easy to understand. A junior
developer should know what a function does just by reading its name.

Prefer:

```go
score()
fingerprint()
challenge()
classify()
block()
allow()
detect()
verify()
```

Avoid:

```go
executeMultiSignalBotClassificationPipeline()
initializeAdaptiveRiskAssessmentOrchestrator()
processInboundTrafficFingerprintCorrelationEngine()
```

Better:

```go
score_request()
check_fingerprint()
run_challenge()
```

---

## 5. File Names Must Be Simple

File names should immediately tell a developer what's inside.

Prefer:

```text
score.go
fingerprint.go
challenge.go
proxy.go
session.go
ratelimit.go
honeypot.go
dashboard.go
metrics.go
errors.go
```

Avoid:

```text
multi_signal_scoring_orchestration.go
traffic_classification_pipeline_manager.go
```

One file, one clear responsibility.

---

## 6. Detection Signals Must Be Independent Layers

This is bot-shield's core correctness rule, not just a style
preference:

> **No single signal may be the only thing standing between "allow"
> and "block."**

Advanced automation tools (e.g. patched browser automation frameworks)
are specifically built to defeat one or two common checks
(`navigator.webdriver`, basic CDP detection). A single boolean check
is a single point of failure.

- Every detection layer (TLS/JA4, HTTP/2 fingerprint, behavioral
  timing, session consistency, rate pattern) contributes a **score**,
  not a verdict.
- The final allow/challenge/block decision is threshold-based across
  combined signals, never a single `if isBot { block() }` on one
  signal.
- When adding a new signal, document what it catches that existing
  signals miss — duplicate signals add cost without adding coverage.

---

## 7. Tests Before Implementation

Every feature follows this process: understand the requirement →
write the test → watch it fail for the right reason → implement the
smallest correct version → make it pass → keep the test in the repo.

Every detection signal needs tests against:

- known-good traffic (a real browser's actual fingerprint/behavior)
- known-bot traffic (a scripted client's fingerprint/behavior)
- borderline/ambiguous cases (should not hard-crash the pipeline)

Do not merge a detection feature backed only by "it looked right in
manual testing."

---

## 8. False Positives Are a Production Incident, Not a Bug Report

Blocking a real human/customer is often worse for the client's
business than letting a bot through. Every scoring change must be
evaluated for both:

- **false negative rate** (bots getting through)
- **false positive rate** (real users getting blocked)

Never ship a stricter rule to "catch more bots" without checking what
it does to legitimate traffic patterns (mobile browsers, corporate
proxies/VPNs, accessibility tools, privacy-focused browsers).

---

## 9. Production Safety Rules

- **Latency**: this service sits in the request path. A slow decision
  is a slow site for every visitor. Budget and measure p50/p99 latency
  per signal; anything expensive (ML scoring, external lookups) must
  have a timeout and a safe fallback.
- **Fail open vs fail closed**: decide deliberately, per deployment,
  whether an internal error in bot-shield lets the request through
  (fail open — protects uptime) or blocks it (fail closed — protects
  security). Never let this default silently; make the client choose.
- **Concurrency**: never create unlimited goroutines or connections.
  Every worker pool, queue, and cache needs a bounded size.
- **Cancellation**: every blocking call takes a `context.Context`.
- **No unbounded storage**: fingerprint/session stores need eviction
  and size limits — this runs against live, adversarial traffic.

---

## 10. Error Messages

Errors should say what failed, where, and why. Prefer structured
errors. Do not build a giant custom error framework for simple
failures.

---

## 11. Documentation

Every detection signal and every scoring change needs a short doc
explaining: what it detects, what it costs (latency/complexity), and
its known false-positive risk. Keep it beginner-friendly — the person
reading it may be the client's ops engineer, not a security expert.

---

## 12. Existing Code Must Be Respected

Read a file, understand why it exists, check its tests and callers
before changing it. Do not rewrite working detection logic just
because you'd write it differently — bot detection code that works
against real traffic is hard-won; treat it carefully.

---

## 13. Dead / Unused Code Alert

If you find unused, duplicated, or unreachable code: **stop and alert
the project owner** before deleting it. Explain what it is and why it
looks unused. Do not silently remove it.

---

## 14. No Unnecessary Code

Do not add configuration, abstractions, or signals "in case a client
needs it later." Build for the threats the roadmap has already
identified as real (see `docs/ROADMAP.md`).

---

## 15. Solo/Small-Team Maintainer Mandate

Treat every merged feature as production code the day it ships, not a
draft. It must handle its own errors, timeouts, and cancellation; be
safe under sustained adversarial traffic (not just a clean demo); and
have tests that prove the failure cases, not just that it compiles.

Research before implementing anything nontrivial (TLS fingerprinting
internals, HTTP/2 frame parsing, scoring/ML approaches): check how
mature open-source projects (fingerproxy, BotD, open-appsec) solved
the same problem before writing a naive version.

### 15a. Write Every Line Like a Real Company Will Run It Tomorrow

This is not a demo, a portfolio piece, or a "make it work once"
script. bot-shield is going to sit directly in front of a paying
client's live website. Before writing or merging anything, hold it to
this bar:

- **Every file, every function, every feature needs a real test** —
  not "it compiled" or "it looked right when I ran it once." Test the
  normal case, the bad/attacker-controlled input case, and the
  timeout/failure case. If a function has no test, it is not done.
- **Fix bugs the moment you find them, in the same pass** — don't
  write "known issue, fix later" for something you already know how
  to fix. "Later" doesn't exist for a solo-dev project (see Section
  17); a bug found and not fixed is a bug shipped.
- **Neither over-engineered nor under-engineered:**
  - Under-engineered = missing something a real attacker or real
    production load would hit on day one: no timeout on a blocking
    call, no panic recovery on code that parses untrusted input, no
    cap on a resource a flood of connections could exhaust. This is
    not "polish for later" — it's the difference between "bot-shield
    protects the site" and "bot-shield *is* the outage."
  - Over-engineered = configuration, abstraction layers, or signals
    for a threat that isn't real yet (this duplicates Section 14 —
    the same discipline applies in both directions).
  - When in doubt, ask: "if this exact code ran in front of a real
    client's checkout page right now, what's the first way a bored
    attacker or a bad network breaks it?" If you can answer that and
    haven't handled it, it's not done yet.
- Before calling any feature finished, re-read this bar and the
  Section 19 checklist against it — not just "does it pass go test."

---

## 16. Self-Report Gaps Without Being Asked

The project owner is trusting this implementation. After finishing
any feature (not just when asked to review it), proactively state, in
the same message that reports the feature done:

1. What was built and why.
2. Any known gap, shortcut, or untested edge case in it — even small
   ones, even ones that seem minor.
3. What you'd fix next if given the choice.

Do not wait to be asked "is this really done?" Silence about a known
weakness is the same as hiding it.

---

## 17. Fix It, Don't Just Report It

Reporting a gap is not the same as handling it.

> **If you found it and you can fix it, fix it in the same pass. Only
> report-and-defer when fixing is genuinely blocked.**

"Genuinely blocked" means one of these, and you must say which:

- It needs a decision only the owner can make (a product/threat-model
  trade-off, not a technical one).
- It depends on something unavailable right now (missing credential,
  an environment that can't run it, an upstream bug).
- It is a large feature of its own, already on the roadmap, and
  fixing it now would mean shipping it half-done.

"It's an edge case", "it's rare in practice", "it's hard to test
here", and "I already documented it" are never valid reasons to defer.

---

## 18. Ethical / Legal Boundary

bot-shield **detects and mitigates** automated traffic. It must never
include, on either side:

- techniques whose purpose is to help automation defeat other
  companies' anti-bot systems — that is a different kind of project
  entirely and does not belong here.
- fingerprinting or data collection beyond what's needed to score a
  request (no scraping/selling of end-user personal data).
- dark-pattern blocking that harms accessibility tools, screen
  readers, or legitimate monitoring/uptime bots without a documented
  allowlist mechanism.

When in doubt about whether a signal or feature crosses this line,
stop and ask the project owner before building it.

---

## 19. Before Every Coding Task

```text
[ ] Read docs/ROADMAP.md and related docs
[ ] Read target files and related code
[ ] Read existing tests
[ ] Identify the real threat/gap this solves
[ ] Check whether it already exists
[ ] Write the test (good traffic + bot traffic + borderline cases)
[ ] Implement the smallest solution
[ ] Run tests, check false-positive impact
[ ] Add production/failure tests
[ ] Update docs
[ ] Check for unnecessary code
```

---

## 20. Threat-Driven Development

Before building a detection feature, document:

```text
Threat: what bot behavior/tool does this target?
Current signals: what do we already catch, and what slips through?
Gap: what specifically gets past existing layers?
Cost: latency/complexity this adds?
False-positive risk: what real traffic could this wrongly flag?
Test plan:
```

Do not add detection signals just to increase signal count — every
signal must close a real gap (see `docs/ROADMAP.md`).

---

## 21. Final Rule

Keep bot-shield **boring internally and effective externally**: short,
readable, testable, predictable. Impress with how little code is
needed to catch real bot traffic reliably — not with clever tricks.

**Every signal, every layer, every line must earn its place.**
