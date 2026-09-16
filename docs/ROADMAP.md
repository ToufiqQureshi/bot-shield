# bot-shield Roadmap

Plain-English list of what's built, what's next, and the plan to make
bot-shield a genuinely useful, affordable alternative to
Akamai/DataDome/PerimeterX for companies that can't afford them.

Not a wishlist — every item here targets a real bot-evasion technique
seen in the field (from tools like Patchright, Scrapling, curl_cffi-based
impersonation clients, and plain scripted HTTP clients). See `CLAUDE.md`
for the engineering rules every item must follow before it counts as done.

**Competitive position.** Not "more features than Akamai." The target
is: **good enough detection, deployed in an afternoon, at a price a
mid-size company can actually pay.** Multi-layer scoring beats any
single clever check, because any single check is exactly what
stealth-automation tools are built to defeat.

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

**Shape:** one product — a proxy binary + dashboard a client deploys
directly, no code required on their end. Internally, each layer above
is its own small Go package, purely for code clarity (easier to test
and fix), not as a reusable library for outside use.

---

## Done

- [x] Project scaffolding, engineering rules (`CLAUDE.md`), this roadmap.
- [x] **1. Reverse proxy skeleton** (`proxy/proxy.go`, `cmd/botshield`)
      — a stdlib `httputil.ReverseProxy` that forwards every request
      to the configured origin unchanged, plus a `botshield` binary
      with graceful shutdown on SIGINT/SIGTERM. Verified with unit
      tests and a real end-to-end run (local origin + proxy + curl).
      Known gap: no timeout/error handling yet for a dead or slow
      origin (proxy just returns Go's default 502) — that belongs
      with item 2 (HTTP fetcher) / retry work, not this skeleton.
- [x] **2. TLS/JA4 fingerprinting** (`proxy/fingerprint.go`,
      `proxy/capture.go`) — `cmd/botshield -tls-cert`/`-tls-key` makes
      bot-shield terminate TLS itself, capture each connection's raw
      ClientHello, turn it into a JA4 hash, and forward it to the
      origin as `X-BotShield-JA4`. Tested against known-good JA4
      vectors, a real end-to-end TLS handshake + proxied request, a
      bad/garbage handshake (must not hang or crash the listener), and
      `-race`. Verified with a real binary run (openssl self-signed
      cert + curl through botshield). Plain HTTP (no `-tls-cert`) still
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
      `X-BotShield-UA-Mismatch: true`, stripped from the incoming
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
      cost (see `DECISIONS.md`), not a one-line addition. What's built
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
      (`X-BotShield-Passed`) and redirects back to the exact page the
      visitor originally asked for. Token and cookie are HMAC-signed
      with a random in-process secret, checked with
      `subtle.ConstantTimeCompare`; the redirect target is validated by
      `safeRedirectPath` against open-redirect. Wired into
      `cmd/botshield` at `/__botshield/challenge` and
      `/__botshield/verify` — reachable today for manual testing, not
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
          bot-shield. Documented, accepted limitation, same class as
          item 3's JA4-database gap.
        - Signing secret is generated fresh per process, in memory
          only — a restart or a second instance invalidates
          outstanding challenges/cookies. Fine for the current
          single-process v1; needs the planned Redis store to share
          across instances.
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
      `cmd/botshield` as the real handler for `/` (the standalone
      `/__botshield/challenge` and `/__botshield/verify` routes stay
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
      connection bot-shield can't examine).
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
      contract Antigravity posted in `agentchat/chat.jsonl` for the
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
      the JS challenge page (item 4) — the one place bot-shield already
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
- [ ] **10. Honeypot fields** — invisible form fields/links only a
      blind selector-based script would interact with.
- [ ] **11. Per-client rules** — each client (site) can tune thresholds,
      allowlist known-good bots (search engine crawlers, uptime
      monitors), and set custom block pages.
- [ ] **11a. Deception mode (decoy response)** — a fifth decision
      outcome alongside allow/challenge/block: for high-confidence-bot
      traffic, forward the request with `X-BotShield-Decision: deceive`
      instead of blocking, and let the origin app decide what fake data
      to return (stale price, dummy inventory). bot-shield only signals
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

## P2 — product-grade

- [ ] **12. Dashboard** — requests scored, blocked, challenged over
      time; top offending fingerprints/IPs; false-positive report
      button for the client's ops team.
      **Started, not done:** `dashboard/` (Next.js) shows a live
      snapshot (total/passed/challenged/blocked) from the real
      `/api/v1/dashboard/stats` endpoint — see item 5's Done entry and
      `docs/PROGRESS.md` 2026-09-16. Missing: history over time (the
      backend only keeps a running total, no time series), top
      offending fingerprints/IPs, the false-positive report button,
      and any auth on the dashboard itself.
- [ ] **13. Real-time scoring API** — for clients who want to call
      bot-shield from their own app instead of routing all traffic
      through the proxy.
- [ ] **14. Deployment story** — Docker image + simple config, so a
      client can stand this up in under an hour (this is the actual
      competitive edge over enterprise tools' weeks-long onboarding).
- [ ] **15. Metrics/observability** — latency added per layer, decision
      breakdown, exportable to Prometheus.
- [ ] **16. Long-running soak test** — sustained adversarial traffic
      simulation proving memory/latency stay stable over hours.
- [ ] **17. Pricing/tiering model** — defined once the MVP is proven
      against real client traffic, not before.

---

## How to read this list

Work top to bottom. A P0 item is only "done" when it meets the bar in
`CLAUDE.md` (tested against real bot traffic *and* real browser
traffic, not just the happy path) before moving to the next one.

The MVP goal is items 1–6: a working proxy that meaningfully reduces
naive-to-intermediate bot traffic (plain scripts, unconfigured
libraries, basic headless browsers) for a real client, deployable
cheaply. Advanced-automation resistance (P1) comes after that's proven.
