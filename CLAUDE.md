# CLAUDE.md — bot-shield Development Rules

This file is engineering rules only: coding standards, workflow, and
production-safety rules — **how** we work.

## What you're building (30 seconds)

> bot-shield is **the inline layer that decides which automated
> clients reach a site — and proves why it decided that.**
> Closed-source commercial software, one solo maintainer, sold as a
> **hosted service we run** at roughly $200/mo. Customers point DNS
> at us and install nothing.

Three things that are easy to get wrong, so they're here and not
three files away:

- **We are not "a cheaper DataDome."** The market's floor is $0
  (CrowdSec, Coraza, Cloudflare's free tier). Anything argued on
  price loses to free software. We win on *what it can prove*
  (per-request evidence) and on serving people Cloudflare serves
  badly — never on being cheapest.
- **We host it; their traffic is our bill.** Since 2026-09-16 this is
  a SaaS, not software customers install (`docs/DECISIONS.md`). Two
  consequences that change how you write code: anything per-customer
  needs a **tenant boundary**, and anything that consumes bandwidth or
  CPU per request is now a **cost line**, not just a latency line.
  Self-hosting survives as a priced-up Enterprise option — so never
  assume we are always the operator, and never delete the
  single-tenant path as dead code.
- **Inline TLS/JA4 scoring is our technical edge.** Free tools parse
  logs and react to IPs that already misbehaved. We score the first
  request. Protect that; don't dilute it with features that need a log
  pipeline. But it is code, and code gets rebuilt — the *commercial*
  moat is the maintained browser-fingerprint database (ROADMAP item
  19), because data goes stale and that decay is what a subscription
  actually pays for. Hosting makes that database cheaper to build —
  we see real browser traffic continuously.
- **The category is agent governance now, not bot blocking.** Allow /
  rate-limit / deceive / block *per agent*, with a record of why.

Full reasoning and the numbers behind it: `docs/ROADMAP.md` intro,
`docs/DECISIONS.md` (2026-09-16 positioning entry), `docs/RESEARCH.md`
(2026-09-16 market scan). Read those before proposing product
direction — they already contain the rejected alternatives.

## The four rules that matter most

If you read nothing else in this file, read these. Each one exists
because breaking it already cost this project something real.

1. **Section 24 — check the standard library first.** Four things
   here were hand-written that net/http already did better. Reading
   `$GOROOT/src` deleted ~60 lines and closed a security hole.
2. **Section 23a — mutation-check every test.** Break the code, watch
   the test go red. Two tests here passed while the feature they
   "tested" was deleted.
3. **Section 22 — run the pre-push checklist.** It has found a
   spoofable header, a silently dying listener, and a fingerprint
   bypass, all in code already called done.
4. **Section 17 — fix it, don't just report it.** A known bug that
   isn't fixed in the same pass is a bug shipped.

## Where to find things

| Need | Section |
|---|---|
| Which doc to read, and updating docs before you stop | 0 |
| What this project is for, what not to build | 1, 14 |
| Who pays for this and why — positioning | top of this file, `docs/ROADMAP.md` |
| Short code, comments, naming, file names | 3, 3a, 4, 5 |
| How detection signals must combine | 6 |
| Testing: order, real tests, mutation checks | 7, 15a, 23 |
| False positives, production safety, latency | 8, 9 |
| Respecting and deleting existing code | 12, 13, 24a |
| Standards for shipped work; reporting gaps honestly | 15, 15a, 16, 17 |
| Legal and ethical limits | 18 |
| Checklists: before coding / before pushing | 19, 22 |
| Threat-driven design | 20 |
| Standard library first; using the review tools | 24, 24b |
| Working alongside Antigravity; review scope | 25 |

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
8. **`claude_and_agy.md`** — *who* does what: Claude Code's role
   (PM/senior dev/reviewer) versus Antigravity's (implementer), the
   60/40 split, and how the two agents coordinate. Read this whenever
   assigning or reviewing work, not just when writing code.

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
      it was tested. "Tested how" means naming what you broke and
      which test caught it (Section 23a), not "tests pass".
- [ ] Run the Section 22 pre-push checklist, and `/security-review`
      if the change touches anything a visitor controls (Section 24b).
- [ ] Check no doc now contradicts another. If you changed behaviour,
      `README.md` and `docs/ARCHITECTURE.md` describe the *current*
      product, not the one you started the session with — a stale doc
      misleads exactly the person who trusted it.

Skipping this is the same failure mode as Section 16 (Self-Report
Gaps): work that isn't written down doesn't exist for whoever picks
this up next.

---

## 1. Project Goal

bot-shield is a **simple, production-grade bot detection and
mitigation service** in Go, sold as a hosted service the customer
reaches by pointing DNS at us. The goal is NOT to out-feature
Akamai/DataDome, and NOT to undercut them on price either —
"affordable" is not a position when free competitors already exist.
The goal is to score traffic *inline* and be able to *show its work*
afterwards. See the top of this file for the short version and
`docs/ROADMAP.md` for product direction.

**This is closed-source, commercial software — a paid product the
owner sells, not an open-source project.** It uses open-source
*libraries* internally where that's the sound engineering choice
(`fingerproxy`, `BotD`, see Section 24 and `docs/DECISIONS.md`), the
same way any commercial product depends on open-source components
without itself being open source. Never suggest an MIT/Apache/GPL
license, a public GitHub release, or "let's open-source this part" —
that is the opposite of the business this code exists to run.

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

When a name needs more than one word, use Go's camelCase — never
snake_case, which `gofmt` culture and every Go reviewer will read as
foreign. Real examples from this repo:

```go
ja4Fingerprint()   // unexported: lowercase first letter
JA4FromContext()   // exported: uppercase first letter
NewCaptureListener()
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
  Every worker pool, queue, and cache **you write** needs a bounded
  size. Handing the work to a battle-tested runtime instead — as the
  capture listener does by letting net/http manage connections — is
  the better answer where it exists, and satisfies this rule (see
  Section 24). What is banned is an unbounded pool of our own making.
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
[ ] Mutation-check every test: break the code, watch the test go RED,
    undo the break (Section 23a — a test that passes while the
    feature is broken is not a test)
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

---

## 22. Pre-Push Production Verification

Run this before calling any feature "done," even one you already
tested. It has caught real bugs before (a spoofable header, a
listener that died silently on one bad Accept) that the first round
of tests missed — this is not paperwork, it's a second, adversarial
pass over your own work.

```text
[ ] Re-read your own diff like an attacker, not the author: what's
    the cheapest way to break this, crash it, or fool it?
[ ] Any value that comes from outside bot-shield (a header, a query
    param, a cookie) and gets trusted or forwarded downstream — is it
    stripped/validated first, or could a visitor just set it
    themselves?
[ ] Every error path, not just the happy path: does it get logged or
    handled, or does it fail silently? (a swallowed error is a bug
    hiding, not a bug handled)
[ ] Every blocking call: does it have a timeout? What happens if 1000
    clients trigger the slow/worst case at once?
[ ] Did you fix the exact bug you were looking for, or did you also
    check the rest of the file for the same class of mistake?
[ ] What, concretely, does NO test cover right now? Say it out loud
    (or in `docs/PROGRESS.md`) — don't let "probably fine" stand in
    for a real answer.
[ ] Did you mutation-check the tests (Section 23a), including the
    ones that already existed before this change? A green suite is
    not evidence until you've seen it go red on purpose.
[ ] Did you change a function whose test you didn't re-read? If so,
    that test is now untrusted (Section 23c).
[ ] Did you check the standard library actually has no built-in for
    what you hand-wrote, by reading its source rather than assuming
    (Section 24)? Is there dead code or one-use indirection to delete
    before this ships?
[ ] Say plainly, in your own report of the work: is this "done", or
    is it "done for the current scope, here's what's still missing"?
    Those are different claims — never let the first one cover for
    the second.
```

This is Section 15a's bar made checkable — a checklist you actually
run, not just a mindset you hold.

---

## 23. A Green Test Suite Proves Nothing By Itself

The most dangerous state this project can be in is **a passing test
suite that isn't actually testing anything.** It reads as safety, so
nobody looks again — and the bug ships anyway.

This is not hypothetical. Tests written for this repo have already
been caught doing exactly this: a handshake-timeout test that passed
with the timeout deleted from the code (it was measuring its own read
deadline, not the server's behaviour), and a fingerprint test that
passed when the fingerprint was replaced with the literal string
`GARBAGE-NOT-A-FINGERPRINT`.

### 23a. The mutation check — mandatory, every test, no exceptions

> **Before a test counts as written: break the code it is supposed to
> protect, run the test, and watch it go RED. Then undo the break.**

If the test still passes while the feature is broken, it is not a
test. Delete it or fix it — never keep it, because a fake test is
worse than no test at all (no test is an honest gap; a fake test is a
lie that stops anyone from looking).

State in `docs/PROGRESS.md` what you broke and that the test caught
it. "Tests pass" is not a test report — "I removed X and the test
failed with Y" is.

### 23b. Ways a test lies (check for each of these)

- **Vacuous assertion** — the check lives inside a handler/callback
  that never ran. If the request never arrives, nothing is asserted
  and the test passes. Always assert *separately* that the callback
  actually ran.
- **Assertion satisfiable another way** — `err != nil` passes whether
  the server closed the connection or your own deadline expired.
  Assert the *specific* cause, not any-failure-will-do.
- **"Not empty" instead of "correct"** — a garbage value is not empty
  either. Check the real shape or the real expected value. A wrong
  fingerprint is worse than a missing one, because the scoring layer
  will trust it.
- **Only the easy input** — one obviously-invalid input doesn't prove
  robustness. Malformed, truncated, empty, nil, and
  lying-about-its-own-length inputs are the *normal* case for a
  bot-detection product, not edge cases.
- **Testing the mock, not the code** — if the test would pass against
  an empty implementation, it's testing scaffolding.

### 23c. Stale tests are a bug, not leftover clutter

When you change what a function does, its old test is now guarding
behaviour that no longer exists — and it will keep passing while
doing it. Every time you touch a function: open its test, and either
confirm it still checks the *current* behaviour, or update it. A test
you didn't re-read after changing the code is untrusted.

### 23d. Never write a test to make the suite green

The purpose of a test is to *fail* when the product is broken. If
you're shaping an assertion so it passes, stop — you're building the
exact trap described at the top of this section. Write the assertion
the product owner would want, then make the *code* satisfy it.

### 23e. Do this without being asked

The project owner is solo. There is no QA, no second reviewer, and no
one else who will catch a fake test. If this audit only happens when
they think to ask for it, the process has already failed. Run it
yourself, every time, and report what you found — including "I
re-checked X and it was genuinely fine."

---

## 24. Check the Standard Library Before Writing the Code

The best code in this repo is the code we didn't write. Before
writing any non-trivial function, and again before calling it done,
answer these five questions — **by reading the actual source or
current docs, never from memory**:

```text
[ ] Could this function be shorter, or does it do more than one thing?
[ ] Does Go's standard library already do this? (read $(go env GOROOT)/src,
    or the current package docs — do not guess)
[ ] Is that built-in production grade? (stdlib and golang.org/x: yes.
    A random GitHub package: check maintenance, tests, real users)
[ ] Is it a whole library, or one function/hook we can call?
[ ] If it exists and it's sound — use it, and delete ours.
```

**Why this is not optional.** On 2026-09-14, an audit against these
questions found that hand-written code in `proxy/capture.go` had
reimplemented four things net/http already does, worse:

| We hand-wrote | Standard library already had | Ours was worse because |
|---|---|---|
| TLS handshake timeout (atomic + init + context) | `Server.tlsHandshakeTimeout()`, from `ReadHeaderTimeout` | silently dropped failed handshakes instead of logging them or replying to a plain-HTTP client |
| Accept retry with backoff | `Server.Serve()`'s `tempDelay` loop | ours was a near-copy of stdlib code, untested against real transient errors |
| Per-connection panic recovery | `conn.serve()`'s `defer recover()` | duplicated |
| Goroutine-per-connection + semaphore | net/http's own connection handling | extra machinery, extra bugs |

Deleting all four removed about 60 lines and left the product
**safer**, because the stdlib's versions handle cases ours didn't.

The same audit found `httputil.ReverseProxy.Director` (what we used)
does **not** strip a visitor's `X-Forwarded-*` headers, while the
newer `Rewrite` does — meaning any visitor could forge their own IP,
defeating per-IP rate limiting before it was even built. One question
from the list above ("is there a newer built-in?") caught a real
security hole.

### 24a. Delete dead code before you add to it

Unused variables, one-use indirection, a helper with a single caller,
a constant nobody reads, a wrapper that wraps nothing — clear these
out *first*, so the next person reads only code that matters. A
junior dev opening any file in this repo should be able to tell what
it does without a guide.

Section 13 still applies: if something looks unused but you're not
certain, **ask the project owner before deleting it** — say what it
is and why it looks dead. Certainty deletes; doubt asks.

### 24b. Use the tools instead of eyeballing it

The Claude Code skills `/code-review`, `/security-review` and
`/simplify` exist and are cheap. Run them on your own work before
declaring it done — especially `/security-review` on anything that
touches visitor-controlled input. An independent pass does not share
your blind spots, which is the entire point on a solo project.

---

## 25. Multi-Agent Workflow — Claude Code + Antigravity

As of 2026-09-15, bot-shield is built by two coding agents working in
parallel, coordinated by the project owner through `agentchat/` (a
local dev tool, gitignored, not shipped product — see
`docs/DECISIONS.md`). It's a shared, plain `chat.jsonl` log — one
JSON object per line, `{"agent": "...", "text": "..."}` — plus
`agentchat/web.py` (stdlib-only, no dependencies) serving
`agentchat/index.html` and a `POST /api/messages` endpoint so the
project owner can type into the page directly.

**An MCP server (`agentchat/mcp_server.py`) was tried and removed the
same day.** Two real problems killed it, in this order: (1) it gave
both agents a shared code file to edit, which caused a live edit war
— each side's fix overwrote the other's, twice, confirmed in
`docs/DECISIONS.md`; (2) even working, it never solved the actual
goal — MCP tool calls only run when an agent chooses to call them, so
neither agent was ever notified when the other posted. Antigravity
confirmed its own background polling script "just prints to stdout,
which doesn't wake me up." Do not reintroduce an MCP server here for
this purpose — see `docs/DECISIONS.md` for what was actually verified
(via web research, cited there) about why MCP notifications can't wake
an idle agent, and what would actually be needed instead (a trigger
built into each agent's own host, which neither agent can build for
the other).

**Coordination is manual, by design now:** the project owner tells
each agent to check `chat.jsonl` when there's something to relay.
**`chat.jsonl` has no authentication on the `agent` field** — anyone
with write access to the file can claim to be anyone (confirmed: one
message was posted under "Claude Code" that Claude Code didn't send).
Accepted for a local, single-machine dev tool; never replicate this
pattern in bot-shield's own product.

**Split, by owner's direction:**
- **Antigravity** owns the dashboard/frontend and overall UI
  architecture (`docs/ROADMAP.md` P2 item 12 and related work) —
  roughly 60% of remaining build effort.
- **Claude Code** (this file's audience) owns the backend/proxy side —
  `proxy/`, `cmd/botshield/` — starting with ROADMAP item 5 (scoring
  engine). Roughly 40% of remaining build effort.

**Review responsibility sits with Claude Code, not just for its own
work:**
> Every change either agent makes — Antigravity's included — gets
> checked by Claude Code against this file's rules (Sections 6, 7, 8,
> 9, 22, 23, 24 in particular) before it counts as done. Owning less
> of the build does not mean reviewing less of it.

This does not make Claude Code Antigravity's gatekeeper for permission
to ship — it means: read what Antigravity built, check it against the
same bar this file sets for any change (tested, mutation-checked
where it claims to be tested, no single-signal verdicts, no silent
failures, standard library checked first), and say plainly in
`docs/PROGRESS.md` what was reviewed and what was found — the same
honesty Section 16 already requires for Claude Code's own work.

**Coordination is file-based, not continuous.** Claude Code has no
standing background process — it can read/post to `agentchat/` and
watch it for a bounded time within an active session (as done to get
Antigravity's item-5 confirmation on 2026-09-15), but cannot monitor
it 24/7 outside a session. Don't assume a message posted to
`agentchat/` reaches the other agent immediately; the owner relaying
or triggering a check-in is still sometimes necessary.

**Before touching a file outside your own owned area** (see split
above), check `agentchat/chat.jsonl` for a conflict and post there
first — this project already hit one real file collision in
`agentchat/` itself before this split was agreed.
