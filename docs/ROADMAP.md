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

---

## P0 — MVP (prove the core idea works)

- [x] ~~**1. Reverse proxy skeleton**~~ — done, see "Done" section above.
- [ ] **2. TLS/JA4 fingerprinting** — capture JA4 (and HTTP/2 fingerprint)
      per connection, using `fingerproxy`-style approach. This alone
      catches most naive scripted clients (raw `requests`/`curl`,
      unconfigured HTTP libraries).
      **In progress:** `proxy/fingerprint.go` computes the JA4 hash
      from a raw ClientHello record (wraps `fingerproxy`'s `ja4`
      package per `DECISIONS.md`), tested against known-good vectors
      (real curl ClientHello → verified JA4). Not done yet: nothing
      captures a *live* ClientHello — bot-shield doesn't terminate TLS
      at all today (`cmd/botshield` proxies plain HTTP). Next step is
      wiring a TLS-terminating listener (see `fingerproxy/pkg/hack`'s
      `HijackClientHelloConn` pattern) so a real connection's
      ClientHello reaches this function.
- [ ] **3. Basic header/UA consistency check** — does the claimed
      User-Agent match the TLS/HTTP2 fingerprint's real client family?
      Mismatch = strong signal.
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
- [ ] **10. Honeypot fields** — invisible form fields/links only a
      blind selector-based script would interact with.
- [ ] **11. Per-client rules** — each client (site) can tune thresholds,
      allowlist known-good bots (search engine crawlers, uptime
      monitors), and set custom block pages.

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
