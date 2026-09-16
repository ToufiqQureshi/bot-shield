# bot-shield Architecture

What the product is made of and why. See `docs/ROADMAP.md` for build
order, `docs/DECISIONS.md` for the reasoning behind each choice, and
`CLAUDE.md` for the rules code must follow.

**Read the status markers.** Most of this document describes the
target design. Only the parts marked **BUILT** exist today — don't
assume a box on a diagram is running code.

---

## System overview

```text
Internet (every visitor, hostile until scored)
   │
   ▼
[bot-shield]  ← terminates TLS, sits in front of the client's origin
   │
   ├── capture      BUILT   keep the raw TLS handshake  (proxy/capture.go)
   ├── fingerprint  BUILT   handshake → JA4 hash        (proxy/fingerprint.go)
   ├── proxy        BUILT   forward, strip spoofable headers (proxy/proxy.go)
   │
   ├── score        BUILT    combine signals → risk score (proxy/score.go,
   │                         proxy/guard.go) — allow/challenge/block
   ├── challenge    BUILT    JS challenge (proxy/challenge.go), now
   │                         triggered by score via Guard
   ├── ratelimit    planned  per-IP / per-fingerprint caps
   │
   ├──► Redis       planned  session/fingerprint cache, rate counters
   ├──► Postgres    planned  client configs, block logs, analytics
   │
   ▼
[Client's origin server]
   receives the request plus the headers in the contract below

[Dashboard]  BUILT (skeleton)  Next.js app in dashboard/, wired to
             the real /api/v1/dashboard/stats endpoint (proxy/stats.go)
             — one stat card, no history/charts/auth yet
```

As of 2026-09-15, bot-shield **acts** on what it observes: `Guard`
(`proxy/guard.go`) scores every request and allows, JS-challenges, or
blocks it — see `ROADMAP.md` item 5. Only 2 of the planned signals
feed the score so far (JA4 fragmentation, UA mismatch); items 6+ add
more inputs, not a new decision mechanism.

---

## What the origin receives — BUILT

This is the product's contract with the client's server, and with our
own future scoring code. Treat it as an API: changing it breaks both.

| Header | Meaning |
|---|---|
| `X-BotShield-JA4` | The connection's JA4 fingerprint, e.g. `t13d1516h2_8daaf6152771_e5627efa2ab1`. |
| `X-BotShield-JA4: unreadable` | The connection was TLS, but the handshake couldn't be read — see the fragmentation note in `docs/RESEARCH.md`. A normal client never causes this, so it is itself a signal. |
| *(header absent)* | Not a TLS connection at all — bot-shield is running without `-tls-cert`, so there is nothing to fingerprint. |
| `X-Real-IP` | The real client address, set by us. |
| `X-Forwarded-For`, `X-Forwarded-Host`, `X-Forwarded-Proto` | Set by us from the real connection. |

**Every one of these is stripped from the inbound request before we
set our own value.** A visitor cannot forge any of them. That is not
a detail — a spoofable signal is worse than no signal, because the
scoring layer would trust it (`CLAUDE.md` Section 6).

---

## Tech stack

| Layer | Choice | Status | Why |
|---|---|---|---|
| **Core service** | Go | BUILT | Single static binary, low memory, high concurrency — it sits in every request's path |
| **Reverse proxy** | stdlib `httputil.ReverseProxy`, using `Rewrite` (not `Director`) | BUILT | `Rewrite` makes net/http strip the visitor's `X-Forwarded-*` headers; `Director` does not (`DECISIONS.md`) |
| **TLS termination + handshake capture** | stdlib `crypto/tls` + `fingerproxy`'s `pkg/hack` conn wrapper | BUILT | Go discards the raw handshake bytes after the handshake; JA4 needs them. `Accept` returns a real `*tls.Conn`, so net/http owns the handshake, its timeout, its error handling and its connection management |
| **JA4 computation** | `fingerproxy`'s `pkg/ja4` only | BUILT | Don't reinvent TLS parsing. Importing only this package keeps Prometheus and gopacket out of the binary (`DECISIONS.md`) |
| **Client-side automation probe** | small custom JS snippet (BotD-inspired) | planned | Catches automation in a real browser, which server-side signals can't see |
| **Fast state** (rate limits, session cache) | Redis | planned | Sub-millisecond reads with TTL; must not add latency per request |
| **Durable state** (configs, logs, analytics) | PostgreSQL | planned | Dashboard queries and per-client settings must survive restarts |
| **Dashboard** | Next.js, separate app (`dashboard/`) | BUILT (skeleton) | Client-facing UI, no reason to share the proxy's release cycle. Wired to the real `/api/v1/dashboard/stats` endpoint (`proxy/stats.go`); one stat card, no history/charts/auth yet |
| **Deployment** | Docker image + compose (proxy + Redis + Postgres) | planned | Running in under an hour is the actual edge over enterprise onboarding |
| **Metrics** | Prometheus client lib, off by default | planned | Optional; zero cost for clients who don't want it |

---

## How a request flows today — BUILT

1. `cmd/botshield` listens on `-addr`. With `-tls-cert`/`-tls-key` it
   wraps the listener in `proxy.NewCaptureListener`; without them it
   serves plain HTTP and no fingerprinting happens.
2. `Accept` wraps the raw connection so the handshake bytes are kept,
   then hands net/http a real `*tls.Conn` — **unhandshaked on
   purpose**, so the standard library performs the handshake with its
   own timeout (derived from `ReadHeaderTimeout`), its own error
   handling, and its own panic recovery.
3. `http.Server.ConnContext` puts the connection in the request
   context.
4. Per request, `JA4FromContext` reads the saved handshake off the
   connection and derives the fingerprint.
5. The proxy's `Rewrite` strips every spoofable identity header, sets
   the real ones, and forwards to the origin.

---

## Why this shape (not something fancier)

- **Let the standard library do the work.** The capture listener
  deliberately owns as little as possible. Everything net/http
  already does — handshake timeouts, accept retries, panic recovery,
  connection handling — is its job, not ours (`CLAUDE.md` Section 24).
- **No Kubernetes, no microservices for v1.** One binary and two
  datastores covers a single-client or few-client deployment. Split
  only when a measured bottleneck proves it's needed.
- **No ML model in v1.** Rule/threshold scoring is explainable, fast,
  and good enough for naive-to-intermediate bots. Training a model on
  no data produces something worse than simple thresholds.
- **Fail open by default.** If fingerprinting fails, the request is
  still forwarded — labelled, never blocked. A broken bot-detector
  must never take down the client's actual site.

---

## Request path budget

Target: **under 15ms added to a passthrough request** (excluding the
JS-challenge path, which only suspicious traffic sees).

| Step | Budget | Measured |
|---|---|---|
| JA4 fingerprint | ~2ms | **14.3µs** — 0.7% of budget |
| Redis rate-limit check | ~2ms | not built |
| Scoring (rule-based) | ~1ms | not built |
| Proxy overhead | ~5ms | not measured |
| Headroom | ~5ms | — |

The fingerprint is derived per request rather than cached per
connection. At 14.3µs that is deliberate: caching it would be
optimising 0.7% of a budget (`CLAUDE.md` Section 3 — measure first).
Re-measure before assuming this still holds.

If a layer can't hit its budget, it needs a timeout and a fail-open
fallback, not a slower default.

---

## Known architectural limits — BUILT code only

- **HTTP/1.1 only.** The capture listener does not offer h2, because
  HTTP/2 fingerprinting isn't built. Browsers fall back to HTTP/1.1.
  Adding h2 is now cheap — stdlib negotiation works precisely because
  `Accept` returns a real `*tls.Conn`.
- **A fragmented ClientHello can't be fingerprinted.** A handshake
  message split across TLS records defeats the capture (it reads one
  record). Reported as `unreadable` so it is visible, but it is not
  prevented. See `docs/RESEARCH.md`.
- **bot-shield must terminate TLS to see anything.** Behind a CDN or
  load balancer that terminates TLS first, there is no handshake to
  capture and no fingerprint — a deployment constraint, not a bug.
