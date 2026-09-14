# DECISIONS.md — Why We Chose What We Chose

A running log of real decisions made on this project and the
reasoning behind them, so a new session/agent doesn't have to guess
or re-derive context that already exists. Newest entries at the top.

When you make a real decision (tech choice, scope cut, priority
change, rejected alternative), add an entry here **before** ending
your session. See `CLAUDE.md` Section 0 / the mandatory update rule.

---

## Format for new entries

```text
## <short title> — <date>
Decision:
Why:
Alternatives considered / rejected:
Revisit when:
```

---

## Assemble proven open-source pieces, don't reinvent TLS/fingerprint parsing — 2026-09-14
**Decision:** use `fingerproxy` for JA3/JA4/HTTP2 fingerprinting and a
`BotD`-style approach for the client-side automation probe, instead of
writing TLS ClientHello parsing or automation-detection heuristics
from scratch.
**Why:** these are hard, well-solved problems with mature open-source
implementations already (see `docs/RESEARCH.md`). Original engineering
effort should go into the part that's actually the product: the
scoring/decision layer and the deployment experience — not
re-deriving TLS fingerprinting from the spec.
**Alternatives considered:** building fingerprinting in-house for
full control — rejected, high effort for something already solved,
delays the actual differentiator.
**Revisit when:** a specific limitation in `fingerproxy`/`BotD` blocks
a real requirement and can't be worked around.

---

## Large-scale residential proxy networks: fingerprint/pattern-based, not IP-based — 2026-09-14
**Decision:** don't attempt IP-reputation-based defense against
large commercial residential-proxy traffic (e.g. Bright Data-class
networks). Rely on cross-IP fingerprint correlation and
aggregate-rate detection instead, and treat this as a cost-raising
goal, not a 100%-block goal.
**Why:** the IPs in these networks are real residential connections —
IP reputation has nothing to flag. The client software driving the
proxy still has a consistent fingerprint/behavior regardless of which
IP it tunnels through; that's the real signal. See `docs/RESEARCH.md`
for the full reasoning.
**Alternatives considered:** none viable — even Akamai/DataDome don't
fully stop this tier; promising full protection here would be a false
claim to clients.
**Revisit when:** P0/P1 fingerprint+rate infrastructure exists and
real client traffic shows this tier is a priority (ties to
`ROADMAP.md` item 9).

---

## Product is one deployable service, not a library — 2026-09-14
**Decision:** bot-shield ships as one product (reverse proxy +
dashboard) a client deploys directly. It is not published/marketed as
a reusable Go library for other developers to import.
**Why:** the actual customer is a non-technical site owner, not a Go
developer. A "library + product" framing added an audience and
maintenance burden nobody asked for.
**Alternatives considered:** core packages exposed as an importable
library in addition to the proxy — rejected as unnecessary scope for
v1 (see `CLAUDE.md` Section 3, no unnecessary abstraction).
**Revisit when:** a real client explicitly asks to embed detection
logic directly in their own app instead of running the proxy.

---

## Go stack, same as goScraper — 2026-09-14
**Decision:** bot-shield is written in Go.
**Why:** team already knows Go from goScraper; this service sits in
every request's path, so it needs high concurrency and low memory —
Go fits (same reason Cloudflare/Caddy/Traefik are Go/Rust, not
Python/Node, for this kind of workload).
**Alternatives considered:** Node.js (faster to prototype, weaker at
sustained high-concurrency proxy workloads); Rust (better raw
performance, slower team ramp-up, not worth it for v1).
**Revisit when:** never, unless a specific measured bottleneck proves
Go is the wrong tool for a specific component.

---

## Multi-signal scoring, no single-check verdicts — 2026-09-14
**Decision:** every detection layer (TLS/JA4, behavioral, session
consistency, rate pattern) contributes to a combined risk score.
No single signal alone can allow or block a request.
**Why:** advanced automation tools (patched browser-automation
frameworks, TLS-impersonating HTTP clients) are specifically built to
defeat one or two common checks (`navigator.webdriver`, basic CDP
detection). A single boolean check is a single point of failure that
these tools are already designed around.
**Alternatives considered:** simple allowlist/blocklist checks only —
rejected, defeated trivially by the exact tools we studied
(patched browser automation, TLS-impersonation clients).
**Revisit when:** never as a principle; specific thresholds/weights
will be tuned as real traffic data comes in.

---

## MVP scope = naive-to-intermediate bots, not nation-state-grade evasion — 2026-09-14
**Decision:** P0 (items 1-6 in `ROADMAP.md`) targets plain scripted
clients, unconfigured HTTP libraries, and basic headless browsers.
Large-scale residential-proxy traffic (e.g. big commercial proxy
networks) is explicitly P1/P2, not MVP.
**Why:** that class of traffic is the majority of what mid-size
clients actually get hit by today, and it's realistically achievable
with fingerprint + JS-challenge alone. Defeating large residential-
proxy-network traffic requires cross-IP fingerprint correlation and
aggregate rate analysis across huge IP pools — real capability, but a
P1/P2 problem, not something an MVP can or should promise.
**Alternatives considered:** trying to handle every adversary tier in
v1 — rejected, would delay shipping anything usable (see `CLAUDE.md`
Section 15, Solo/Small-Team Maintainer Mandate: ship something solid,
not something big and half-done).
**Revisit when:** P0 is live against real client traffic and proven;
then large-proxy-network correlation moves up the roadmap.

---

## Target market: mid-size companies priced out of enterprise bot management — 2026-09-14
**Decision:** target customer is a mid-size company (e-commerce,
ticketing, job portals, SaaS) that currently has weak/no bot
protection because Akamai/DataDome/PerimeterX pricing ($1,500-
$50,000+/month) is out of reach for them.
**Why:** this is a real, provable gap — the project owner's own
clients are already in this exact situation. Competing on feature
count against Akamai is not viable for a small team; competing on
price + fast deployment for an underserved segment is.
**Alternatives considered:** building a scraping/automation-evasion
tool instead (selling to bot operators, not against them) — rejected
outright: worse legal/ethical position, saturated market, and directly
conflicts with what the project owner's actual clients need protection
from. See `docs/AGENT.md` and `CLAUDE.md` Section 18 (Ethical/Legal
Boundary).
**Revisit when:** never as a market direction; pricing/tiering itself
is still TBD (`ROADMAP.md` item 17).
