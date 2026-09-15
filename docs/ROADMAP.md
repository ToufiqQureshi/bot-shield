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

---

## P0 — MVP (prove the core idea works)

- [x] ~~**1. Reverse proxy skeleton**~~ — done, see "Done" section above.
- [x] ~~**2. TLS/JA4 fingerprinting**~~ — done (HTTP/2 fingerprint part
      still open), see "Done" section above.
- [x] ~~**3. Basic header/UA consistency check**~~ — done (narrower
      than full HTTP2-family verification, no fingerprint database
      needed), see "Done" section above.
- [ ] **4. JS challenge** — a lightweight challenge page (math + timing +
      basic canvas check) served to unscored/ambiguous traffic. A
      plain HTTP client without a JS engine fails immediately.
- [ ] **5. Scoring engine v1** — combine signals 2–4 into one risk score
      with configurable thresholds; three outcomes: allow, challenge,
      block.
- [ ] **6. Client-side automation-tool probe** — a small JS snippet
      (inspired by/leveraging open-source detectors like `BotD`) that
      flags obvious automation frameworks in-browser.

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
