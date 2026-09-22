# CLAUDE.md — hakaishield Engineering Operating System

> **This file defines how engineering work is performed.**
>
> Claude Code is expected to operate autonomously: investigate, decide, implement,
> test, review, secure, simplify, document, and report work without needing the
> owner to repeatedly instruct it to perform each step.
>
> Do not wait for the owner to say "run tests", "check security", "read the docs",
> "update PROGRESS", or "review your diff". Those are part of the job.

---

# 0. Mission

hakaishield is a **hosted inline traffic intelligence and governance system**.

It sits in front of customer websites and evaluates automated traffic using
request-level evidence. It can then apply policy such as:

- allow
- rate-limit
- challenge
- deceive
- block

The important output is not only the action. The system must be able to explain
**why** the action was taken using useful, defensible evidence.

hakaishield is:

- closed-source
- commercial
- primarily a multi-tenant hosted SaaS
- operated as infrastructure in the request path
- capable of a priced Enterprise/self-hosted deployment

## Product principle

Do not think of hakaishield as simply "a bot blocker".

Think:

> **Identify automated traffic, collect defensible evidence, make a safe policy
> decision, and give the customer control over that traffic.**

The long-term product value includes maintained browser/fingerprint intelligence
and continuously improved detection knowledge.

Do not add features merely because Akamai, DataDome, Cloudflare, or another large
vendor has them. Every feature must solve a real threat, customer problem,
operational problem, or roadmap requirement.

## Product positioning

hakaishield is **not "a cheaper DataDome."** The market floor includes free
alternatives. Do not compete primarily on being the cheapest product.

The product differentiates through:

- request-level evidence
- explainable decisions
- inline TLS/JA4 and other defensible signals
- practical traffic governance
- maintained browser/fingerprint intelligence
- serving use cases where existing providers are a poor fit

Hosting means customer traffic is also our infrastructure cost. Any per-request
CPU, memory, bandwidth, network, storage, browser-rendering, or external API
operation is a cost consideration, not merely a latency consideration.

---

# 1. Autonomous Engineering Rule

Claude owns the engineering lifecycle of the requested change.

When given a meaningful task, automatically perform:

```text
Understand
  ↓
Read relevant documentation
  ↓
Investigate repository
  ↓
Find existing implementation
  ↓
Identify requirements / threat / impact
  ↓
Plan
  ↓
Write or update tests
  ↓
Implement smallest correct solution
  ↓
Run verification
  ↓
Adversarial review
  ↓
Security review
  ↓
Performance / resource / cost review
  ↓
Simplify
  ↓
Review final diff
  ↓
Update documentation
  ↓
Final verification
  ↓
Report truthfully
```

Do not wait for the owner to request individual steps.

If a step is genuinely irrelevant to a trivial change, skip it deliberately and
say so when reporting the work.

---

# 2. Decision Rules — When to Act and When to Ask

Claude should make normal technical decisions autonomously.

## Act without asking when:

- the requirement is clear
- the choice is reversible
- the choice follows existing architecture
- existing project decisions already answer it
- standard Go practice clearly applies
- the change is internal and low-risk

## Research first when:

- the implementation is non-trivial
- the behavior involves security-sensitive protocol details
- a mature open-source implementation may already solve the problem
- current standards or library behavior matter

## Ask the owner when:

- two valid choices materially change product behavior
- a security posture must be chosen
- a customer-facing policy/default is unclear
- the change conflicts with an existing documented decision
- the task materially changes architecture
- a destructive migration or irreversible operation is required
- credentials, infrastructure, or access are required and unavailable
- requirements genuinely cannot be inferred from project documentation

Do not ask questions merely because a decision exists.

**Resolve ordinary engineering decisions yourself. Escalate only decisions that
belong to the product owner.**

---

# 3. Documentation Is Project Memory

Before non-trivial work, read:

1. `docs/AGENT.md`
2. `docs/ARCHITECTURE.md`
3. `docs/ROADMAP.md`
4. `docs/DECISIONS.md`
5. `docs/RESEARCH.md`
6. `docs/PROGRESS.md`
7. this `CLAUDE.md`

Then inspect relevant source files, tests, callers, configuration, and interfaces.

Do not blindly implement from the task description if the repository already
contains the answer.

## Document responsibilities

| Document | Purpose |
|---|---|
| `AGENT.md` | Product purpose and engineering standard |
| `ARCHITECTURE.md` | Current architecture and system behavior |
| `ROADMAP.md` | What should be built and current status |
| `DECISIONS.md` | Why important choices were made |
| `RESEARCH.md` | Threats, techniques, vendors, libraries, research |
| `PROGRESS.md` | What actually happened in previous work |
| `CLAUDE.md` | How engineering work must be performed |

If documentation conflicts, do not silently invent an answer. Identify the
conflict, determine which source reflects the current intended state when
possible, and update affected documentation after resolving it.

## Mandatory documentation before stopping

For meaningful work:

- update `ROADMAP.md` when roadmap status changes
- update `DECISIONS.md` for meaningful technical/product decisions
- update `RESEARCH.md` for new threat/technique/tool research
- update `ARCHITECTURE.md` or `README.md` when behavior/API/architecture changes
- append to `PROGRESS.md` with what changed, why, files affected, tests,
  meaningful mutation checks, security/performance verification, and remaining gaps
- check that documentation does not contradict the implementation

A future session starts from these files with zero memory of this conversation.

---

# 4. Repository Investigation Before Coding

Before creating new code, search the repository.

Check:

```text
Does this already exist?
Is there already a helper?
Is there already an abstraction?
Is there already a test?
Is the behavior implemented elsewhere?
Who calls this?
What configuration controls it?
What assumptions do callers make?
Is there an open roadmap item?
Was this previously rejected?
Is there a known threat/research entry?
```

Prefer extending correct existing code over creating duplicate code.

Do not rewrite working detection logic merely because another design looks cleaner.
Detection behavior that works against real traffic is hard-won.

---

# 5. Threat-Driven Development

Every non-trivial detection/security feature must have a real reason.

Before implementation determine:

```text
Threat:
What behavior/tool/attack are we addressing?

Current coverage:
What already catches it?

Gap:
What gets through today?

New value:
What does this change catch or improve?

Cost:
CPU / memory / latency / network / operational cost

False-positive risk:
Which legitimate clients could be affected?

Test plan:
How will we prove it works?
```

Do not add signals merely to increase the signal count.

A new signal must provide meaningful coverage that existing signals do not already
provide.

---

# 6. Planning

For non-trivial work, create a concise internal plan:

```text
Goal
Affected components
Current behavior
Required behavior
Implementation approach
Tests
Security risks
Performance/cost risks
Documentation changes
```

Keep the plan proportional to the task.

Do not create architecture, abstractions, queues, caches, or configuration
frameworks for problems that do not require them.

---

# 7. Implementation Standard

Primary rule:

> **Simple, readable, production-grade code beats clever code.**

Prefer:

```text
correctness
clarity
small surface area
existing primitives
measurable behavior
```

Avoid unnecessary:

- interfaces
- wrappers
- factories
- generic frameworks
- configuration layers
- helper layers
- queues
- caches
- state machines
- abstractions

Every line must have a reason to exist.

If something can be done correctly in 10 lines, do not make it 20 without a reason.

Do not optimize without evidence.

---

# 8. Go Standards

Use normal Go conventions.

## Naming

Prefer names that explain behavior immediately:

```go
score()
detect()
classify()
allow()
block()
challenge()
verify()
fingerprint()
```

Avoid unnecessarily long orchestration names.

Use:

- `camelCase` for unexported identifiers
- `PascalCase` for exported identifiers
- standard Go acronym conventions such as `HTTP`, `TLS`, `JA4`

## Files

Prefer focused filenames:

```text
score.go
fingerprint.go
proxy.go
session.go
ratelimit.go
challenge.go
metrics.go
errors.go
dashboard.go
```

One file should have one obvious responsibility.

---

# 9. Comments

Comments explain **what/why**, not obvious implementation details.

Every non-trivial function should have one short comment when its purpose or
reason is not obvious.

Good:

```go
// checkUserAgent compares the claimed browser with observed request traits.
// Bots often imitate a browser but fail to reproduce its other signals.
```

Bad:

```go
// Loop through headers and check the value.
```

Use plain language suitable for a junior developer or client's ops engineer.

Do not put architecture documents inside source comments.

Do not add comments that merely restate code.

---

# 10. Detection Architecture

Detection signals are independent evidence sources.

Prefer:

```text
request
  ↓
signal
  ↓
evidence / score
  ↓
combined decision
  ↓
policy
  ↓
ALLOW / RATE-LIMIT / CHALLENGE / DECEIVE / BLOCK
```

Relevant layers may include:

```text
TLS / JA4
HTTP / HTTP2
Browser fingerprint
Behavior
Session consistency
Rate pattern
Known automation intelligence
Other researched signals
```

**No single weak signal may be the only reason for a hard block.**

Each layer contributes evidence or score. The final decision is based on combined
signals according to the documented architecture and policy.

When adding a signal, document what it catches that existing signals miss.
Duplicate signals add cost without meaningful coverage.

---

# 11. Testing Is Part of Implementation

A feature is not complete when code is written.

A feature is complete when its important behavior is proven.

For meaningful changes, test:

- normal behavior
- invalid input
- attacker-controlled input
- failure paths
- timeout behavior
- boundary conditions

For detection signals also test:

- known-good traffic
- known-bot traffic
- ambiguous/borderline traffic

Do not rely on manual testing alone.

## Required development sequence

```text
Understand requirement
→ write/update test
→ watch the test fail for the right reason
→ implement smallest correct version
→ make it pass
→ keep the test
```

---

# 12. Mutation Verification

A passing test suite is not automatically evidence that tests are useful.

For every important new or modified test:

```text
1. Run correct implementation → PASS
2. Identify exact behavior being protected
3. Temporarily break/remove/change that behavior
4. Run relevant test → MUST FAIL
5. Restore implementation
6. Run again → PASS
```

If the test still passes after the protected behavior is removed, the test is not
trustworthy. Fix or delete it before declaring the change complete.

Record meaningful mutation checks in `docs/PROGRESS.md`, for example:

```text
Mutation check:
Removed request timeout → timeout test failed as expected.
Restored timeout → test passed.
```

Do not manufacture meaningless mutations for trivial getters or generated code.

---

# 13. Test Quality — Do Not Let Tests Lie

Automatically look for:

### Vacuous assertions

A callback/handler may never run while the test still passes.

Assert that the expected operation actually occurred.

### Weak error assertions

`err != nil` can hide the wrong failure.

When the exact failure matters, assert the specific behavior.

### Garbage accepted as valid

"Not empty" is not equivalent to "correct".

For fingerprints, IDs, parsed values, and classifications, validate their actual
expected shape/value.

### Only happy-path input

Adversarial systems must test malformed, truncated, empty, nil, oversized, and
inconsistent input where relevant.

### Tests of mocks instead of production code

A test should fail if the real implementation is removed or broken.

### Stale tests

Whenever a function changes, re-read its tests. Update tests when behavior changes.

A test not re-read after a behavior change is untrusted.

### Never write a test merely to make the suite green

Write the assertion the product owner would want, then make the code satisfy it.

---

# 14. False Positives Are a Production Problem

Blocking legitimate traffic can directly harm a customer's website.

Every detection change must consider:

- false negatives
- false positives

Pay particular attention to:

- mobile browsers
- corporate proxies
- VPNs
- privacy-focused browsers
- accessibility tools
- legitimate crawlers
- uptime monitors
- unusual but valid clients

Never make a rule stricter merely because it catches more bots.

---

# 15. Production Safety

hakaishield runs in the request path.

## Latency

Measure p50/p95/p99 latency for relevant stages.

Expensive work must have:

- timeout
- cancellation
- bounded resource use
- explicit fallback behavior

A slow decision is a slow customer website.

## Failure behavior

Fail-open vs fail-closed is a deliberate security/product decision.

Never allow accidental failure behavior to silently define the product.

## Concurrency

Never create unlimited:

- goroutines
- connections
- workers
- queues
- memory growth

All custom pools must be bounded.

Prefer the standard library/runtime when it already solves the problem.

## Storage

Traffic-derived state must have limits and eviction/expiration.

Examples:

- fingerprints
- sessions
- rate-limit state
- temporary request data

## Cancellation

Blocking operations must have a cancellation path where applicable.

Use `context.Context`.

---

# 16. Multi-Tenant Security

hakaishield is a multi-tenant SaaS.

Treat tenant isolation as a security boundary.

Customer-specific data includes:

- configuration
- policies
- credentials
- traffic data
- metrics
- logs
- sessions
- fingerprints
- usage/billing information

Every customer-owned operation must have an explicit tenant scope.

Never trust a tenant identifier supplied by a visitor.

Tenant identity must come from a trusted, validated boundary.

For new storage/APIs ask:

```text
Can tenant A access tenant B's data?
Can a visitor forge tenant identity?
Can cached state cross tenants?
Can metrics/logs leak another tenant?
Can one tenant exhaust a shared resource?
```

---

# 17. Visitor-Controlled Input

Treat all visitor-controlled values as hostile.

Examples:

- headers
- cookies
- query parameters
- request bodies
- user-agent
- forwarded headers
- IP-related headers
- connection metadata
- TLS-derived input

Before trusting or forwarding a value:

```text
Who supplied it?
Can it be forged?
Was it validated?
Was it normalized?
Can it affect security decisions?
Can it reach the origin?
```

Never trust client-provided infrastructure headers merely because their names look
authoritative.

---

# 18. Security Review

After implementation, perform an attacker-oriented review.

Ask:

```text
Can this be spoofed?
Can it be bypassed?
Can malformed input crash it?
Can a visitor exhaust memory?
Can a visitor exhaust CPU?
Can a visitor create unlimited state?
Can a visitor create unlimited connections?
Can a visitor bypass rate limiting?
Can a visitor forge IP/tenant identity?
Can this cause origin abuse?
Can this cause unexpected fail-open behavior?
Can this trigger expensive work repeatedly?
Can this leak customer data?
```

For changes involving visitor-controlled input, run available security review
tooling when relevant.

Do not claim "secure" merely because tests pass.

---

# 19. Resource and Cost Awareness

hakaishield is hosted infrastructure.

A request is both a latency event **and potentially a cost event**.

For every new per-request operation consider:

```text
CPU cost
Memory cost
Bandwidth cost
External API cost
Storage cost
Connection cost
Worst-case attacker amplification
```

Especially scrutinize:

- browser rendering
- external network calls
- ML inference
- large request bodies
- expensive parsing
- repeated lookups
- unbounded logging
- unbounded telemetry

A feature that allows one request to trigger many expensive operations is not
production-ready.

---

# 20. Standard Library First

Before implementing non-trivial infrastructure, check the Go standard library
and established `golang.org/x` packages.

Do not rely on memory.

Check current source/docs and ask:

```text
Does Go already solve this?
Does net/http already solve this?
Does the runtime already provide this?
Is there a standard primitive?
Is there a maintained library that is clearly appropriate?
Can our implementation be deleted?
```

Prefer a battle-tested standard implementation over maintaining a custom copy.

### Important hakaishield lesson

A previous audit found hand-written infrastructure in `proxy/capture.go` that
duplicated behavior already handled by `net/http`, including:

- TLS handshake timeout behavior
- Accept retry/backoff
- per-connection panic recovery
- goroutine/connection handling

The cleanup removed roughly 60 lines and reduced risk.

Another audit found that `httputil.ReverseProxy.Director` did not strip visitor
supplied `X-Forwarded-*` headers, while the newer `Rewrite` path does. That exposed
a security issue around forged client IP information.

**Before writing infrastructure code, read the actual standard-library source or
current docs. Never assume.**

---

# 21. Existing Code and Dead Code

Respect existing working code.

Before changing it:

- understand why it exists
- inspect callers
- inspect tests
- inspect related docs

Do not rewrite working detection logic merely for style.

## Dead/unused code

If code appears unused:

1. determine whether it is genuinely dead
2. inspect callers, build tags, generated code, interfaces, tests, and entry points
3. if clearly dead and safe to remove, clean it up
4. if certainty is impossible, identify it and ask the owner before deletion

Do not silently delete code merely because it looks unused.

Do not build new code on top of obvious dead code when it can safely be removed.

---

# 22. Documentation Maintenance

Do not wait for the owner to remind you.

Update `ROADMAP.md` when:

- a roadmap item is completed
- status materially changes
- scope/priority changes

Update `DECISIONS.md` when:

- a meaningful technical decision is made
- architecture changes
- scope is deliberately cut
- an alternative is rejected
- a product/technical trade-off is resolved

Update `RESEARCH.md` when:

- a new threat is researched
- a detection technique is researched
- an important library/vendor/tool is evaluated
- new security knowledge affects implementation

Update `ARCHITECTURE.md` or `README.md` when:

- externally visible behavior changes
- APIs change
- architecture changes
- deployment behavior changes
- customer-visible configuration changes

Update `PROGRESS.md` for every meaningful work session with:

- what changed
- why
- affected files/components
- tests executed
- meaningful mutation checks
- security/performance verification
- known remaining gaps

Never write only:

```text
Tests pass.
```

Write what was actually verified.

---

# 23. Documentation Consistency

Before finishing, check:

```text
README
ARCHITECTURE
ROADMAP
DECISIONS
RESEARCH
PROGRESS
CLAUDE
```

They must not contradict the current implementation.

Stale documentation is a correctness problem because the next engineer/agent uses
it as project memory.

---

# 24. Pre-Push Production Verification

Run this before calling any meaningful feature "done":

```text
[ ] Re-read the final diff like an attacker, not the author.
[ ] What's the cheapest way to break, crash, or fool this?
[ ] Is every visitor-controlled value validated before trust/forwarding?
[ ] Can a visitor forge IP or tenant identity?
[ ] Are all error paths handled or logged?
[ ] Does every blocking call have an appropriate timeout/cancellation path?
[ ] What happens if many clients trigger the worst case simultaneously?
[ ] Is resource growth bounded?
[ ] Did the exact bug get fixed, not merely documented?
[ ] Did I inspect the rest of the relevant file for the same class of mistake?
[ ] What does NO test cover right now?
[ ] Did I mutation-check important tests?
[ ] Did I re-read tests for every changed function?
[ ] Did I check the standard library before keeping custom infrastructure?
[ ] Is there dead code or one-use indirection to remove?
[ ] Did I check false-positive impact?
[ ] Did I check p50/p95/p99 or relevant performance impact?
[ ] Did I check bandwidth/CPU/memory/external-cost impact?
[ ] Did I run /code-review when useful?
[ ] Did I run /security-review when relevant?
[ ] Did I run /simplify when useful?
[ ] Are docs current?
[ ] Is this truly DONE or only done for the current scope?
```

This checklist is a second adversarial pass, not paperwork.

---

# 25. Claude Code Review Tools

Use available review tools automatically when relevant:

```text
/code-review
/security-review
/simplify
```

Use them especially when:

- security-sensitive code changed
- visitor-controlled input changed
- proxy behavior changed
- authentication/tenant logic changed
- detection logic changed
- concurrency/resource handling changed

Do not run tools mechanically when they add no value to a trivial change.

Review tools supplement reasoning; they do not replace it.

---

# 26. Product Boundary — Defensive Only

hakaishield detects and governs automated traffic.

Never turn it into a tool for defeating other companies' anti-bot systems.

Do not implement:

- anti-detection techniques intended to evade third-party defenses
- CAPTCHA bypass
- stealth automation
- scraping evasion
- personal-data collection unrelated to documented product needs

Legitimate automated traffic must have a documented way to be recognized and
governed where appropriate.

Accessibility is not an enemy signal.

---

# 27. Dashboard Is Production Code

`dashboard/` is part of the product.

It is not "just frontend".

The dashboard must:

- handle API errors
- handle slow backend responses
- avoid misleading empty states
- display believable data
- respect tenant boundaries
- have meaningful tests
- handle loading/error states
- avoid silently hiding backend failures

The customer judges the product by what the dashboard tells them.

A blank or false-looking dashboard is a product correctness failure.

## Every frontend file must be brutally tested

hakaishield is a security product. A dashboard bug is not merely a UI
annoyance here — it can leak one tenant's data to another, let a visitor
forge a privileged action, or make the product itself the thing that gets
hacked.

For every meaningful frontend file (component, page, API client function,
auth flow, hook):

- Write tests the same way `CLAUDE.md` §11–§13 require for the backend:
  normal behavior, invalid/malformed input, empty/loading/error states, and
  anything auth- or tenant-boundary-related.
- Mutation-check the important ones the same way as backend code: break the
  behavior on purpose (remove a check, flip a condition, drop an
  `Authorization` header), confirm the test fails, then restore it. A test
  that still passes after the protected behavior is removed is not a real
  test — fix or delete it.
- Treat anything that touches auth tokens, tenant/owner identity, or data
  coming back from the backend as adversarial input, not trusted input.
- Do not ship a component/page/API function without a test unless it is
  trivial (pure presentation with no logic, no conditionals, no data
  fetching).

If the dashboard has no test runner installed yet, installing one (e.g.
Vitest, since the project already uses Vite) is part of doing this work, not
a separate task to defer.

## No dead code, no unnecessary code, beginner-readable

Every frontend file must stay something a junior developer can open and
understand in one pass.

- No unused imports, unused variables, unused props, unused components, or
  unused exported functions. If it is not called from anywhere, delete it —
  do not comment it out and do not leave it "just in case."
- No unnecessary abstraction: no wrapper component, custom hook, or utility
  function that exists for only one caller and adds no real reuse or
  clarity. Three similar lines beat a premature abstraction (same rule as
  §7 for the backend).
- No custom-built version of something a library already installed in
  `package.json` already provides. Before writing a custom date formatter,
  fetch wrapper, form validator, animation helper, drag-and-drop
  implementation, chart, or similar, check whether an existing dependency
  (or the standard browser/React APIs) already does it correctly. Do not
  add a new dependency for something trivial either — check the standard
  library/browser APIs first.
- Comments follow the same rule as backend code (§9): explain the non-obvious
  why, not the obvious what. No comment blocks restating what the JSX
  already shows.
- If a file, component, or code path is found to be dead (nothing renders
  it, nothing imports it, no route reaches it), remove it the same way §21
  requires for backend dead code: confirm it is genuinely unused, then
  delete it in the same pass rather than leaving it to rot.

## Full frontend–backend wiring is mandatory

A feature is not done if only one side of it exists. This is checked every
time frontend or backend code changes, not just when someone asks.

- If the frontend has a button, form, toggle, or section that implies an
  action or data ("Add domain", "Toggle rule", "Protection settings"), the
  corresponding backend endpoint must exist, be authenticated/tenant-scoped
  per §16–§17, and must actually be called by that UI element — not a
  handler that only updates local state or shows a fake success toast.
- If the backend has an endpoint, data model, or feature with no frontend
  surface for it, either wire it into the dashboard or explicitly record in
  `docs/PROGRESS.md`/`docs/ROADMAP.md` why it is intentionally backend-only
  or not yet exposed. Do not let backend capability silently sit unused
  while the dashboard shows something unrelated or fake in its place.
- When adding or changing a frontend API call, verify the request
  path/method, request body shape, and response shape against the actual
  backend handler and its response envelope (see `dashboard/src/lib/api.ts`
  header comment) — not against what seems reasonable. A mismatch here is a
  production bug even though both sides "look done" individually.
- This wiring check applies in both directions on every relevant change:
  after touching a backend handler, check what in the dashboard calls it;
  after touching a dashboard page/component, check what backend endpoint it
  expects to exist and whether it is real.

---

# 28. Fix It, Don't Just Report It

If you find a problem and can fix it safely in the current scope, **fix it in the
same pass**.

Only report-and-defer when fixing is genuinely blocked by:

- an owner decision
- an unavailable dependency/environment/credential/upstream issue
- a large separate feature already on the roadmap where fixing now would mean
  shipping it half-done

These are not valid reasons to defer a fix:

- "it's an edge case"
- "it's rare"
- "it's hard to test"
- "I documented it"

If you found a fixable bug, fix it.

---

# 29. Solo/Small-Team Maintainer Mandate

Treat every merged feature as production code the day it ships.

It must:

- handle its own errors
- have timeouts/cancellation where applicable
- be safe under sustained adversarial traffic
- have tests proving important failure cases
- avoid unnecessary complexity
- be documented enough for the next session

This is not a demo, portfolio piece, or "make it work once" script.

Ask:

> **If this exact code ran in front of a real client's checkout page right now,
> what is the first way a bored attacker or bad network could break it?**

If you can answer that and haven't handled it, it is not done yet.

---

# 30. Definition of Done

Do not use "done" as a synonym for "code exists".

A meaningful change should reach:

```text
IMPLEMENTED
    ↓
TESTED
    ↓
MUTATION-VERIFIED (when applicable)
    ↓
SECURITY-REVIEWED
    ↓
RESOURCE/PERFORMANCE/COST-CHECKED
    ↓
SIMPLIFIED
    ↓
DOCUMENTED
    ↓
FINAL DIFF REVIEWED
    ↓
DONE
```

If something is genuinely blocked:

```text
Status: Done for current scope

Blocked / remaining:
- ...

Reason:
- owner decision / unavailable dependency / separate substantial feature
```

Never hide incomplete work behind "done".

---

# 31. Final Report

When reporting completed work, automatically provide:

```text
## Completed
What changed and why.

## Verification
Tests and meaningful checks actually executed.

## Security
Important security checks performed and findings.

## Performance / Cost
Relevant latency, resource, load, or cost checks performed.

## Bugs Found / Fixed
Problems discovered during implementation or review.

## Documentation
Which docs were updated.

## Remaining Gaps
Anything genuinely incomplete or blocked.

## Production Status
READY
or
READY FOR CURRENT SCOPE — remaining gap: ...
```

Do not claim a check was performed if it was not.

Do not claim production readiness if important verification was skipped.

---

# 32. Operating Principle

The owner should be able to give Claude a task such as:

```text
"Implement roadmap item X."
```

and Claude should understand that it means:

```text
read the project
→ understand the product
→ investigate existing code
→ research when necessary
→ make normal technical decisions
→ implement
→ test
→ try to break it
→ review security
→ review performance/cost
→ simplify
→ update docs
→ verify the final result
→ report honestly
```

The owner should not need to separately say:

```text
read CLAUDE.md
read ROADMAP
write tests
run tests
check security
check performance
review your diff
update PROGRESS
update ROADMAP
check for bugs
```

Those are already part of the engineering job.

> **Do the work, verify the work, document the work, and tell the truth about the
> work.**

**That is the standard for every part of hakaishield.**

## When to use graphify vs archify

Two different tools are installed. They do not overlap:

- **graphify** — *understanding* an existing codebase. Read-only. Answers
  "what calls what", "how does X connect to Y", "what's the blast radius of
  changing this file". Use it before reading raw source when
  `graphify-out/graph.json` exists (see the `## graphify` section below —
  this is enforced by a hook, not optional).
- **archify** — *explaining/sharing* a system, workflow, or plan as a
  standalone interactive HTML diagram. Use it when the owner (or a PR
  description, an incident writeup, an onboarding doc) needs a visual —
  e.g. "diagram the deception+honeypot request flow", "show the scoring
  pipeline as a sequence diagram", "visualize the JA4 evidence trail for
  the owner". It produces a shareable artifact, not something Claude
  queries internally. Reach for `graphify path`/`graphify explain` first to
  get the facts right, then hand those facts to archify if a diagram is
  what's actually being asked for.

Rule of thumb: graphify answers Claude's own questions about the code;
archify produces something a human looks at.

## graphify

This project has a knowledge graph at graphify-out/ with god nodes, community structure, and cross-file relationships.

Rules:
- For codebase questions, first run `graphify query "<question>"` when graphify-out/graph.json exists. Use `graphify path "<A>" "<B>"` for relationships and `graphify explain "<concept>"` for focused concepts. These return a scoped subgraph, usually much smaller than GRAPH_REPORT.md or raw grep output.
- If graphify-out/wiki/index.md exists, use it for broad navigation instead of raw source browsing.
- Read graphify-out/GRAPH_REPORT.md only for broad architecture review or when query/path/explain do not surface enough context.
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).
