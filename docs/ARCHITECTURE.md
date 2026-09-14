# bot-shield Architecture

Production-grade tech choices for the MVP. See `docs/ROADMAP.md` for
build order, `CLAUDE.md` for coding rules.

---

## System overview

```text
Internet
   │
   ▼
[bot-shield proxy]  ← sits in front of client's origin server
   │
   ├── fingerprint (TLS/JA4, HTTP/2, headers)
   ├── score        (combine signals → risk score)
   ├── challenge    (JS challenge for ambiguous traffic)
   ├── ratelimit    (per-IP/fingerprint request caps)
   │
   ├──► Redis        (fast: session/fingerprint cache, rate counters)
   ├──► Postgres      (durable: client configs, block logs, analytics)
   │
   ▼
[Client's origin server]  (only sees traffic bot-shield let through)

[Dashboard] ← reads from Postgres, shown to the client
```

---

## Tech stack

| Layer | Choice | Why | How it helps |
|---|---|---|---|
| **Core service** | Go | Same stack as goScraper, team already knows it | Single static binary, low memory, high concurrency — needed since this sits in every request's path |
| **Reverse proxy** | Go `net/http/httputil.ReverseProxy` (stdlib) | No heavy framework needed for v1 | Fewer dependencies, full control over request/response, easy to instrument |
| **TLS/JA4 fingerprint** | `fingerproxy` (open-source, Go) | Already solves JA3/JA4/HTTP2 fingerprint extraction correctly | Don't reinvent TLS parsing — biggest single detection signal, for free |
| **Client-side automation probe** | Small custom JS snippet (BotD-inspired) | Needed in-browser, can't be done server-side alone | Catches automation frameworks that pass TLS checks (real browser, scripted) |
| **Fast state (rate limits, session cache)** | Redis | Sub-millisecond reads, built-in TTL/eviction | Rate-limiting and fingerprint lookups must not add latency to every request |
| **Durable state (configs, logs, analytics)** | PostgreSQL | Battle-tested, relational fits client/config/analytics data well | Dashboard queries, historical trends, per-client settings survive restarts |
| **Dashboard** | Next.js (separate small app, own repo/folder) | Client-facing UI; no reason to couple it to the Go proxy's release cycle | Client sees blocked-bot counts/trends without touching the proxy |
| **Deployment** | Single Docker image + docker-compose (proxy + Redis + Postgres) | Client should be running in under an hour | This is the actual competitive edge vs. Akamai/DataDome's weeks-long onboarding |
| **Metrics** | Prometheus client lib (optional, off by default) | Standard, lightweight, no forced dependency | Client can plug into their own monitoring if they want; zero cost if they don't |

---

## Why this shape (not something fancier)

- **No Kubernetes, no microservices for v1.** One binary + two datastores is
  enough traffic for a single-client or few-client deployment. Split
  services only when a real bottleneck proves it's needed (`CLAUDE.md`
  Section 3 — no premature abstraction).
- **No ML model in v1.** Rule/threshold-based scoring (Section 6/20 in
  `CLAUDE.md`) is explainable, fast, and good enough to catch naive-to-
  intermediate bots. ML scoring is a P1+ item once we have real traffic
  data to train on — training on nothing produces a worse model than
  simple thresholds.
- **Fail-open by default.** If bot-shield itself errors or times out,
  traffic passes through untouched. A broken bot-detector must never
  take down the client's actual site (see `CLAUDE.md` Section 9).

---

## Request path budget

Every layer adds latency. Target: **under 15ms added to a passthrough
request** (excludes the JS-challenge path, which is only shown to
already-suspicious traffic).

| Step | Budget |
|---|---|
| TLS/JA4 fingerprint lookup | ~2ms |
| Redis rate-limit check | ~2ms |
| Scoring (rule-based) | ~1ms |
| Proxy overhead | ~5ms |
| Headroom | ~5ms |

If a layer can't hit its budget, it needs a timeout + fail-open
fallback, not a slower default.
