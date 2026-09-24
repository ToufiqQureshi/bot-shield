# RESEARCH.md — Reference Notes

Background research this project is built on. Not decisions (see
`docs/DECISIONS.md` for those) — this is the raw "what we learned"
that the decisions are based on. Update this when new research
happens; don't let it go stale silently.

---

## 2026-09-24 — source review of five local reference repositories

Reviewed `inspired/horizon/HARDENING.md`, its server admission flow and
`server-go/inputforensics.go`,
`inspired/nexus/data/common/` policy recipes, `inspired/vertex/src/server/analysis.ts`,
`inspired/quantum/brotector.js`, and `inspired/zenith/lib/action/challenge.go`
plus `lib/challenge/resource-load/resource-load.go`.
The useful operational finding is that replay markers need storage without
eviction and a stable signing key. The useful detection finding is that browser
client hints can contradict a forged user agent, but are themselves client
claims and require measured false-positive rates. Nexus's infrastructure and
API carve-outs show why one global challenge policy can break crawlers and
integrations. Quantum's debugger and prototype hooks are unsuitable for live
visitors. Horizon's input timing thresholds came from its own browser/hardware
corpus, so importing those numbers without local calibration would overclaim.
Zenith's resource-load challenge is a browser capability check, but a scripted
client can fetch a resource too; it is corroboration rather than proof of a
human. See `CLIENT_PILOT_RELEASE.md` for the per-repo decision table and
outstanding work. No code was copied.

---

## Proof-of-work challenges: Anubis and FCaptcha cost model (2026-09-23)

Researched for Phase 2 (adaptive challenge difficulty) to pick a difficulty
ceiling that cannot lock out real phones.

**The mechanism both use:** the client must find an input whose SHA-256
starts with N hex zeros. Expected attempts is 16^N, and each attempt is a
separate awaited `crypto.subtle.digest` call in the browser — roughly
0.05–0.3 ms each on desktop, several times slower on a mid-range phone.
Anubis ships difficulty 4 (65k hashes, ~1–3 s desktop) as its default and
documents that higher values trade directly against low-end devices;
FCaptcha-style deployments keep visible work under ~1 s for the 95th
percentile device rather than tuning for the median.

**What this means for hakaishield's cap:** our range is 1–3 (16/256/4096
hashes). Difficulty 3 is ~49σ below the page's own 200k-iteration safety
cap and stays well under a second on desktop; raising to 4 multiplies the
work ×16 and starts eating into Anubis's observed phone-time budget. The
cap stays at 3 until a real mid-range device is measured (`DECISIONS.md`
2026-09-23). Difficulty per extra zero is ×16, so the ladder is:

- 1 → ~16 hashes (clean/strict traffic: imperceptible)
- 2 → ~256 hashes (the original fixed puzzle)
- 3 → ~4096 hashes (highest-scoring traffic, still mobile-safe)

**Attack-side cost:** a native solver (Go/openssl, not a browser) does
difficulty 3 in milliseconds, so the PoW's value is never "impossible" —
it is that solving at scale through real browsers costs real latency, and
the *server-chosen* difficulty (inside the signed token) lets us raise the
price for exactly the clients that scored worst, which a fixed puzzle
cannot do.

---

## How Akamai / DataDome / Cloudflare actually detect bots

Studied because hakaishield's detection layers are modeled on the same
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
default, so hakaishield doesn't rely on checks that are already solved
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

**Implication for hakaishield:** none of these tools are stopped by a
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
  roadmap as item 11a**, scoped so hakaishield only signals the
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
  separate maintenance surface, and hakaishield's whole product shape
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

| | CrowdSec | hakaishield |
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

---

## 2026-09-18 — Real-Chrome stealth automation (Patchright, Scrapling) beats every existing signal

### Threat

An owner test with **Patchright** (a stealth-patched Playwright fork)
passed straight through in-place scoring and the JS challenge. Root
cause: every signal we had — JA4, UA mismatch, header-anomaly, the
`navigator.webdriver`/CDP-leak probe — operates on "what the client
claims to be" (its TLS handshake, its UA string, its JS globals).
Patchright drives a real, patched Chromium binary via CDP and
specifically removes the well-known automation tells:
`--disable-blink-features=AutomationControlled`, no `--enable-automation`,
`Runtime.enable`/`Console.enable` avoided via isolated execution
contexts. Its TLS handshake and UA are therefore **genuinely** a real
browser's — nothing to catch at the network layer. Same root cause
applies to Scrapling's `camoufox` real-Firefox mode and any other
tool built the same way (nodriver, nodriver-based rebrowser-patches
consumers, nodriver-driven SeleniumBase CDP mode).

### Sources checked

- [Patchright README](https://github.com/Kaliiiiiiiiii-Vinyzu/patchright) —
  its own docs admit `__pwInitScripts`-style init-script injection
  "can be detected by Timing Attacks. However, no antibot currently
  checks for this" — a confirmed, still-open gap as of this scan.
- [rebrowser-bot-detector](https://github.com/rebrowser/rebrowser-bot-detector) —
  the most current, narrowly-targeted open check set (by the team
  that makes `rebrowser-patches`). Confirmed checks that survive
  patching: `window.__pwInitScripts !== undefined` (Playwright's own
  init-script global — a different mechanism than the CDP leaks that
  get patched), a default 800x600/1280x720 viewport, and a Chromium
  UA claiming "Chrome" while `navigator.userAgentData` client hints
  list "Chromium" without "Google Chrome". No LICENSE file in that
  repo — we did not copy its code, only implemented the same
  publicly-documented techniques ourselves.
- [FingerprintJS BotD](https://github.com/fingerprintjs/botd) (MIT) —
  evaluated and **not adopted**: covers Selenium/Puppeteer/basic
  headless well, but FingerprintJS's own docs say the open-source
  version does not reliably catch stealth-patched tools — the exact
  threat here. Not worth the extra client-side weight for coverage we
  don't need.
- Academic: "Detection of Advanced Web Bots by Combining Web Logs
  with Mouse Behavioural Biometrics" (Bournemouth, ACM Digital
  Threats 2021) and "FP-Agent: Fingerprinting AI Browsing Agents"
  (arXiv 2605.01247, 2026) both conclude that once a scraper drives a
  real browser, **network/fingerprint signals alone stop working** —
  behavioral biometrics (mouse movement entropy, timing) combined
  with the fingerprint layer is what catches what fingerprinting
  alone lets through.

### What shipped from this (see PROGRESS.md 2026-09-18)

Added to `challenge.go`'s browser probe: `__pwInitScripts` global
check, 800x600 exact-viewport check (1280x720 deliberately excluded —
too common a real window size, CLAUDE.md Section 14), and the
Chromium-without-Chrome client-hints brand check.

### What's still open (not built yet)

- **Behavioral biometrics** (mouse/click/scroll timing entropy) — the
  literature's actual answer to "real browser, stealth-patched" — not
  implemented. This is the next real signal layer, not a one-line fix.
- **Init-script timing attack** — Patchright's own admitted open gap.
  Nobody has published a concrete implementation; would need our own
  empirical research (timing distribution of real vs. Patchright-
  injected scripts) before it could ship. Logged as a research
  project, not started.
- Scrapling's `camoufox`-based real-Firefox mode not separately
  tested — same root cause expected (real browser engine), same gap.

---

## 2026-09-18 — How commercial vendors actually get to high block rates: layered, continuous

### Finding

Vendor research (Scrapfly's anti-bot bypass write-ups, Evomi's Kasada
analysis) is consistent on one point: no single technique gets a
vendor to a high block rate. Cloudflare/DataDome/Akamai/Kasada each
stack 5+ independent layers (TLS/JA4, HTTP/2 fingerprint, JS/browser
fingerprint, behavior, per-customer ML) specifically so that beating
one layer doesn't beat the product — "passing one layer means
nothing, you must pass all five simultaneously."

The other pattern worth copying at our scale: **continuous trust**,
not a one-time pass. Kasada's `p.js` re-solves a proof-of-work puzzle
every 60-180 seconds per session, with difficulty scaled by a trust
score that decays if the session behaves mechanically. A challenge
passed once does not mean trusted forever.

### What this changed for us

Checking our own challenge flow against that pattern surfaced a real
gap unrelated to Patchright: `guard.go`'s `Passed(r)` branch trusted a
challenge solve for the full 30-minute cookie lifetime with zero
further scoring, including velocity. Fixed same day — see
docs/PROGRESS.md 2026-09-18 "Passed-challenge sessions are now still
rate-limited."

### Not built (bigger projects, logged for the roadmap)

- **True periodic re-challenge** (Kasada-style: re-verify identity,
  not just rate, mid-session). What shipped only closes the
  rate-limiting half of "continuous trust" — the identity check is
  still one-shot per 30-minute cookie.
- **HTTP/2 fingerprinting** (pseudo-header order, SETTINGS/PRIORITY
  frames). Does not help against Patchright specifically (it drives a
  real Chrome network stack), but would catch a lightweight/non-browser
  Scrapling session the same way JA4 catches non-browser TLS stacks
  today. Go's `net/http` doesn't expose this without our own HTTP/2
  frame-level handling — nontrivial, not started.
- **Proof-of-work with scaling difficulty.** Our canvas/sha256
  challenge is fixed-cost; Kasada's scales cost with session
  suspicion. Would raise the economic cost of scraping at volume even
  when a request isn't caught outright.
- **Obfuscating the challenge JS itself.** It ships as plain,
  readable JS in `challenge.go` today — trivial for anyone motivated
  to read exactly what's being checked. Not a detection improvement
  by itself, but raises the cost of building a bypass in the first
  place.

Sources: [Scrapfly — How to Bypass Anti-Bot Protection in 2026](https://scrapfly.io/blog/posts/how-to-bypass-anti-bot-protection),
[Evomi — Kasada, Shape, and the Next Generation of Anti-Bot](https://evomi.com/blog/kasada-shape-and-the-next-generation-of-anti-bot-what-scrapers-need-to-know),
[Scrapfly — HTTP/2 and HTTP/3 Fingerprinting](https://scrapfly.io/blog/posts/http2-http3-fingerprinting-guide).

---

## 2026-09-20 — Open-source anti-bot landscape scan (what to learn from, what to take)

Scanned for currently-maintained open-source projects solving pieces of
the same problem, specifically to close the Patchright/Scrapling gap
logged in the 2026-09-18 entry above.

**Scope caveat:** this pass read READMEs, docs sites and package
metadata — **not source code**. Everything below is the projects' own
claims plus our assessment of fit. Nothing here has been code-reviewed,
so treat the technique list as leads to verify, not as verified facts.

### Ranked by usefulness to hakaishield

**1. `okasi/bot-signal`** — TypeScript, MIT, ~555★, ~13.6k npm
downloads/week, v2.0.14 (Aug 2026), active.
Closest match to our own architecture: weighted multi-signal scoring
with per-signal explainability, split across three layers (instant
browser checks, behavioural over time, server-side IP/TLS/timezone).
Three things worth lifting as *technique*:

- **CDP detection via `Error` serialization side-effect**, in both the
  page realm and a dedicated worker realm. This is the interesting one:
  it detects the Chrome DevTools Protocol itself rather than the
  globals a stealth tool scrubs, so it is a candidate for catching
  Patchright-class automation that defeats everything in our current
  set. Unverified against Patchright specifically — its own docs are
  careful here, saying generic environment anomalies never identify
  Patchright on their own and it only ever appears as an *alternative*
  attribution alongside a Chromium automation pattern.
- **An explicit false-positive carve-out list** — `isEmptyPlugins`
  skipped entirely on mobile Chrome (which legitimately reports none);
  in-app browsers, kiosk/F11 fullscreen and GPU-less VMs weighted
  0.25–0.45 so they only matter in combination. This is the single
  most directly useful part for us: we have no measured FP data
  (see "Known gaps" below), and this is a free list of the cases that
  bite in production.
- **Two cheap server-side signals we don't have:** browser-reported
  timezone vs GeoIP of the source IP, and datacenter IP range
  membership. Both fit `RequestFacts` directly.

Also notable: it scores `1 - Π(1 - wᵢ)` rather than summing weights,
with definitive markers at 1.0 and ambiguous ones at 0.25–0.45. Same
*principle* as our "no weak signal blocks alone" rule (CLAUDE.md §6),
reached by a different formula. Not a reason to rewrite `score.go` —
noted only so the next person doesn't think our additive scheme is
unconsidered.

**2. `WeebDataHoarder/go-away`** — Go, MIT, ~174★, created Apr 2025,
active. The only one in our language. Stated goal is close to our own
false-positive posture: "minimize impact to legit users, while
surgically targeting heavy endpoints or scrapers". Its README carries a
comparison table of the whole PoW-proxy ecosystem (Anubis, powxy, PoW!
Bot Deterrent, haproxy-protection) — useful as a landscape map. Its
rule/condition system is the closest existing art to ROADMAP item 11
(per-client rules).

**3. `WebDecoy/FCaptcha`** — Go + Node + Python, MIT, ~180★, created
Dec 2025. Broadest feature overlap with us (behavioural signals + TLS
fingerprinting + PoW in one system). The part worth studying is
**Web Bot Auth (RFC 9421)**: verifying a self-declaring agent
(ClaudeBot, GPTBot, PerplexityBot…) by checking its HTTP message
signature against the operator's published key directory, rather than
by reverse+forward DNS as `signals/goodbots.go` does today. That is
where good-bot verification is heading and it is a published RFC, so it
can be implemented from the spec with no licence question at all.
Younger and smaller than the others, so less battle-tested.

**4. `TecharoHQ/anubis`** — Go, MIT. The most production-proven of the
set (deployed in front of GNOME's GitLab, kernel.org and much of the
self-hosted FOSS world). Less to take than expected, because we already
have PoW and a signed challenge cookie. **Its value to us is its
documented failure:** difficulty was raised from 4 to 5 leading zero
bits — enough to make phones "uncomfortably warm", per its author — and
scrapers adapted and kept solving it. GNOME reported ~3.2% of requests
passing the challenge at all. This is direct evidence that a *fixed*
difficulty loses over time, which is the argument for the adaptive
scheme in "Cost asymmetry" below.

**5. `ttlns/brotector`** — JS/Python, MIT, ~277★, **last push Dec 2024
— not maintained.** Included despite that because it remains the best
single catalogue of raw webdriver-detection primitives:
`Runtime.enable`/`Console.enable` CDP leak, `Input.coordinatesLeak`
(crbug#1477537, bypassable with CDP-Patches), Playwright ≥1.46.1
init-script detection, injected-JS detection via stack-trace signature.
**Do not adopt its crash techniques** (popupCrash via crbug#340836884,
the Selenium script-injection crash): they crash the visitor's browser,
and a real person with DevTools open can trigger the same paths. That
is a product-breaking false positive, not a detection (CLAUDE.md §14).
`bot-signal` has already distilled the usable subset and documents
which of these it rejected as too noisy or too intrusive — prefer that
as the reference.

**Adjacent, different category: `Nepenthes`** (and its Python rewrite)
— an AI-crawler tarpit that serves an infinite maze of deterministic
Markov-babble pages, drip-fed byte by byte to hold the crawler open.
Conceptually close to our deception mode. **Not adoptable as-is:** its
own documentation warns that any site it is applied to will likely
disappear from all search results, because it cannot distinguish a
search indexer from an AI trainer. We have `IsVerifiedGoodBot`
specifically to protect customer SEO, so this is an idea to read, not
a component to ship.

### Cost asymmetry: bandwidth is the wrong lever, CPU is the right one

Question raised: scrapers must run real browsers, and browsers are
expensive at scale — so should we make pages *heavier* to tax them,
since a real visitor only loads the page once or twice?

The instinct (asymmetric cost) is right, the lever is wrong:

- **Egress is our cost, not theirs.** We are an inline hosted proxy;
  every injected byte is billed to us (CLAUDE.md §19). Deliberately
  inflating responses is a self-inflicted bill.
- **Real users are not insensitive to page weight.** Heavier pages hurt
  Core Web Vitals and therefore the customer's search ranking — the
  exact thing `IsVerifiedGoodBot` exists to protect — and cost
  conversions on mobile networks. A protection product that degrades
  the site it protects is a contradiction (CLAUDE.md §14).
- **It is trivially sidestepped.** A scraper already declines images,
  CSS and fonts for cost reasons; heavier pages just make it decline
  more. (That declining *is* itself a signal — see the asset-fidelity
  note under "Known gaps".)

CPU is the lever that survives all three: a proof-of-work challenge
costs us a few bytes of JavaScript, costs a clean visitor nothing
because they never see it, and costs an attacker real compute that
cannot be outsourced to a proxy pool. The refinement over what we ship
today is **difficulty scaled by suspicion and by repeat offence**
(clean → none; suspicious → light; a caller that keeps coming back →
rising), which is also the answer to Anubis's documented fixed-
difficulty failure above. mCaptcha pioneered variable-difficulty PoW
and Cap now offers a GPU-resistant PoW variant; both are worth reading
before we design ours.

### Licence position for consuming any of this

All five are MIT, which permits commercial and closed-source use and
modification — the only condition is preserving the copyright notice.
For hakaishield (closed-source, commercial, hosted) that means:

- **Techniques, signal lists, weights and known false-positive cases
  are facts and ideas** — not copyrightable, free to use, no
  attribution owed. This is where essentially all the value above sits.
- **Porting a specific file to Go is a derivative work**, not original
  authorship; the licence and its attribution condition follow the
  translation. Stripping the notice is the one move that converts a
  free, permitted use into infringement, and it is the kind of thing
  that surfaces in enterprise SBOM requests and acquisition IP
  diligence rather than in a lawsuit.
- If we ever do want lines rather than ideas (most plausible for
  `go-away`, being Go), the correct handling is a `NOTICES.md` /
  third-party licence file. That is cheap and keeps the option open.

Practical consequence: read these for technique, implement into our own
`RequestFacts`/`checks` shape with our own tests and false-positive
analysis. That also produces better-fitting code, since none of these
carry our tenant scoping or evidence-trail requirements.

### Known gaps this scan did not close

- **No source code was read.** Verify the CDP `Error`-serialization
  technique against a real Patchright session before weighting it.
- **We still have no measured false-positive rate.** Every threshold in
  `score.go` is an estimate, and the FP carve-out list above is
  borrowed rather than observed. A real site in shadow mode for two
  weeks would be worth more than any single new signal here.
- **Asset-fidelity correlation is still unbuilt.** A real browser
  rendering a page fetches its CSS, fonts and images; a scraper
  declines them to keep cost down. We already classify static assets
  (`signals/velocity.go`) and already track distinct-path breadth
  (`signals/pattern.go`), but nothing correlates "many HTML documents,
  almost no subresources" per session. That is a server-observed
  signal a real-browser stealth tool cannot fake without giving up the
  cost advantage that motivates scraping in the first place.

Sources: [okasi/bot-signal](https://github.com/okasi/bot-signal),
[WeebDataHoarder/go-away](https://github.com/WeebDataHoarder/go-away),
[WebDecoy/FCaptcha](https://github.com/WebDecoy/FCaptcha),
[TecharoHQ/anubis](https://github.com/TecharoHQ/anubis),
[ttlns/brotector](https://github.com/ttlns/brotector),
[Cap — open-source CAPTCHA comparison](https://github.com/tiagozip/cap),
[Nepenthes](https://nepenthes.online/),
[Pinggy — AI crawlers cost more CPU than real traffic](https://pinggy.io/blog/ai_crawlers_cost_more_cpu_than_real_traffic/).

## 2026-09-22 — "System One" models (TypeSafe Jev) and what they mean for scoring

TypeSafe AI launched **Jev** on 2026-09-15 ($40M seed, DCVC; founder Diogo
Almeida, ex-OpenAI). It is not a language model. Instead of generating text
token by token, it takes unstructured program state plus a typed question and
returns a typed value with a probability and a confidence score in one parallel
pass. Because the valid outputs are fixed by a schema up front, it cannot
hallucinate or emit a type error. Reported latency is 70–500 ms at roughly
$0.042 per million input tokens, pitched as 100–200x faster and cheaper than an
LLM for decision and classification work. Target use cases are real-time loops:
games, robots, simulations, and agent harnesses that need one judgement.

Community ports appeared within a week, all aimed at local Apple Silicon
inference and none official: `bnsd55/jevmlx` (Jev-style parallel constrained
decisions over any MLX model), `daseinlabs/open-jev` (one prefill, KV cache
expanded across an option batch, every option scored in one padded forward
pass), `laya-mlx` (MLX port of the Laya checkpoints, ~13 ms median), and
Core ML/ANE builds in Swift (`siren2345/jevlocal-mac`, `jev_apple_npu`,
`GodModeAI2025/JevCoreML`).

### Why none of this ships in hakaishield

Calling a hosted System One model per request is the wrong shape for us on
three counts, each a documented project rule:

- **Cost and latency (Sections 15, 19).** 70–500 ms of network round trip per
  request against a guard that currently spends ~36 µs end to end. It is also a
  per-request external API charge on traffic that is already our hosting cost.
- **Failure behaviour (Section 15).** An external dependency in the request
  path needs a timeout and a fail-open path, which hands anyone who can make
  the dependency slow a way to switch our scoring off.
- **Explainability.** Our differentiator is request-level evidence a customer
  can argue with. A hosted probability is not evidence we can defend.

The MLX and Core ML ports are Apple Silicon local inference. The backend is Go
on Linux. They do not apply.

### What is worth taking

The **shape**, not the model. The useful idea is that a decision system should
return a typed value with a calibrated probability and a confidence, rather
than a number a human has to interpret — and that the weights behind it should
be measured rather than guessed.

Our scorer already has that shape: `pkg/signals` turns a request into a small
set of fired binary checks, adds fixed weights, and compares the total to a
threshold. That is a linear model whose coefficients were chosen by hand. The
honest version of "build our own Jev" is therefore not a transformer, an MLX
port, or a hosted call. It is logistic regression over the checks we already
run, trained on our own labelled traffic, inferring in-process in Go.

That is what `pkg/decide` implements: 14 ns per request, zero allocations, no
network, and a per-feature contribution breakdown that keeps the evidence trail
intact. See `docs/DECISIONS.md`, "Learned decision weights are a linear model
over existing signals".

Sources: typesafe.ai/blog/introducing-system-one-models-and-jev;
thenewstack.io/typesafe-jev-system-one; tomshardware.com (2026-09);
datacamp.com/blog/system-one-models-jev; en.wikipedia.org/wiki/Jev_(AI_model).

## 2026-09-22 — How the large vendors deploy, and what bandwidth actually costs

Researched while deciding where to host. Full write-up and the deployment
reasoning are in `docs/DEPLOYMENT.md`; this entry records the findings that
affect detection design rather than hosting.

### The architectural difference that matters most

**DataDome and Akamai do not carry the response bytes.** DataDome deploys as a
module — an Akamai EdgeWorker, a CloudFront Lambda@Edge function, a Fastly or
nginx module — which makes a *sideband* call to the nearest DataDome endpoint
with request metadata over a keep-alive connection, gets a verdict in about
2ms, and then the CDN serves the content. DataDome's own infrastructure never
sees the page body. Akamai's Bot Manager reaches the same outcome from the
other side: detection runs on the hop that was already delivering the traffic.

hakaishield is a full reverse proxy, so every byte of every response crosses
our network interface and is billed as egress. At AWS's $0.09/GB that is about
$81/month at 10M requests (100KB average response) and roughly $6,900/month at
1B. Compute is a rounding error beside it.

Two consequences worth holding onto:

- **The proxy model is why our evidence is better.** We see the entire request
  rather than the summary someone else chose to forward. That is not a cost to
  eliminate; it is what the product sells.
- **A sideband decision API is a real future option for high-volume
  customers**, not a replacement for the proxy. Recorded as a roadmap item
  rather than left to be discovered on a bill.

### Cloudflare and Akamai scoring architecture

Cloudflare assigns every request a bot score of 1–99 from layered engines:
Heuristics (1 for high-confidence, 29 while confidence is still being assessed),
Machine Learning (2–99, the majority of detections), JavaScript Detections for
headless and automation fingerprints, and a deprecated Anomaly Detection engine.
Alongside the score they expose Bot Score Source, Detection IDs and Bot Tags.
Akamai scores 0–100 and groups responses into Cautious / Strict / Aggressive
bands the customer tunes.

This is the same shape as `pkg/signals` plus `pkg/decide`, which is
reassuring — and the gap is still where we thought it was. They expose a
*tag* ("detection ID 1234, `automated_browser`"). `decide.Explain` exposes each
signal's contribution in log-odds with arithmetic a customer can check. That
difference survives contact with what the category leaders actually ship.

What is not worth copying: their scale of data. 40 billion bot requests a day
(Akamai) and 5 trillion signals (DataDome) are not a target we can reach or
should chase.

### Bloom and cuckoo filters — if a large blocklist is ever built

Relevant because a maintained fingerprint/IP intelligence set is the roadmap's
stated moat (item 19), and membership checks are how it would be queried.

**Cloudflare's "When Bloom filters don't bloom"**: a Bloom-filter deduplicator
over ~1 billion IP records ran in 12 seconds where hashing alone took 2. The
filter operations cost 10 seconds, and the cause was random memory access — a
bit array larger than cache turns every probe into a cache miss. A Bloom filter
sized past L2/L3 is far slower than its arithmetic predicts.

**Cuckoo filters** (Fan et al., CoNEXT 2014) are the better modern default:
deletion is supported, they are more space-efficient than Bloom below a 3%
false-positive rate, and any lookup touches at most two cache lines, so cost is
predictable for both hits and misses. A space-optimised Bloom filter at 1% FPR
needs 7 probes, each a potential miss.

**The rule that matters most here** is stated directly in the Perfect Cuckoo
Filter paper (CoNEXT 2021): if a filter decides whether to *block* an IP, a
false positive disables communication from a legitimate address, and the
authors name this as a case where a plain Bloom filter cannot be used.

That is `CLAUDE.md` §14 derived independently by network researchers. So if a
large blocklist is built here:

  - the filter is a fast **negative** — "definitely not in the list, stop"
  - a positive is a **hint**, never a verdict; confirm against the real list
  - size it to fit in cache, or measure and be disappointed

It is the no-lone-signal rule in a different domain.

## 2026-09-24 — Challenge browser probe limits

The `inspired/vertex/src/checks.ts` legacy key list contains exact globals
from several automation and embedded-browser families. These names were
adapted as observational challenge telemetry; merely finding one truthy
property is insufficient to reject a visitor.

The [WebGPU specification](https://www.w3.org/TR/webgpu/#features) defines
`shader-f16` as optional. [Chrome's WebGPU 120 notes](https://developer.chrome.com/blog/new-in-webgpu-120)
also say some hardware lacks 16-bit support. Therefore the prior
"Chrome 113+ hardware always supports f16" premise is false; the probe is
limited to Chromium 120+ and remains shadow-only. A missing GPU API, missing
adapter, or lookup timeout is neutral. No detection percentage is inferred
until real labelled client traffic is measured.

Sources are listed at the end of `docs/DEPLOYMENT.md`.
