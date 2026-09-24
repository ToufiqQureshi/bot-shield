# Upcoming Enterprise Innovations

**Pilot status (2026-09-24):** Phase 0/1/2 core code exists; a first Phase 3
client-hint consistency candidates now record shadow-only evidence. Passed
sessions now run detection on each request after the first challenge. Tenant
counter isolation and challenge replay/deployment hardening are in the pilot
branch. Real traffic precision/recall, live server/domain/TLS, durable evidence,
and multi-node resilience remain open. See `CLIENT_PILOT_RELEASE.md`; do not
advertise a 70–80% catch rate without a measured, labelled traffic sample.
The pilot dashboard now exposes managed setup only: unverified self-service
domain creation, misleading live-policy controls, and unimplemented billing
claims are not available to customers. Automated ownership proof, certificate
issuance and customer activation remain item 21 below.

What's built, what's next, and why — in that order. Not a wishlist:
every item targets a real evasion technique seen in the field
(Patchright, Scrapling, curl_cffi impersonation clients, plain
scripted HTTP clients). `CLAUDE.md` has the engineering rules every
item must meet before it counts as done.

## What hakaishield is

> **The inline layer that decides which automated clients reach a
> site — and proves why it decided that.**

Read that twice before adding anything to this list, because the
obvious alternative framing is wrong and was tried:

| ❌ What we are NOT | ✅ What we are |
|---|---|
| "A cheaper DataDome" | A hosted service doing **inline TLS/JA4 scoring** |
| Competing on price | Competing on **what it can prove** and **who it serves** |
| Blocking bots | Governing **agents** — allow, rate-limit, deceive, log |
| Reacting to bad IPs | Scoring the **first request**, no prior sighting needed |
| Software you install | A **CNAME away** — nothing for the customer to run |

**How it's sold (changed 2026-09-16).** hakaishield is a **hosted
service we run**. The customer points a CNAME at us and installs
nothing. Self-hosting still exists, but as a priced-up **Enterprise**
option for customers who cannot send traffic to our cloud — it is not
the default and is not to be built yet. See `docs/DECISIONS.md`,
"Pivot: hosted SaaS is the product", for what that costs us
(bandwidth is now our bill, and our downtime is their site down) and
what it buys us.

**Why we still don't compete on price.** The market floor is $0 —
CrowdSec, SafeLine and Coraza are free, and Cloudflare has a free
tier. "Cheaper bot detection" is answered with "CrowdSec is free,"
and that argument cannot be won. What *cannot* be answered that way:
CrowdSec parses **logs** (reactive, needs a prior sighting);
hakaishield reads the **live ClientHello** and scores the first
request. Full numbers and sources: `docs/RESEARCH.md`, market scan
2026-09-16.

**Who pays for this** (so a feature can be judged against a buyer,
not against a competitor's feature list):

- Sites where bots are a **revenue leak, not an annoyance**:
  pricing-sensitive e-commerce, ticketing/booking inventory, job
  boards and classifieds, usage-billed APIs. **This is now the
  primary segment.**
- Teams who **cannot** send traffic to someone else's cloud
  (GDPR/DPDP, regulated sectors). Their alternative to us is not
  DataDome — it's nothing. They are now the **Enterprise** tier, not
  the default customer.

**Still true, and unchanged:** multi-layer scoring beats any single
clever check, because a single check is exactly what stealth tools
are built to defeat (`CLAUDE.md` Section 6).

**Out of scope, on purpose:** anything whose real purpose is helping
automation evade detection, or collecting more user data than a
detection decision requires.

---

## Product direction: multi-signal scoring proxy

```text
Client Request
     │
     ▼
[Fingerprint layer]   → TLS/JA4, HTTP/2, header order
     │
     ▼
[Behavioral layer]    → timing, mouse/scroll entropy (via JS snippet)
     │
     ▼
[Consistency layer]   → IP/timezone/locale/session coherence
     │
     ▼
[Rate/pattern layer]  → request velocity, sequential access patterns
     │
     ▼
[Scoring engine]      → combine signals → risk score
     │
     ▼
[Decision]            → allow / JS challenge / CAPTCHA / block
     │
     ▼
[Dashboard]           → client sees what happened and why
```

No single layer is the whole product — see `CLAUDE.md` Section 6.

**Shape:** one product — a proxy + dashboard **we operate**, that the
customer reaches by pointing DNS at us. No code and nothing to install
on their end. Internally, each layer above is its own small Go
package, purely for code clarity (easier to test and fix), not as a
reusable library for outside use. The same binary is what an
Enterprise customer runs themselves, so no layer may assume we are
always the operator.

---

## Done

- [x] Project scaffolding, engineering rules (`CLAUDE.md`), this roadmap.
- [x] **1. Reverse proxy skeleton** (`proxy/proxy.go`, `cmd/hakaishield`)
      — a stdlib `httputil.ReverseProxy` that forwards every request
      to the configured origin unchanged, plus a `hakaishield` binary
      with graceful shutdown on SIGINT/SIGTERM. Verified with unit
      tests and a real end-to-end run (local origin + proxy + curl).
      Known gap: no timeout/error handling yet for a dead or slow
      origin (proxy just returns Go's default 502) — that belongs
      with item 2 (HTTP fetcher) / retry work, not this skeleton.
- [x] **2. TLS/JA4 fingerprinting** (`proxy/fingerprint.go`,
      `proxy/capture.go`) — `cmd/hakaishield -tls-cert`/`-tls-key` makes
      hakaishield terminate TLS itself, capture each connection's raw
      ClientHello, turn it into a JA4 hash, and forward it to the
      origin as `X-HakaiShield-JA4`. Tested against known-good JA4
      vectors, a real end-to-end TLS handshake + proxied request, a
      bad/garbage handshake (must not hang or crash the listener), and
      `-race`. Verified with a real binary run (openssl self-signed
      cert + curl through hakaishield). Plain HTTP (no `-tls-cert`) still
      works unchanged for local dev.
      The listener hands net/http a real `*tls.Conn` and lets it run
      the handshake, so the handshake timeout, error handling, accept
      retries and panic recovery are the standard library's, not
      hand-written (see `DECISIONS.md`).
      Known gaps: HTTP/2 fingerprinting is not built — the capture
      listener only negotiates HTTP/1.1 for now (see `DECISIONS.md`).
      A client that splits its ClientHello across two TLS records
      still handshakes fine but can't be fingerprinted — reported as
      `JA4Unreadable` rather than silently unfingerprinted, so it's
      visible, but proper multi-record reassembly is still open (see
      `RESEARCH.md`).
- [x] **3. Basic header/UA consistency check** (`proxy/useragent.go`)
      — `UAMismatch(ua, ja4)` flags a request whose User-Agent claims
      a real browser (Chrome/Firefox/Safari/Edge) but whose TLS
      handshake says otherwise: negotiates ancient TLS 1.0/1.1 (no
      current real browser does), or triggers the `JA4Unreadable`
      fragmentation evasion (real browsers never fragment their
      ClientHello). Forwarded to the origin as
      `X-HakaiShield-UA-Mismatch: true`, stripped from the incoming
      request first so a visitor can't set it themselves. A UA that
      openly declares itself a crawler (Googlebot etc.) is exempted —
      that's not a lie. Tested: known-good (real Chrome + modern TLS),
      known-bad (claimed browser + old TLS / unreadable handshake),
      and borderline cases (curl, empty UA, empty/garbage JA4,
      crawler UA containing "Chrome"), all mutation-checked. A real
      end-to-end test proves the flag reaches the origin through the
      actual proxy wiring, not just the pure function.
      Scope, deliberately narrower than the literal ROADMAP wording
      ("real client family"): this does not classify JA4 into "this
      is really Chrome 120" — that needs a maintained database of
      known-browser fingerprints, which is a real ongoing research
      cost (see `DECISIONS.md`), not a one-line addition. That
      database is now item 19, and the 2026-09-16 decision entry
      reframes its cost as the product's moat rather than a reason to
      defer it. What's built
      catches the concrete, well-defined lie (claims modern browser,
      handshake proves otherwise) without that database.
      This is a **signal only** — nothing blocks yet (`CLAUDE.md`
      Section 6); it's an input for item 5's scoring engine.
- [x] **4. JS challenge** (`proxy/challenge.go`) — `Challenge` issues a
      signed, single-use puzzle: the page's JS must compute the SHA-256
      of a server-issued nonce (proves it parsed the page and hashed
      something we chose, not a cached/replayed answer) and submit a
      `canvas.toDataURL()` render (raises the bar toward needing a real
      browser engine, not just an HTTP client). A plain scripted client
      that never runs JS never reaches the verify step at all. On
      success, sets a signed, HttpOnly/Secure/SameSite=Lax cookie
      (`X-HakaiShield-Passed`) and redirects back to the exact page the
      visitor originally asked for. Token and cookie are HMAC-signed
      with a random in-process secret, checked with
      `subtle.ConstantTimeCompare`; the redirect target is validated by
      `safeRedirectPath` against open-redirect. Wired into
      `cmd/hakaishield` at `/__hakaishield/challenge` and
      `/__hakaishield/verify` — reachable today for manual testing, not
      yet triggered automatically for real visitors (that decision
      belongs to item 5's scoring engine, which doesn't exist yet).
      Tested: full round-trip against real end-to-end flow (extract
      nonce/token from the rendered page, compute the real answer,
      verify), wrong answer, missing/malformed canvas proof, tampered
      token, expired token, a token signed by a different `Challenge`
      instance, forged/expired passed-cookie, oversized POST body,
      wrong HTTP methods, and open-redirect payloads against
      `safeRedirectPath`. Mutation-checked: removed the expiry check,
      the canvas-proof check, and the answer comparison one at a time —
      each turned the matching test red with the exact case named, then
      reverted. Also run as the real compiled binary against a real
      local origin: full challenge solve over actual HTTP, confirmed
      redirect + signed cookie, confirmed plain proxy passthrough on
      `/` still works unchanged. Independent `/security-review`
      (background agent) checked XSS/template-escaping, open-redirect
      bypasses, and signature/cookie forgery — no findings.
      Known gaps (see `docs/DECISIONS.md`):
        - Not wired to automatic triggering — nothing decides *who*
          gets challenged yet; that's item 5.
        - The canvas proof is a client-reported string, not a verified
          render — spoofable by a bot that specifically studies
          hakaishield. Documented, accepted limitation, same class as
          item 3's JA4-database gap.
        - Redis now shares nonce consumption across nodes, but the signing
          secret still must be shared through
          `-challenge-secret`/`HAKAISHIELD_CHALLENGE_SECRET` so issued tokens
          and passed cookies verify everywhere.
        - Signing secret is generated fresh per process, in memory
          only — a restart or a second instance invalidates
          outstanding challenges/cookies. Fine for the current
          single-process v1. Redis nonce sharing is now built; this remaining
          warning is specifically about sharing the signing secret across nodes.
- [x] **5. Scoring engine v1** (`proxy/score.go`, `proxy/guard.go`) —
      this is the first thing in the codebase that actually acts on a
      signal instead of just labeling it. `Score(ja4, ua)` combines
      the JA4-fragmentation signal (item 2) and the UA-mismatch signal
      (item 3) — two different layers, even though a fragmented
      handshake is one of UAMismatch's own inputs, see
      `docs/DECISIONS.md` for why that's not double-counting.
      `Decide(score)` maps the score to allow/challenge/block with
      fixed thresholds (50 for challenge, 100 for block) — a single
      mid-strength signal only ever earns a challenge, never a block
      on its own (`CLAUDE.md` Section 6). `Guard` wires this to a live
      request: a visitor who already solved the JS challenge (item 4)
      is forwarded straight through with no re-scoring; otherwise
      Guard scores the request and either forwards it, serves the
      challenge in its place, or returns 403 — never proxying an
      unscored or blocked request to the origin. Wired into
      `cmd/hakaishield` as the real handler for `/` (the standalone
      `/__hakaishield/challenge` and `/__hakaishield/verify` routes stay
      reachable directly for manual testing).
      Tested: known-good browser traffic (no signal fires → allowed),
      known-bad traffic (both signals fire → blocked before reaching
      the origin), a single mid-strength signal (fragmented handshake,
      non-browser UA → challenged, origin never reached), and a real
      passed-cookie bypassing what would otherwise score a block — all
      four as real end-to-end tests through actual TLS handshakes (the
      same `fragmentingRelay`/`startCapture`-style harness items 2–4
      already built), not just the pure `Score`/`Decide` functions in
      isolation. Plus unit tests for every `Score`/`Decide` boundary
      and `Decision.String()`. Mutation-checked: removed the
      passed-cookie bypass in `Guard` (the block/challenge tests that
      should have still worked stayed correct, and the bypass test
      itself went red naming the wrong status code), and removed the
      UA-mismatch weighting from `Score` (three tests — the pure score
      test, the combined-signal score test, and the real end-to-end
      block test — all went red), then reverted both and confirmed the
      suite was back to green. Real compiled binary run: plain HTTP
      (no TLS) passthrough still works unchanged (fail-open — no
      fingerprint to score, so nothing is ever penalized for a
      connection hakaishield can't examine).
      Known gaps (not deferred without reason — `CLAUDE.md` Section
      17):
        - Thresholds (50/100) and weights (50/50) are a reasoned
          starting point, not tuned against real traffic — there is no
          real traffic yet. Revisit once this runs in front of an
          actual client.
        - No per-client configuration (`docs/ROADMAP.md` item 11) —
          every deployment gets the same thresholds today.
        - Item 6 (client-side automation-tool probe) isn't built, so
          it isn't a scoring input yet — only items 2 and 3 are.
      `proxy/stats.go` (added same day, same item): `Guard` counts
      every decision, and `GET /api/v1/dashboard/stats` serves
      `{total_requests, passed, challenged, blocked}` — the exact
      contract agreed for the
      dashboard skeleton (ROADMAP item 12) to consume. In-memory
      counters only (resets on restart — same class of gap as
      Challenge's in-memory secret, see `docs/DECISIONS.md`); durable
      analytics need the planned Postgres store. CORS is open
      (`Access-Control-Allow-Origin: *`) on this one endpoint since it
      only exposes aggregate counts, not per-visitor data, so the
      dashboard's separate dev server can call it directly.
      Tested: JSON shape and exact field names (not just that the Go
      struct round-trips through itself — the contract is the raw JSON
      keys), wrong-method rejection, CORS header present, and a real
      end-to-end test proving `Guard` actually increments the counters
      it claims to, not just that `Decide()` picked an outcome.
      Mutation-checked: removed the allow-path counter increment →
      the end-to-end stats test went red naming the wrong count.
      Verified against the real compiled binary: two real plain-HTTP
      requests through `Guard`, then `curl` the stats endpoint —
      returned `{"total_requests":2,"passed":2,"challenged":0,"blocked":0}`,
      exactly matching the contract.
- [x] **6. Client-side automation-tool probe** (`proxy/challenge.go`) —
      scoped narrower than the literal wording, deliberately: rather
      than injecting a JS snippet into every proxied origin response
      (a real HTML-rewriting feature this codebase doesn't have and
      hasn't decided to build — no existing infra for it, and it's a
      meaningfully large piece: charset handling, compressed
      responses, CSP interaction), the automation check runs inside
      the JS challenge page (item 4) — the one place hakaishield already
      serves its own JS to a visitor's browser. The page's JS checks
      `navigator.webdriver` and known Selenium/PhantomJS/Nightmare.js
      globals; `handleVerify` fails the challenge (no passed cookie)
      if any fire, even when the sha256/canvas checks passed. This
      targets exactly the gap `docs/RESEARCH.md` already named: a
      stock automation framework drives a real browser, so it passes
      every fingerprint/render check, but leaves these markers behind.
      Tested: a real end-to-end verify call with a correct answer and
      valid canvas proof but `automation=true` must still fail and
      must not set the passed cookie. Mutation-checked: removed the
      automation check → the new test went red, confirmed, reverted.
      Known gaps (documented, not silently accepted):
        - Patchright (patched Playwright) specifically removes
          `navigator.webdriver` and similar artifacts — this check
          does not catch it, by design of the attacker tool, not an
          oversight here. `docs/RESEARCH.md` already named
          behavioral/timing signals (item 7) as what would catch that
          tier; this is unchanged.
        - Only runs when a visitor reaches the challenge page — traffic
          scored `DecisionAllow` (score 0) never executes this check.
          Extending it to all traffic needs the HTML-injection feature
          this scope deliberately avoided; revisit if/when that's
          actually decided as a roadmap item.

- [x] **12a. Decision evidence trail** (`proxy/evidence.go`,
      `proxy/guard.go`, `proxy/score.go`) — every Guard decision is
      recorded with the time, the JA4, the names of the signals that
      fired, the score and the outcome, and served newest-first from
      `GET /api/v1/dashboard/evidence` (`?limit=N`).
      Storage is a fixed 1000-entry ring buffer with a 24h retention
      window, so memory can't grow with request volume and visitor
      records don't outlive answering a complaint (`CLAUDE.md`
      Sections 9 and 18). In-memory only — resets on restart, same
      limitation as `Stats`.
      `Score` and the signal names now come from one shared `checks`
      table, so a score and the explanation shown for it can't
      disagree; adding a signal to one and forgetting the other used
      to be a live bug waiting to happen.
      A request let through on a solved-challenge cookie is recorded
      as `challenge_solved`, not as a clean score-0 allow — recording
      it the other way would have made the trail claim a visitor
      looked clean when they actually carried both bad signals.
      **Security:** the endpoint returns per-visitor fingerprints and
      would tell a caller whether their own fingerprint is being
      flagged, so it requires `Authorization: Bearer` against
      `-evidence-token`, is not mounted at all when that flag is
      unset, and (unlike `/stats`) never sets wildcard CORS. An empty
      configured token denies everyone rather than disabling the check.
      Tested: record/order/capacity/retention/limit, token missing,
      token wrong, token unset, non-GET, CORS header absent,
      concurrent writes under `-race`, plus end-to-end assertions that
      the record Guard writes matches what really happened to a real
      allowed/challenged/blocked/cookie-bearing request. Nine
      mutations were run against these tests and all nine went red —
      listed in `docs/PROGRESS.md` 2026-09-16. Verified against a real
      compiled binary (401 without token, 404 when disabled, real JSON
      with it).
      Known gaps: no history beyond the 1000-entry window, no
      filtering or search, the dashboard doesn't read this endpoint
      yet, and the record carries no request path — so correlating a
      specific complaint still means matching on time and fingerprint.

- [x] **18. Shadow mode** (`proxy/mode.go`, `proxy/guard.go`,
      `proxy/stats.go`, `dashboard/`) — `hakaishield -mode shadow` scores
      and records every request exactly as enforce mode does, then
      forwards all of it to the origin. Nothing is blocked or
      challenged, so a client can point real traffic at hakaishield with
      zero risk to their customers.
      `-mode` accepts only `enforce` (default) or `shadow`; anything
      else refuses to start rather than defaulting quietly.
      **Visible in four places, because the one real danger here is a
      client believing they are protected when they are not:** a
      startup log line, `mode`/`enforcing` on every `/stats` response,
      `enforced: false` on every evidence record, and in the dashboard
      both a status badge ("Shadow mode — not enforcing") and a banner
      over the numbers. In shadow mode the stat labels themselves
      change to "Would block" / "Would challenge" / "Would pass" —
      "Blocked: 500" when nothing was blocked is the worst thing this
      product could say.
      Tested: shadow never blocks, shadow never serves the challenge
      page, enforce still stamps `Enforced: true`, `ParseMode` rejects
      unknown values, `/stats` reports the mode, and the dashboard
      shows the right labels and badge in each mode and claims nothing
      when the backend is down. Eight mutations run across Go and the
      frontend, all eight went red (`docs/PROGRESS.md` 2026-09-16).
      Verified against a real binary in both modes and in a real
      browser against a real backend.
      **Not done — the traffic report.** There is no "here is your two
      weeks of traffic" summary yet: no history (the trail is a
      1000-entry in-memory ring), no top-offenders view, no export.
      That is the part a client is actually shown, and it needs item
- [x] **19. Enterprise Hardening & Production Traffic Governance** (`pkg/config`,
      `pkg/signals/goodbots.go`, `pkg/core/proxy.go`, `pkg/core/guard.go`) —
      Introduced adaptive policy modes (`PolicyBalanced` vs `PolicyStrict`):
      `balanced` allows clean traffic (score 0) direct zero-latency access to the origin,
      while `strict` enforces mandatory invisible challenge execution.
      Integrated automated reverse+forward DNS verification engine for search engine
      crawlers (`IsVerifiedGoodBot`) with 6-hour caching to ensure zero SEO penalty.
      Upstream proxy connections now use production-tuned pooled `http.Transport`
      (1000 max conns, 200 per host, 90s idle timeout, 15s response header timeout)
      and custom structured 502/504 error handlers. Added `/__hakaishield/healthz`
      for load balancer health probes and `BOTSHIELD_CHALLENGE_SECRET` env var for
      multi-instance cluster synchronization. Tested: 100% test coverage across
      unit test suites, fresh build/vet checks, and real-world headful Patchright
      adversarial tests.

---

## P0 — MVP (prove the core idea works)

- [x] ~~**1. Reverse proxy skeleton**~~ — done, see "Done" section above.
- [x] ~~**2. TLS/JA4 fingerprinting**~~ — done (HTTP/2 fingerprint part
      still open), see "Done" section above.
- [x] ~~**3. Basic header/UA consistency check**~~ — done (narrower
      than full HTTP2-family verification, no fingerprint database
      needed), see "Done" section above.
- [x] ~~**4. JS challenge**~~ — done (mechanism only; not yet wired to
      automatic triggering), see "Done" section above.
- [x] ~~**5. Scoring engine v1**~~ — done (fixed thresholds, not yet
      per-client configurable — that's item 11), see "Done" section
      above.
- [x] ~~**6. Client-side automation-tool probe**~~ — done, narrower
      than the literal wording (checks known automation-framework
      globals inside the existing JS challenge page, not a
      site-wide-injected snippet — see "Done" section above and
      `docs/DECISIONS.md` for the scope reasoning).

## P1 — makes it meaningfully harder to bypass

**Phase 2 challenge & continuous trust status (2026-09-23):** adaptive
proof-of-work (difficulty 1–3 banded by risk score), progressive escalation
via a signed attempt cookie (never per-IP), trust decay on the passed cookie
(30/15/5 min by difficulty solved), and bounded client telemetry with
browser-class counters are built in `pkg/challenge` and wired from
`core.Guard`'s score. Difficulty lives only inside the signed token, so it
cannot be lowered by a client. The canvas proof is decoded and checked for
dimensions and nonblank pixels (item 28 follow-up). A scripted client can
still forge a PNG, so solves remain unverified candidate observations.
The challenge also records four browser-reported candidates as shadow-only
evidence after valid solves; they require real client traffic review before
any policy promotion. The initial proxy shadow mode never serves challenges,
so these four candidates will have no real-visitor samples during that phase.
They need a separately reviewed challenge cohort before promotion. DNSBL/IP
reputation is deferred for the low-cost pilot;
no provider or request-path query is configured. Revisit after labelled client
traffic shows a measurable gap that an IP list can close.

**Phase 1 backend policy status (2026-09-23):** versioned tenant snapshots,
ordered rules, preview, rollback, shadow evidence/summary, and gated
enforcement code are implemented. Legacy dashboard rules remain account-wide
and shadow-only. Live false-positive review, durable cross-node shadow
metrics, and a tenant policy dashboard editor remain before calling this a
production self-service feature. See `BACKEND_IMPLEMENTATION_PLAN.md` and
`CLIENT_PILOT_RELEASE.md`.

- [~] **25. Learned scoring weights (`pkg/decide`)** — the scoring engine's
      weights (item 5) are hand-chosen guesses. `pkg/decide` fits them to
      labelled traffic instead: logistic regression over the same checks,
      typed decision plus estimated probability, confidence, and a
      per-feature contribution breakdown. Pure Go, in-process, 14 ns and zero
      allocations per request, no new dependency.

      **Status: pipeline done, enforcement deliberately not.** The model runs
      in shadow only (`-model`), recording what it would have decided next to
      what the rules actually did. `cmd/hakaishield-train` fits a model from
      labelled traffic in the shape the evidence trail already records.

      **Blocked on verified labels, not inference code.** Challenge solves are
      forgeable human candidates; honeypot hits are automated candidates,
      with possible prefetch/accessibility false positives. Operator-reviewed
      labels are required before claiming model quality. Labelling from the
      current rule score would only teach the model to repeat the guesses it
      exists to improve on. The next step is item 26, not more model code.

- [ ] **27. Sideband decision API for high-volume customers** — a mode where
      the customer's own CDN or nginx calls hakaishield for a verdict instead of
      routing their traffic through us.

      **Why it will be needed, with the number.** As a full reverse proxy we
      pay egress on every byte of every response. At AWS's $0.09/GB that is
      about $81/month at 10M requests (100KB average response) and roughly
      $6,900/month at 1B. This is exactly how DataDome and Akamai avoid the
      problem: their modules make a sideband call with request metadata and the
      CDN serves the content, so their infrastructure never touches the page
      body.

      **Not a replacement for the proxy.** The proxy model is why our evidence
      is better — we see the whole request, not a summary someone else chose to
      send. This is an option for customers whose volume makes proxying
      uneconomic, and a deployment mode for customers who will not reroute DNS.

      The threshold should be measured against a real traffic profile, not
      guessed. See `docs/DEPLOYMENT.md` §4 and `docs/RESEARCH.md` (2026-09-22).

- [~] **26. Label pipeline for learned scoring** — capture labelled traffic
      that item 25 can actually train on. Persisting fired checks plus a
      label, tenant-scoped and bounded, is the prerequisite for ever
      enforcing a learned model.

      **Read `docs/LEARNED_SCORING.md` before starting.** It works the
      whole thing through, and two of its findings contradict the obvious
      plan:

      - A **solved JS challenge** is a human candidate, but the solve
        arrives on a later request than the one that was scored, so the
        fired vector has to be parked against the challenge nonce
        (`pkg/challenge` already has a Redis `NonceStore`). It must not
        ride in the token — that hands a bot a signed list of the checks
        it tripped.
      - A **verified good-bot lookup is not a usable label**, contrary to
        what this item used to say. `guard.go` forwards verified crawlers
        *before* `signals.Evaluate` runs, so no vector exists — and
        training on it would teach the model to stop crawler-shaped
        traffic, which is a false positive aimed at legitimate bots.
      - A **honeypot trip** works, but `honeypot_trap` must be dropped
        from the vector of any sample it labelled, or the model just
        learns the label back.
      - **Selection bias** is the real trap: under PolicyBalanced only
        score>0 traffic is challenged, so every human candidate comes from
        traffic that already looked suspicious. Pick a correction before
        collecting, not after.

      **Status: candidate collection built, verification and bias correction
      not.** `pkg/labels` stores the two candidate sources behind
      `-collect-labels` with tenant, mask and source, but no IP/UA/path.
      The first honeypot request is captured once. The trainer refuses
      automatic DB candidates unless `-allow-unverified-labels` is
      explicitly set for shadow experiments; curated JSONL remains usable.
      The bounded queue and per-identity cap limit request cost and volume.

      **Still open:** the selection-bias correction (§3 of
      `LEARNED_SCORING.md`) is an unmade product decision, and the parked
      challenge samples are per-process, so behind several nodes a
      visitor challenged on one and verified on another produces no
      label. Retention (`db.DeleteSamplesBefore`) exists but nothing
      calls it on a schedule yet.

      Also see `docs/DECISIONS.md`, "Learned decision weights are a linear
      model over existing signals".

- [x] **28. Validate the submitted canvas PNG** — `validCanvasProof`
      (`pkg/challenge/challenge.go`) decodes a bounded PNG and checks expected
      dimensions and nonblank pixels. Prefix plus filler no longer passes.
      A script can still generate its own PNG and PoW answer, so this does
      not establish that JavaScript ran or that a visitor is human.

      As a challenge that is an accepted trade-off (`DECISIONS.md`): the
      point is to cost a scraper something, not to be unbeatable. **As the
      training-label source for item 25/26 it is a poisoning vector**, and
      that argument was never made when the limitation was accepted. One
      forged `human` label costs one challenge token and one nonce. The
      per-identity cap (5/hour) bounds the rate; rotating IPs defeats it.

      **Implemented:** bounded PNG decoding, expected 300x150 dimensions,
      and nonblank pixels on the verify path. Pixel checks raise the cost of
      a fake form value but cannot prove a real render.

      **Still required:** no model trained on `challenge_solved` samples may
      enforce anything — which is already the rule (item 25), for a
      different reason. See `docs/LEARNED_SCORING.md` 2.1.

- [ ] **29. Label the honeypot trip on the request that trips it** —
      `Guard.ServeHTTP` answers `signals.HoneypotPath` with a 404 and
      returns before `collectLabels` runs (`pkg/core/guard.go`). The
      `honeypot_trap` check only fires on a *later* request from the same
      `(tenant, IP, JA4)` within the 6h TTL, so a bot that trips the trap
      and leaves is never labelled. Nothing measures how often that
      happens.

      The trap request itself carries a perfectly good feature vector —
      its UA, headers and JA4 are real evidence, and the honeypot bit is
      stripped from the sample anyway. Evaluate and record it in the
      `firstTrip` branch, and add `label_honeypot_trip_total` so the yield
      against `label_sample_queued_total` is visible instead of assumed.

      **Owner decision, not a bug:** this changes which traffic the model
      trains on, so it belongs with the selection-bias question in
      `docs/LEARNED_SCORING.md` 3.

- [ ] **19. Known-browser fingerprint database** — a maintained set of
      JA4 fingerprints for real browser builds, refreshed on a
      schedule, so `UAMismatch` can answer *"is this actually Chrome
      120?"* instead of only *"did the handshake look broken?"*
      **This is the commercial moat, not just a signal.** Everything
      else on this list is code, and code gets rebuilt in a fortnight
      by a competent engineer — which is the honest answer to "why
      wouldn't a client just build this themselves?" A database isn't
      code: it's data that goes stale, and a client paying us is
      buying *our* problem of keeping it fresh. Chrome ships roughly
      every 4 weeks and its fingerprint moves with it, so an in-house
      system decays silently — it never errors, it just drifts toward
      allowing everything. Full reasoning and rejected alternatives:
      `docs/DECISIONS.md`, 2026-09-16 "the moat is the fingerprint
      database" entry.
      Item 3 already identified this and deferred it as "a real
      ongoing research cost" — correct as engineering, backwards as
      business. The recurring cost is the asset.
      **Scope this before building it.** The open question is how much
      work a refresh cycle actually is (where fingerprints come from,
      how often, how they're validated). If that turns out bigger than
      one maintainer can carry, that's a strategic problem worth
      knowing early — not a reason to start and stall.
      **False-positive risk, and it's the serious kind:** a stale or
      incomplete database makes every *unlisted* browser look like a
      liar. Niche browsers, older mobile builds and privacy browsers
      are exactly the real users who'd get wrongly flagged, so an
      unknown fingerprint must mean "no opinion", never "suspicious"
      (`CLAUDE.md` Section 8).

- [ ] **7. Behavioral scoring** — mouse movement entropy, click timing,
      scroll velocity, collected client-side. Targets tools that use a
      real browser (defeats fingerprint-only checks) but drive it with
      unnaturally perfect timing.
- [ ] **8. Session consistency check** — IP geolocation vs. declared
      timezone/Accept-Language vs. TLS fingerprint's likely OS. Flags
      automation pipelines that mix mismatched proxy/fingerprint pairs.
- [ ] **9. Rate & pattern anomaly detection** — per-fingerprint and
      per-IP request velocity, sequential/enumerated URL access,
      missing normal referrer chains.
      **Started, not done:** per-IP/per-JA4 velocity is built and now
      asset-aware (navigations vs subresources have separate limits),
      and `crawl_pattern` detects a browser-claiming client walking many
      distinct page paths in a window via Redis HyperLogLog, with a bounded
      fail-open Redis outage circuit — see `docs/PROGRESS.md` 2026-09-21.
      Missing: referrer-chain analysis,
      per-fingerprint request *rate* (not just distinct paths), and the
      caps are reasoned guesses, not tuned against real traffic.
- [ ] **9a. API-aware endpoint rules** — tag endpoints by category
      (login, checkout, listing, generic) in config so item 9's rate
      thresholds differ per category, instead of one global rate limit
      for the whole site. Reuses items 9 + 11's machinery — a config
      field, not a new signal or new package. Competitor gap: see
      `docs/RESEARCH.md`'s 2026-09-15 competitor scan (DataDome's
      stated differentiator).
      **Risk:** wrong category tagging is worse than no tagging — a
      client mislabeling their login endpoint as "generic" gets the
      loose threshold on their most sensitive route, silently, with no
      warning. Needs a sane default (unlabeled endpoint = strictest
      category, not loosest) and validation that catches an empty/
      missing category rather than defaulting quietly.
- [x] **10. Honeypot fields** — invisible form fields/links only a
      blind selector-based script would interact with.
      **Done 2026-09-20:** an `aria-hidden`, `tabindex="-1"`,
      `rel="nofollow"`, `display:none` link injected into deceived HTML
      (`pkg/deception`), with the fetch recorded by `pkg/signals`
      scoped to (tenant, IP, JA4) with a TTL and a hard entry cap.
      Scored at 50, not as a block: see `DECISIONS.md` 2026-09-20 for
      why a lone trip must stay recoverable.
      **Still open:** the trap link only reaches traffic already being
      deceived. Injecting it into challenge pages (so it also catches
      bots that never reach the deceive decision) and propagating trips
      between nodes are both follow-ups.
- [ ] **11. Per-client rules** — each client (site) can tune thresholds,
      allowlist known-good bots (search engine crawlers, uptime
      monitors), and set custom block pages.
- [ ] **11a. Deception mode (decoy response)** — a fifth decision
      outcome alongside allow/challenge/block: for high-confidence-bot
      traffic, forward the request with `X-HakaiShield-Decision: deceive`
      instead of blocking, and let the origin app decide what fake data
      to return (stale price, dummy inventory). hakaishield only signals
      the decision — it never generates or owns the fake data itself,
      keeping this a proxy-layer change, not new business logic.
      Competitor gap: see `docs/RESEARCH.md`'s 2026-09-15 scan — an
      under-offered mitigation even among big vendors.
      **Risk:** this is worse than a false-positive block, not the same
      severity — a wrongly-deceived real customer doesn't just get an
      error, they see *wrong data as if it were real* (a fake price,
      fake stock) and may act on it (place an order, make a decision)
      before finding out. Only wire this to the highest-confidence
      band of the scoring engine (item 5), strictly above the block
      threshold, never as a default action for ambiguous scores. Needs
      its own false-positive tracking in the dashboard (item 12),
      separate from block/challenge counts, before any client turns it
      on for real traffic.
- [ ] **11b. Verified agent policy** — extends item 11's allowlist from
      a yes/no list into a per-agent rule: allow, rate-limit, deceive
      (item 11a), or block, per declared agent. The point is to make
      *"GPTBot is welcome; a scraper wearing GPTBot's User-Agent is
      not"* something a client can actually express — today a
      mid-market site's only options are allow-all or block-all.
      Reuses item 11's config and item 5's decision output; the
      matching evidence (does the TLS/JA4 handshake agree with the
      claimed agent?) is item 3's `UAMismatch` logic, already built.
      **Why now:** Forrester renamed the category to *Bot and Agent
      Trust Management* in 2026; Cloudflare ships pay-per-crawl, RSL
      and Web Bot Auth exist. This is the one part of that shift we
      can build without waiting on anyone else's adoption — see
      `docs/RESEARCH.md` 2026-09-16.
      **Explicitly not in scope:** pay-per-crawl / HTTP 402 billing.
      No major AI lab has adopted it, so it would be a toll booth
      nobody pays at (`docs/DECISIONS.md` 2026-09-16).
      **Risk:** an over-broad allow rule is a bypass with a config
      file. A rule that allows an agent by User-Agent alone must still
      run the fingerprint check, never skip scoring entirely —
      otherwise `CLAUDE.md` Section 6 is defeated by our own feature.

## P2 — product-grade

- [ ] **12. Dashboard** — requests scored, blocked, challenged over
      time; top offending fingerprints/IPs; false-positive report
      button for the client's ops team.
      **Started, not done:** `dashboard/` (React + Vite) has account
      signup/signin (JWT), domain management, mitigation rule
      CRUD, protection settings, live stats
      (`/api/v1/dashboard/stats`), top-offending JA4 fingerprints
      (`/api/v1/dashboard/top-offenders`), and evidence logs
      (`/api/v1/dashboard/evidence-logs`) — verified running
      end-to-end (backend on real Postgres + Redis, frontend `npm run
      dev`, live signup/signin/domain-add against it) — see
      `docs/PROGRESS.md` 2026-09-21 entries. Remaining work, in
      priority order:
      1. **Payments (Stripe).** Blocked on real Stripe credentials —
         nobody has provided an account/API key yet. Needs
         `POST /payment/create-intent`, `POST /payment/webhook`,
         `POST /subscription/upgrade` and Stripe SDK integration.
         `Subscription`/`Payment` pages show an honest "not set up
         yet" notice (fixed 2026-09-21 — they previously showed
         fabricated plan/usage/invoice data and a card form that
         falsely claimed to be processed by Stripe; see
         `docs/DECISIONS.md`) rather than being unwired silently.
      2. ~~**Email (SendGrid/SES).**~~ **Done differently, 2026-09-21:**
         auth (including email verification and password reset)
         moved to Supabase Auth instead of building a SendGrid/SES
         integration — see `docs/DECISIONS.md`'s Supabase-migration
         entry. `backend/pkg/account` and the custom JWT issuer are
         deleted; `pkg/auth` now only verifies Supabase-issued session
         JWTs via that project's JWKS. **Not yet fully verified**: the
         Go backend needs the new Supabase project's DB connection
         string (`-db-url`) from the owner, and one real email needs
         verifying to confirm the full signed-in loop end-to-end —
         both still open, see `docs/PROGRESS.md`.
      3. **Enforce custom mitigation rules.** Rules are real CRUD
         today but have zero effect on live traffic — nothing in
         `pkg/core`/`pkg/signals` reads `mitigation_rules`. No
         external dependency; needs a design decision on how an
         arbitrary JA4/score/path condition composes with the
         existing signal-based scoring in `pkg/signals/score.go`.
      4. **Enforce protection settings.** Same shape of gap as #3:
         `protection_settings` is real CRUD but `pkg/signals/score.go`
         still uses fixed thresholds in code. No external dependency.
      5. **Traffic-over-time chart.** `stats.Stats` is a running total
         only, no time series — the dashboard shows an explicit "not
         available" notice instead of a fake chart. Needs a
         time-bucketed store (e.g. hourly buckets in Postgres or
         Redis) and a retention policy.
      6. **Immediate domain enforcement.** A domain added via the
         dashboard is picked up by `tenant.Store` lazily (on the next
         request or dashboard read that touches it), not the instant
         it's created. Small fix: push it into the in-memory store at
         creation time instead of waiting for the next lookup.
      7. **Multi-domain scoping on `/dashboard/top-offenders` and
         `/dashboard/evidence-logs`.** Both implicitly use the
         caller's first-created domain; no `?domain=` parameter yet,
         and the frontend has no per-domain selector wired to these
         two calls (though `Layout`'s domain switcher already exists
         and could drive one).
      8. **SIEM integrations** (Datadog/Splunk/S3/webhooks) — entirely
         unbuilt. Needs a vendor priority decision, then that vendor's
         API/SDK and credentials. The dashboard shows a "not built"
         notice in place of the old fake toggle state.
      9. **WAF** (SQLi/XSS detection, configurable rate limiting) — a
         separate, large detection feature; the dashboard already
         marked it "Coming Soon" before this session and still does.
      10. **False-positive report button** — never built.
      11. **`npm audit`: 2 moderate CVEs** (react-router-dom open
          redirect, uuid buffer bounds) — fix requires a breaking
          major-version bump on code with no test coverage; not
          force-upgraded blind (see `docs/PROGRESS.md` 2026-09-21
          "Brought the untracked dashboard frontend into the repo").
- [x] ~~**12a. Decision evidence trail**~~ — done, see "Done" section.
- [ ] **13. Real-time scoring API** — for clients who want to call
      hakaishield from their own app instead of routing all traffic
      through the proxy.
- [ ] **14. Deployment story** — Docker image + simple config, so a
      client can stand this up in under an hour (this is the actual
      competitive edge over enterprise tools' weeks-long onboarding).
- [ ] **15. Metrics/observability** — latency added per layer, decision
      breakdown, exportable to Prometheus.
      **Started, not done:** aggregate bearer-protected counters now cover
      JWKS refresh/failures, unknown-kid rejects, Goodbot DNS budget rejects,
      Redis circuit opens/probes/skips, origin proxy errors, malformed client
      IP forwarding, malformed Host, unknown Host, and SNI/Host mismatch.
      Missing: latency histograms, per-tenant usage metrics, Prometheus
      export, readiness semantics, and production p95/p99 measurements.
- [ ] **16. Long-running soak test** — sustained adversarial traffic
      simulation proving memory/latency stay stable over hours.
- [ ] **17. Pricing/tiering model** — finalised once real traffic and
      a real bandwidth bill exist, not before. What is decided: no
      free tier (owner's call), a 14-day trial without a card, and
      **every plan carries a bandwidth/request cap** — without one,
      a single large customer erases the margin on ten small ones now
      that their traffic is our bill. Pricing is never argued as
      "cheaper than DataDome"; a discount pitch loses to free software
      (`docs/DECISIONS.md` 2026-09-16).
      Working shape, all numbers unvalidated: a hosted plan around
      **$200/mo** with a few domains, a larger hosted plan above it,
      and **Enterprise** (self-hosted, sales-led, invoiced) priced
      *above* both — never below. What is not decided: cap sizes,
      metering unit, overage rates, and every price.
- [x] ~~**18. Shadow mode + traffic report**~~ — the mode is done; the
      report is not. See "Done" section.

## P0-SaaS — required before anyone can pay us

Added 2026-09-16 with the hosted-service pivot. None of this is
detection work, and all of it blocks revenue: today's binary serves
one origin from one config file and knows nothing about customers.

- [ ] **20. Multi-tenancy** — a tenant boundary on every record and
      every query. `Stats`, `Trail` and the origin target are global
      today; each becomes per-customer, loaded from storage rather
      than a flag.
      **This is the highest-risk item in the entire roadmap.** One
      query that crosses a tenant boundary shows customer A the
      fingerprints and traffic of customer B. That is not a bug, it is
      the end of the company. Every storage access needs the tenant in
      the key, and a test that proves a second tenant's data is
      *never* returned — not just that the first tenant's is.
- [ ] **21. Domain onboarding + automatic certificates** — a customer
      adds a domain, proves they own it, points a CNAME at us, and we
      issue and renew its certificate over ACME with no human step.
      Without this, JA4 doesn't work — we can't read a ClientHello we
      don't terminate — so this gates the entire product, not just
      convenience.
      **Risk:** domain verification is an authorisation boundary. If
      someone can add a domain they don't own, they get our cert and
      our proxy pointed at a site they don't control.
- [ ] **22. Usage metering + bandwidth caps** — count requests and
      bytes per tenant, enforce the plan's cap, and make the number
      visible to both the customer and us.
      **Why it's P0 and not a billing nicety:** their traffic is now
      our bill. An uncapped plan means an unbounded cost we discover
      at the end of the month. The cap must degrade honestly (tell
      the customer, keep serving or stop by a documented rule) —
      never silently, and never by breaking their site without notice.
- [ ] **23. Billing (Stripe) + signup** — subscription, plan changes,
      failed-payment handling, and a signup that gets someone from
      landing page to protected domain without us in the loop.
      Self-serve is the point; a sales call for a $200/mo product
      costs more than the product.

**Enterprise (self-hosted) is deliberately absent from this list.**
It stays supported and priced above the hosted plans, but there are
zero customers for it — building tooling now is `CLAUDE.md` Section 14
exactly. Keep the single-tenant path working (it is what Enterprise
ships) and do not delete it as dead code.

---

## How to read this list

Work top to bottom. A P0 item is only "done" when it meets the bar in
`CLAUDE.md` (tested against real bot traffic *and* real browser
traffic, not just the happy path) before moving to the next one.

The MVP goal is items 1–6: a working proxy that meaningfully reduces
naive-to-intermediate bot traffic (plain scripts, unconfigured
libraries, basic headless browsers) for a real client, deployable
cheaply. Advanced-automation resistance (P1) comes after that's proven.
Items 1–6 are now done — see the "Done" section.

**Before adding an item, answer both:**

1. Which real evasion technique or buyer need does this close?
   ("A big vendor has it" is not an answer — `CLAUDE.md` Section 14.)
2. Does it make us *more* the thing at the top of this file
   (self-hostable, inline, provable), or just more feature-equal with
   someone else? Only the first kind earns a slot.

**Priority note, 2026-09-16.** Detection depth (items 7–10) is no
longer automatically ahead of items 12a, 18, 11b and 19. What is built
already catches naive-to-intermediate bots; what is missing is the
ability to *show a client what it caught* and *let them set policy on
it*. A signal nobody can see the output of does not sell, and cannot
be checked for false positives against real traffic. Re-order once a
real client's traffic says otherwise — but don't default to "more
signals" just because signals are the fun part.

Item 19 sits above them all on *defensibility*, below them on
*readiness*: it is the only item a competitor can't copy in a
fortnight, and also the only one whose real cost is still unknown.
Scope it early even if it's built late. (12a is done; 18 is the
next build, since it's what measures the false-positive rate against
real traffic instead of assumptions.)

**Priority note, 2026-09-16 (hosted pivot).** The P0-SaaS block
(items 20–23) now sits ahead of everything in P1 and P2. Not because
it is more interesting — it is the least interesting work in this
file — but because no amount of detection quality produces revenue
while the product cannot take a second customer or a payment. Item 18
(shadow mode) is the one exception worth doing alongside it: it is
how the first customer is won, and it needs no multi-tenancy to
demonstrate on a single site.

Note also what the pivot did to item 19: hosting every customer's
traffic means we observe real browser fingerprints continuously, so
the database can substantially build itself rather than being
researched from scratch. Its cost estimate should be redone before
anyone scopes it — see `docs/DECISIONS.md`, hosted-pivot entry.
