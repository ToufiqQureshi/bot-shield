# RESEARCH.md — Reference Notes

Background research this project is built on. Not decisions (see
`docs/DECISIONS.md` for those) — this is the raw "what we learned"
that the decisions are based on. Update this when new research
happens; don't let it go stale silently.

---

## How Akamai / DataDome / Cloudflare actually detect bots

Studied because bot-shield's detection layers are modeled on the same
signal types, at a smaller scale:

1. **TLS/JA3/JA4 fingerprint** — hashes the TLS ClientHello's cipher
   order + extensions. Every HTTP client/browser has a distinct
   signature; scraping tools built on raw `requests`/`curl` or
   unconfigured HTTP libraries mismatch a real browser's signature.
2. **HTTP/2 fingerprint** — SETTINGS frame order, header order,
   pseudo-header (`:method`, `:path`) order. Browsers have a fixed,
   consistent order; most automation libraries don't replicate it.
3. **JS challenge** — math + canvas render + timing puzzle served to
   ambiguous traffic. A plain HTTP client without a JS engine fails
   immediately; even a real headless browser without JS enabled fails.
4. **CDP detection** — checks for `Runtime.enable` artifacts,
   `window.chrome` inconsistencies, extra V8 bindings. Catches
   Playwright/Puppeteer-style automation specifically.
5. **Behavioral scoring** — mouse movement, scroll, keystroke timing
   fed into a "human score" model.
6. **Signed cookie / session fingerprint** — an encrypted
   device-fingerprint cookie checked for consistency request-to-request.
7. **IP reputation + rate pattern** — datacenter IP ranges, proxy pool
   patterns, unnaturally regular request timing.

`docs/ROADMAP.md`'s P0/P1 signals map directly to 1–3 and 5 above; 4 is
covered by the client-side automation probe (item 6); 6–7 are P1/P2.

---

## Stealth/evasion tools studied (what we're building against)

Researched to understand what modern scraping automation defeats by
default, so bot-shield doesn't rely on checks that are already solved
problems for attackers:

- **Patchright** (patched Playwright) — suppresses CDP-detection
  artifacts (`Runtime.enable` leaks, `navigator.webdriver`) by using
  isolated execution worlds and dropping automation flags. Real Chrome
  binary, so its TLS/JA4 fingerprint looks legitimate — CDP-detection
  alone does not catch it; behavioral/timing signals are what would.
- **Scrapling** (`StealthyFetcher`, built on Camoufox) — randomizes
  canvas/WebGL/font fingerprints per session, humanizes mouse movement,
  and (via `curl_cffi`) replicates real Chrome's TLS ClientHello
  byte-for-byte for its non-browser HTTP fetcher — defeats naive
  TLS/JA3 checks specifically.
- **obscura-scraper** — proxy + fingerprint rotation, smaller/simpler
  version of the same ideas.

**Implication for bot-shield:** none of these tools are stopped by a
single check. This is the direct source of the multi-signal scoring
decision in `DECISIONS.md`.

---

## Large-scale residential proxy networks (e.g. Bright Data-class)

A different threat tier from the tools above — not a browser-evasion
trick, but real traffic through real residential IPs (Bright Data
alone advertises 72M+ IPs).

**Why IP-based defense fails against this:** the IP genuinely belongs
to a real residential ISP connection. IP reputation/blocklisting has
nothing to flag.

**What still works:**
- **Fingerprint, not IP** — the TLS/JA4 fingerprint, header order, and
  request timing of the *client software* driving the proxy stay
  consistent no matter which IP it's tunneled through. This is the
  actual detection surface.
- **Cross-IP correlation** — the same exact fingerprint + request
  pattern appearing across many distinct IPs, in a way real independent
  users wouldn't produce, is a strong signal even though each
  individual IP looks clean.
- **Aggregate rate, not per-IP rate** — per-IP rate limiting is
  useless when each IP sends one request. Rate limiting keyed on the
  fingerprint/signature, aggregated across the whole apparent pool,
  is what catches it.
- **Reality check:** nobody fully stops this tier, including Akamai
  and DataDome. The realistic goal is raising the attacker's cost
  enough that targeting a mid-size site stops being worth it — not
  hitting 100%. This is why it's explicitly P1/P2 in `ROADMAP.md`, not
  MVP (see `DECISIONS.md`).

---

## Open-source building blocks identified (don't reinvent these)

- **[fingerproxy](https://github.com/wi1dcard/fingerproxy)** — Go
  HTTPS reverse proxy / library that generates JA3, JA4, and Akamai
  HTTP/2 fingerprints and forwards them via headers. Direct fit for
  `ROADMAP.md` item 2.
- **[BotD](https://github.com/fingerprintjs/botd)** (FingerprintJS,
  MIT) — client-side JS library that detects automation frameworks in
  the browser. Basis for `ROADMAP.md` item 6.
- **SafeLine** (Chaitin Tech) — self-hosted open-source WAF, reverse
  proxy pattern worth referencing for the proxy skeleton's deployment
  story, not a dependency.
- **open-appsec** — open-source ML-based WAF/bot-defense; reference
  for scoring-engine ideas, not a dependency for v1 (rule-based scoring
  first, see `docs/ARCHITECTURE.md`).

Principle: assemble proven open-source primitives for the hard,
well-solved parts (TLS parsing, client-side automation detection);
spend original engineering on the part that's actually the product —
the scoring/decision layer and the deployment experience.

---

## ClientHello fragmentation — a cheap way past TLS fingerprinting

Found on 2026-09-14 by an independent security review of our own
capture code, then reproduced locally.

**The trick:** TLS allows a single handshake message to span several
records. A client can therefore send its ClientHello split across two
records. Go's `crypto/tls` server reassembles them and the handshake
completes normally — the site works fine for that client.

**Why it defeats naive capture:** code that grabs the first TLS
*record* (rather than the whole handshake *message*) only ever sees
the first fragment. `fingerproxy`'s `hack.HijackClientHelloConn` does
exactly this: it reads the record header, sets
`expectedLen = 5 + recordLength`, and truncates there. The captured
bytes are a partial ClientHello, so JA4 parsing fails.

**Cost to the attacker:** one line in their socket layer. No special
tooling, no fingerprint forging, no proxy network.

**What we do about it today** (see `DECISIONS.md`): a TLS connection
whose handshake we cannot read is reported as `JA4Unreadable`, not as
an empty fingerprint. The evasion still works at the capture level,
but it stops being *invisible* — and a real browser never triggers it,
so it becomes a scoring signal of its own rather than a silent bypass.

**Still open:** actually reassembling a multi-record handshake message
before fingerprinting. That means record-layer reassembly, which
`DECISIONS.md` says we don't hand-write. Right fix is upstream in
fingerproxy, or a small well-tested reassembly step in front of it.
Worth checking whether other JA4 implementations (and commercial
vendors) handle this — if they don't, fragmentation is a general gap
in the JA4 ecosystem, not just ours.

---

## Competitor feature scan — 2026-09-15

What DataDome, Akamai Bot Manager, Cloudflare Bot Management, Kasada,
Arkose Labs, and HUMAN/PerimeterX offer that this roadmap doesn't
cover, so a future session doesn't re-research this from scratch.

- **API-aware rules** (DataDome's stated differentiator) — endpoints
  get different thresholds by role (login vs. checkout vs. product
  listing), not one generic rate limit. Cheap to add: it's `ROADMAP.md`
  item 9 + item 11 combined with an "endpoint category" config field,
  not a new signal. **Added to roadmap as item 9a.**
- **Deception / decoy responses** (mentioned across several vendors as
  an alternative mitigation to CAPTCHA/block) — serve fake data instead
  of blocking, so a scraper doesn't know it's been caught and keeps
  burning its own time on garbage. Genuinely under-offered even by big
  vendors — a real differentiator for a small product. **Added to
  roadmap as item 11a**, scoped so bot-shield only signals the
  decision; the origin app owns what fake data means for it.
- **Persistent cross-session device fingerprinting** (Arkose Device
  ID) — AI similarity matching that re-identifies the same device
  across IP/session changes, not just per-connection JA4. Not added:
  needs a continuously-updated similarity model and cross-session
  storage — that's an ML infra project of its own, not a roadmap item.
  Revisit only if a client's real traffic shows per-connection
  fingerprinting isn't enough.
- **Native mobile SDK** (PerimeterX/HUMAN) — protects native iOS/
  Android app traffic, not just web. Not added: separate codebase,
  separate maintenance surface, and bot-shield's whole product shape
  (`ROADMAP.md`) is a web reverse proxy. Out of scope unless a client
  need makes it real.
- **Shared cross-customer threat intelligence** (DataDome/Cloudflare's
  network effect: a bad fingerprint seen on one customer gets blocked
  network-wide) — not added: requires sharing one client's traffic
  data to protect another, which needs an explicit consent/data-sharing
  framework to not violate `CLAUDE.md` Section 18 (no data overreach).
  Worth revisiting once there are enough paying clients for the network
  effect to matter, with that consent model designed first.
- **WASM-based deep browser-engine fingerprinting** (PerimeterX's
  2025–26 move: measure execution speed of math ops to tell a real
  browser engine from an automated one, even through CDP-suppression
  tools like Patchright) — not added: this is a genuinely hard,
  research-heavy signal; revisit after `RESEARCH.md`'s existing
  Patchright/Scrapling gap (see above) is a real, observed problem in
  client traffic, not before.

---

## Market & pricing scan — 2026-09-16

The 2026-09-15 scan above asked "what features do vendors have?"
This one asks the question that actually decides whether this product
earns money: **who already sells to our buyer, at what price, and
what is nobody selling them?**

### What the market charges today

| Tier | Products | Price |
|---|---|---|
| Enterprise | DataDome, HUMAN, Kasada, Arkose, Akamai | ~$1K–50K/mo, quote-only, weeks-long sales cycle |
| Self-serve SaaS | Prosopo (~$39/mo to 100K), cside (~$99/mo), Moonito | $0–100/mo |
| Free | Cloudflare free tier | $0 |
| Free + self-hosted | CrowdSec, SafeLine WAF, Coraza, ModSecurity, ALTCHA | $0 |

Forrester's Q2 2026 Wave for Bot and Agent Trust Management named
DataDome, HUMAN and Kasada as Leaders — note the category name: it is
no longer "bot management," it is **agent trust management**.

### What this means for us (the uncomfortable part)

The price floor is **$0**, not "cheaper than Akamai." A buyer told
"affordable bot detection" answers with "CrowdSec is free." So
competing on *price* against the enterprise tier is not a strategy —
the ground below us is already given away.

The competitor that actually matters to us is therefore **CrowdSec,
not DataDome**. Where we genuinely differ:

| | CrowdSec | bot-shield |
|---|---|---|
| Input | parses server **logs** | reads the **live request** (TLS ClientHello) |
| Timing | reactive — after bad requests land | first request, before the origin sees it |
| Basis | IP reputation, shared blocklists | per-connection JA4 + UA consistency + score |
| Needs | a prior sighting of that IP | nothing prior |

No self-hostable product does inline TLS/JA4 fingerprint scoring.
That is the actual technical moat, and it is narrow but real.

### The 2026 shift: bots → AI agents

The question changed from *"is this a bot?"* to *"which agent is
this, is it allowed, and can I prove what I decided?"*

- Cloudflare launched **pay-per-crawl** (HTTP `402` on crawler
  requests) and, from 2026-09-15, blocks "mixed-use" AI crawlers by
  default on ad-bearing pages.
- **RSL (Really Simple Licensing)** emerged as an open standard for
  attaching licensing terms to content, not just an allow/deny flag.
- **Web Bot Auth** is the in-flight proposal for crawlers to
  cryptographically prove their identity.
- Caveat worth remembering: OpenAI, Anthropic, Google DeepMind and
  Meta have **not** announced support for pay-per-crawl or Web Bot
  Auth. So "charge the crawler" does not work yet — but "identify,
  govern and log the crawler" does, and that is the part we can
  actually build.

All of this is enterprise-only today. Mid-market sites have exactly
two options for an AI crawler: allow, or block. Nobody sells them
policy. **That gap is where items 11b and 12a come from.**

### Not added from this scan

- **Pay-per-crawl / HTTP 402 billing** — needs crawler-side adoption
  that does not exist yet (see caveat above); we would ship a toll
  booth nobody pays at. Revisit if a major lab adopts Web Bot Auth.
- **Shared/crowdsourced blocklist** (CrowdSec's network effect) —
  same consent problem already logged in the 2026-09-15 scan.

Sources: cside and Prosopo vendor comparisons, Pi Stack's 2026
self-hosted WAF comparison, Stack Overflow's pay-per-crawl writeup,
TechCrunch on Cloudflare's crawler policy (all fetched 2026-09-16).
