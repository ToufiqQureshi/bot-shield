# HakaiShield Backend Implementation Plan

## Purpose

This is the backend plan for turning HakaiShield into a production-grade,
explainable bot and agent traffic-governance service. It is deliberately a
backend plan: the Go proxy, scoring, challenge, tenant state, storage, and API
contract are the source of truth. The dashboard must only display behaviour
that the backend actually enforces or records.

The goal is not to copy every feature from reference projects. We will use
their ideas, public standards, and false-positive lessons, then implement
original Go code that fits HakaiShield's tenant isolation, evidence model, and
request-path latency budget.

## Current Backend Baseline

Already present in `backend/`:

- TLS ClientHello capture and JA4 parsing (`pkg/core`, `pkg/signals`).
- Reverse proxy with spoofable identity-header stripping and bounded response
  rewriting (`pkg/core`).
- Weighted signals: UA/header consistency, known JA4, scripting tools,
  velocity, crawl breadth, and honeypot trips (`pkg/signals`).
- Balanced and strict policies, plus shadow mode (`pkg/config`, `pkg/core`).
- JavaScript challenge with proof-of-work, basic browser checks, and a signed
  passed cookie (`pkg/challenge`).
- Deception response and a tenant-scoped honeypot (`pkg/deception`,
  `pkg/signals`).
- In-memory per-tenant stats/evidence and an initial lazy tenant store
  (`pkg/tenant`, `pkg/evidence`, `pkg/stats`, `pkg/db`).

## Non-Negotiable Product Rules

Every implementation phase must preserve these rules.

1. No single weak signal can hard-block a visitor.
2. Unknown browser/fingerprint data is neutral, never automatically malicious.
3. Every customer-owned record and Redis key is tenant-scoped.
4. Request-path work must be bounded, cancellable, and have an explicit
   fail-open or fail-closed decision.
5. A dashboard must never show generated/mock protection data as real traffic.
6. A new detection signal must state its threat, coverage gap, cost,
   false-positive risk, and tests before it affects enforcement.
7. Reference-repo code is not copied. Techniques are independently designed,
   implemented, and tested; any future third-party code reuse keeps required
   license notices.

## Phase 0 — Correctness and Security Foundation

These are prerequisites. Adding more signals before these are fixed would make
the product harder to maintain and could create bypasses or false positives.

| Work | Why | Backend home | Done when |
|---|---|---|---|
| Evaluate dynamic signals once per request | Current score and evidence analysis can run Redis-backed checks twice, which can double counters and make the evidence disagree with the decision. | `pkg/signals`, `pkg/core/guard.go` | One immutable evaluation result contains score, fired signals, and decision; a test proves one request changes each counter once. |
| Harden verified-good-bot DNS | DNS suffix matching must require a true domain boundary; cache state must be bounded and DNS work cannot become an attacker-controlled latency amplifier. | `pkg/signals/goodbots.go` | Spoofed lookalike PTR names fail, genuine forward-confirmed bots pass, cache/timeout limits are tested. |
| Trusted client-IP resolver | **Done 2026-09-22:** explicit trusted-proxy CIDRs and correct IPv4/IPv6 parsing. Raw forwarded headers remain untrusted unless the direct peer is trusted. | `pkg/core/identity.go`, `main.go` | CDN/LB, direct-client, forged-header, malformed-header, and IPv6 tests pass. |
| Host and tenant canonicalization | Normalize hostnames, safely remove ports, validate SNI/Host when TLS is used, and prevent stale/cross-tenant host mappings. | `pkg/tenant`, `pkg/core` | Case, port, IPv6, unknown-host, and cross-tenant tests pass. |
| Bind challenge state | A solve must be tied to the protected host/tenant and a short-lived visitor context; a captured token/cookie must not become a portable bypass. | `pkg/challenge`, `pkg/core/guard.go` | Cross-host, replay, expiry, tamper, and normal-browser flow tests pass. |
| Redis health/circuit behaviour | A disconnected Redis client must not add multiple request timeouts per visitor. | `pkg/signals` | Outage tests prove bounded latency, visible health state, and documented fail-open behaviour. |
| Separate data-plane from admin APIs | Customer traffic endpoints, health checks, and dashboard/control APIs need distinct authentication and exposure rules. | `main.go`, `pkg/api` | Tenant A cannot read Tenant B; stats/evidence are not anonymously enumerable. |

## Phase 1 — Policy and Explainable Decisions

Inspired mainly by go-away's condition/action model and Anubis's operational
policy approach, but implemented in HakaiShield's own small Go design.

| Feature | What will be implemented | Why it matters |
|---|---|---|
| Per-tenant policy model | Versioned policy with ordered rules, safe defaults, and actions: allow, rate-limit, challenge, deceive, block. | Different sites need different protection for login, search, pricing, checkout, and public content. |
| Safe conditions | Match normalized path/method, CIDR, verified agent, score band, signals, request class, and tenant allowlists. | Lets customers express business intent without raw code changes. |
| Endpoint classes | Explicit categories such as login, API, browse, checkout, and static asset with validated defaults. | A single global rate limit cannot protect both a login form and a high-asset page. |
| Decision object | Immutable request evaluation: facts, signals, score, matched rule, action, and enforcement status. | The proxy, evidence trail, metrics, and dashboard always agree on why a decision happened. |
| Rule validation and preview | Reject unsafe/ambiguous policies; provide shadow-only preview before enforce. | A bad rule must not silently take down a customer's site. |
| Customer overrides | Verified bot/monitor allow rules, challenge themes, and block/deception response options. | This makes the product usable without weakening global protections. |

Guardrails:

- A User-Agent-only allow rule never skips verification and scoring.
- Deception is only available above a stricter confidence threshold than block,
  because showing false data to a real user is worse than returning an error.
- Rule changes are auditable, versioned, tenant-scoped, and can be rolled back.

## Phase 2 — Challenge and Continuous Trust

Inspired by Anubis and FCaptcha: make automation pay a cost, but do not punish
real users, mobile devices, or accessibility tools.

| Feature | What will be implemented | Why it matters |
|---|---|---|
| Replay-safe challenge nonce store | Redis-backed, TTL-bound, single-use challenge state with a degraded single-node mode. | Stops token reuse across requests and nodes. |
| Adaptive proof-of-work | Difficulty is based on policy/risk/repeat behaviour with a strictly capped mobile-safe range. | A fixed trivial puzzle is cheap for a scraper farm; clean visitors should see no work in balanced mode. |
| Progressive challenges | Start with light challenge; escalate only on independent evidence or repeated failed/abusive behaviour. | Raises bot cost without making one noisy signal a denial of service. |
| Periodic trust decay | Re-evaluate high-risk sessions and re-challenge only when policy and behaviour justify it. | One solve should not grant permanent trust. |
| Server-trusted telemetry envelope | Signed, bounded challenge telemetry tied to the issued challenge. | Browser observations become evidence, not a freely editable form field. |
| Challenge accessibility and recovery | Clear fallback, expiry handling, retry limits, and telemetry for challenge failure rate by browser class. | A bot blocker that locks out real customers fails the product mission. |

Not planned: browser-crash tricks, invasive prototype hooks, or expensive
always-on heavyweight WASM challenges. They create unacceptable false-positive,
device-cost, and maintenance risk.

## Phase 3 — Independent Detection Layers

These features are introduced in shadow mode first, with measured false-positive
rates before they can change an enforcement decision.

| Layer | Feature | Main threat closed |
|---|---|---|
| Browser integrity | Carefully validated automation/CDP probes, browser-client-hint consistency, and real-browser carve-outs. | Basic headless and some stealth automation that server-only signals miss. |
| Behaviour | Touch-safe interaction timing, navigation cadence, and session continuity; no raw mouse recording. | Real-browser automation such as patched Playwright/Scrapling. |
| Asset fidelity | Correlate HTML navigation with expected CSS/JS/font/image subresource behaviour per session. | Scrapers that load documents while skipping normal browser resource behaviour. |
| Request graph | Referrer/navigation-chain consistency, enumeration patterns, API pagination patterns, and endpoint-aware velocity. | Slow crawlers and API abuse that evade a simple per-second limit. |
| TLS/HTTP intelligence | Maintained JA4 browser profiles; later, measured HTTP/2/HTTP/3 fingerprint work. | Non-browser protocol impersonation and rotating-IP scraper fleets. |
| IP context | Optional datacenter/Tor/ASN/geo context, used as a corroborating weight only. | Cheap cloud-hosted automation, never a geography blocklist. |
| Session consistency | Timezone/language/browser/connection consistency, with privacy-browser and mobile carve-outs. | Mismatched proxy/fingerprint automation pipelines. |
| Deception intelligence | Tenant-scoped, TTL-bound honeypot propagation between nodes and safe decoy feedback. | DOM-walking and content-ingesting automation. |

Every signal includes:

- exact evidence name and human explanation;
- weight/action policy and why it is not duplicate evidence;
- privacy/data-retention bounds;
- known-good browser, mobile, accessibility, VPN, corporate-network, and
  crawler tests;
- a shadow-mode measurement period before enforcement.

## Phase 4 — Fingerprint and Agent Intelligence

This is the long-term moat: maintained intelligence, not merely detection code.

| Feature | What will be implemented | Why it matters |
|---|---|---|
| Browser fingerprint registry | Curated, versioned profiles of known browser JA4 families with source, confidence, expiry, and review metadata. | Fingerprints evolve continually; unknown remains neutral while verified knowledge becomes useful. |
| Safe intelligence distribution | Signed/versioned data feed, staged rollout, rollback, per-node cache cap, and freshness metrics. | A bad intelligence update must not block every browser family. |
| Verified agent policy | Per-tenant actions for verified search and AI agents: allow, rate-limit, challenge, deceive, or block. | Customers need more than allow-all/block-all for crawlers and AI agents. |
| Web Bot Auth support | Implement the relevant signed HTTP-message verification flow after standards/interop validation. | Moves agent trust beyond a spoofable User-Agent string. |
| Good-bot verification service | Bounded reverse/forward DNS verification plus operator-defined monitor verification. | Protects SEO and trusted automation without creating a bypass. |

## Phase 5 — Durable Multi-Tenant Product Backend

The proxy can only become a hosted product after this layer exists.

| Feature | What will be implemented | Why it matters |
|---|---|---|
| Durable tenant model | Tenant, domain, origin, policy, plan, API credential, and status tables with migrations. | In-memory configuration cannot safely run a paid multi-customer service. |
| Domain onboarding | Domain ownership verification, origin validation, status lifecycle, and audit records. | Adding a domain is an authorization boundary. |
| Automated certificates | ACME issuance/renewal with rate limits, storage, rollback, and alerting. | TLS termination is required for JA4 and cannot be manual at SaaS scale. |
| Usage metering | Request, byte, challenge, and storage counters scoped to tenant/plan. | Hosted traffic has a real cost and needs visible, fair limits. |
| Durable evidence | Retention-configured decision history, aggregated analytics, secure export, and deletion controls. | Customers need explainability beyond a process restart. |
| Audit trail | Policy, domain, credential, and user actions recorded with actor/time/tenant. | Required for support, incident response, and enterprise trust. |
| Customer authentication/authorization | Authenticated control-plane APIs with tenant membership, roles, sessions, and CSRF/rate limits. | A tenant selector or query parameter must never decide access. |

## Phase 6 — Operations, Reliability, and Abuse Resistance

| Feature | What will be implemented | Why it matters |
|---|---|---|
| Metrics | Request-path latency p50/p95/p99, action counts, Redis health, origin errors, challenge completion, and per-tenant usage. | We cannot tune protection or promise an SLA blind. |
| Health/readiness | Separate liveness, dependency readiness, configuration freshness, and certificate health. | Prevents routing traffic to an unhealthy edge. |
| Capacity limits | Bounded request bodies, connection/handshake limits, tenant quotas, and backpressure. | Attackers must not turn one request into unbounded CPU, memory, or network cost. |
| Origin protection | Allow origins to trust only HakaiShield, mTLS or signed edge identity where appropriate, and safe retry rules. | A public origin bypasses the entire protection layer. |
| Soak/adversarial suite | Sustained load, Redis/Postgres outage, slowloris, malformed TLS, replay, rotating IP, real browser, and real automation scenarios. | A request-path product must prove stability, not only unit-test behaviour. |
| Release safety | Feature flags, shadow rollout, per-tenant canary, rollback, schema migration safety, and intelligence-feed rollback. | Detection changes must be reversible under real customer traffic. |

## Reference-Repos to HakaiShield Mapping

| Reference | Techniques worth adapting | HakaiShield destination |
|---|---|---|
| Anubis / Nexus | Policy-driven progressive challenges, operational challenge handling, robust allow/challenge/block controls. | Phases 1 and 2. |
| FCaptcha / Horizon | Replay-aware challenge lifecycle, adaptive proof-of-work, verified agent concepts. | Phases 2 and 4. |
| bot-signal / Vertex | Evidence weighting, browser/behaviour categories, explicit false-positive carve-outs. | Phase 3. |
| Brotector / Quantum | Regression-test catalogue for automation tells only. No crash or visitor-hostile techniques. | Phase 3 test lab. |
| go-away / Zenith | Go policy conditions/actions, secure state/cookie practices, proxy operational ideas. | Phases 0, 1, and 2. |

## Implementation Order

1. Finish Phase 0 correctness/security foundation.
2. Add the Phase 1 policy and immutable decision model.
3. Rebuild the challenge lifecycle in Phase 2 on that policy model.
4. Introduce Phase 3 signals one by one in shadow mode.
5. Build Phase 5 hosted-product boundaries in parallel with real customer
   onboarding needs; do not expose a fake control plane before its backend
   exists.
6. Build Phase 4 intelligence and Phase 6 operational depth continuously,
   driven by real traffic and adversarial regression data.

## Definition of Done for Each Feature

A feature is complete only when all apply:

- threat, current gap, and expected value are documented;
- Go unit/integration tests cover legitimate, malicious, malformed, failure,
  and boundary cases;
- relevant tests fail when the protected behaviour is deliberately mutated;
- tenant isolation, resource bounds, timeout, and failure behaviour are
  reviewed;
- request-path latency/cost impact is measured where relevant;
- shadow mode is used before risky enforcement changes;
- API/evidence/dashboard contracts and project docs match the code;
- `go test ./...`, `go vet ./...`, build, formatting, race checks where the
  environment supports them, and CI linting pass.

## Explicitly Avoided

- Copying code from the reference repos or removing attribution if any external
  code is ever deliberately adopted.
- Blocking on a single weak browser/client-side signal.
- Global JA4 or IP blocklists derived from one tenant's traffic.
- Client-reported behavioural values that reduce risk without server-side
  validation.
- Browser crashes, accessibility-hostile traps, or deceptive data for
  ambiguous traffic.
- Heavy per-request external lookups, unbounded maps/goroutines, and
  unmeasured HTTP/2/HTTP/3 work in the live request path.
