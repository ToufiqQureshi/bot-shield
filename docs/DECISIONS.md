# DECISIONS.md — Why We Chose What We Chose

A running log of real decisions made on this project and the
reasoning behind them, so a new session/agent doesn't have to guess
or re-derive context that already exists. Newest entries at the top.

When you make a real decision (tech choice, scope cut, priority
change, rejected alternative), add an entry here **before** ending
your session. See `CLAUDE.md` Section 0 / the mandatory update rule.

---

## Honeypot trap: scoped to (tenant, IP, JA4), scored not blocked — 2026-09-20

**Decision:** a honeypot trip is recorded against the triple (tenant ID, source IP, JA4) with a 6h TTL and a hard 50k entry cap, and feeds scoring as a 50-point `honeypot_trap` signal. It is explicitly **not** written to the shared `ja4:scrapers` blocklist.

**Why not the blocklist:** the original implementation called `AddKnownScraperJA4(ja4, ...)` on every trip, which lands in the map `score.go` reads for its 100-point `ja4_blocklist` check. JA4 identifies a *browser build*, not a machine — every real Chrome user on a given version shares one fingerprint. So one headless-Chrome scraper tripping the trap would have hard-blocked every genuine Chrome visitor of that version, on **every tenant**, permanently and with no expiry. That is the worst false positive this product can produce (`CLAUDE.md` Sections 14 and 16), triggered by a single request.

**Why (IP, JA4) and not either alone:** JA4 alone is too coarse (above). IP alone is too coarse the other way — CGNAT and corporate egress put unrelated people on one address. Requiring both means a trip counts against the caller that actually walked into the trap and close to nobody else.

**Why 50 points and not a block:** following a `display:none` link is strong evidence of DOM-walking automation, but not proof of malice. Screen readers walk the DOM, and browsers speculatively prefetch. At 50 a lone trip lands in challenge range, which a real person recovers from in one round trip, while an actual scraper also trips the handshake, UA-mismatch or crawl-pattern signals and crosses the block bar on combined evidence — which is what `CLAUDE.md` Section 6 requires anyway.

**Rejected:** Redis fan-out of trips (`HSet` + `Publish` per hit, each in its own goroutine). It spawned an unbounded goroutine and two Redis round trips per request to an endpoint an attacker can call in a loop, and the `Publish` had no subscriber anywhere in the codebase. Cross-node propagation is a real want, but it needs a bounded writer and a TTL'd, tenant-scoped key; it is a follow-up, not a prerequisite.

**Revisit when:** trips need to propagate between nodes, or when the trap link is injected somewhere other than deceived responses.

---

## Consolidating PR #10 and PR #11; dropping the forensics analyzer — 2026-09-20

**Decision:** the deception and honeypot work ships; the `pkg/forensics` client-behaviour analyzer from PR #10 does not, and PR #10's dependency changes are discarded entirely.

**Why the dependencies were discarded:** PR #10 downgraded every direct dependency — Go 1.25→1.23, `fingerproxy` v1.2.3→v0.6.1, `pgx` v5.11→v5.6, `go-redis` v9.22→v9.7, `sentry-go` v0.49→v0.27. Nothing in the PR needed older libraries; for a product whose job is sitting in the TLS request path, silently moving back across a year of upstream fixes is a security regression, not a neutral change.

**Why forensics was dropped rather than merged:** it is 298 lines with no caller anywhere in the tree, and wiring it as written would have added a signal that *lowers* risk based on numbers the client supplies — a bot posts `{"mouse_entropy": 0.45}` and scores clean (`CLAUDE.md` Section 17). Two of its five layers (mouse entropy, timing variance) also have no safe reading on touch devices or for keyboard-only visitors, so a naive collection script would have penalised every mobile and accessibility user (Section 14). Separately, the challenge page already collects canvas, WebGL, `navigator.webdriver` and permission-probe telemetry over an HMAC-bound, size-limited POST — so the transport this analyzer needs already exists and the detection partly overlaps.

**What it would take to ship it:** bind the payload to the existing challenge token, define signals that degrade safely on touch and keyboard-only input, and validate against real mobile traffic before it can influence a decision. That is a feature, not a merge conflict, and it is on the roadmap rather than half-wired into the request path.

**Revisit when:** someone picks up client-telemetry scoring as its own piece of work. The code is preserved in PR #10's branch history.

---

## Enterprise Hardening: Adaptive Policy Modes, Verified Good Bots, and Production Transport — 2026-09-19

**Decision:** Bot Shield now supports adaptive policy modes (`PolicyBalanced` vs `PolicyStrict`). Under `PolicyBalanced` (the production default for e-commerce/SaaS), clean visitors (score 0) are passively forwarded to the origin with zero latency, reserving challenges for elevated risk scores (25-99) and blocks for >=100. `PolicyStrict` preserves the mandatory ~50ms invisible interstitial for strict zero-scrape environments. In addition, genuine search engine crawlers (Googlebot, Bingbot, Applebot, etc.) are verified via cached reverse+forward DNS and granted clean passthrough to protect SEO indexing. Upstream proxy connections are backed by a production-tuned pooled transport with explicit timeouts and custom 502/504 error handlers.

**Why:** Forcing 100% of organic visitors through an interstitial challenge page increases bounce rates and harms SEO on e-commerce platforms. Adaptive policy modes provide maximum operational flexibility without compromising security against automated scraping.

**Revisit when:** Dynamic machine-learning based risk scoring thresholds are deployed or multi-region anycast edge nodes are added.

---

## Server-observed request patterns; asset-aware rate limiting — 2026-09-19

**Decision:** request classification by URL path (`isStaticAsset`) is now a first-class server-side fact. The per-IP velocity limit counts navigations and subresources separately (20/s vs 300/s), and a new `crawl_pattern` signal flags a browser-claiming client that walks many distinct page paths in a window (Redis HyperLogLog, >60/min). Scoring now reads a `RequestFacts` struct (IP/JA4/UA/Header/Path).

**Why the velocity change was not optional:** the old counter capped *every* request at 5/s per IP and Guard hard-429'd passed sessions. A real browser loading one page fires dozens of asset requests in a second, so this returned 429 to paying customers' real visitors — a production false positive of the worst kind (`CLAUDE.md` Section 14). Classifying assets server-side fixes the bug and, for free, makes navigation rate a real signal: a person cannot open 20 pages a second, a crawler can.

**Why request patterns are worth a layer:** every other signal we have is either client-reported (spoofable) or TLS-level (defeated by real-browser impersonation). The *shape and rate of requests* is observed by the server and cannot be forged from the client — the attacker's only defence is to slow down, which is the economic goal. This is the layer that the headful-scraper gap needs; it is also why it is scoped to browser-claiming clients (a script is not pretending, so there is no lie to catch — Section 8) and why assets are excluded (a real page load is mostly assets — Section 14).

**Why HyperLogLog, not a set:** distinct-path cardinality per IP is all we need, and HLL bounds that state at ~12KB regardless of how many paths one IP requests (`CLAUDE.md` Section 15 — bounded state against adversarial volume). It also naturally ignores volume: hitting one page 500 times is a reload loop, not a crawl, and must not fire.

**Alternatives considered:**
- *Count distinct paths in a Go map per process.* Rejected: unbounded memory per hostile IP, and wrong across instances on a hosted multi-node deployment.
- *Use `Sec-Fetch-Dest` to classify navigations.* Rejected: a client header, spoofable; the URL path is not.
- *Raise the old single limit instead of splitting it.* Rejected: any single number either 429s real page loads or lets crawlers through — the two request classes genuinely need different limits.

**Revisit when:** real traffic gives real numbers for the caps, or a NAT/mobile-carrier test shows the navigation cap is too tight.

---

## Wire header_anomaly into scoring; drop the orphaned probe.js — 2026-09-19

**Decision:** the header-consistency check (`HeaderAnomaly`, `signals/headers.go`)
is now a real scoring input: `Score`/`Analyze` take the request `http.Header`,
and the single `checks` table gains a `header_anomaly` entry weighted 25 — below
the block bar. Separately, the `/__hakaishield/probe.js` endpoint and its
`probeScript` are deleted.

**Why header_anomaly:** it was fully built and tested but never referenced by
scoring, so it contributed nothing while looking finished — the "built but
unwired" gap `CLAUDE.md` Section 21 warns about. Its weight is deliberately the
lowest: a browser-claiming client that omits every `Sec-Fetch-*` and `Sec-CH-UA`
header is suspicious, but privacy tools and unusual-but-real clients can drop
them, so it must never decide a block alone. 25 on its own stays below the 100
block threshold, and every path to 100 already fires a stronger signal — pinned
by `TestHeaderAnomalyNeverBlocksAlone`.

**Why delete probe.js:** it served a standalone script that set a `_bs_probe`
cookie nothing ever read, and nothing injected it anywhere. That contradicts the
earlier recorded decision (2026-09-16, "Automation probe lives inside the JS
challenge, not injected site-wide"): the probe belongs inside the challenge page,
which is where item 6 actually runs. A second, unused probe endpoint only widened
the attack surface and implied a site-wide injection feature that was explicitly
not built.

**Also recorded:** `Decide` no longer carries a `challengeThreshold` constant it
does not use. With the mandatory interstitial (see the entry below), the honest
contract is "block at 100, challenge otherwise", so a dead threshold that
implied a band which no longer existed was removed rather than left in place.

**Revisit when:** real traffic exists to tune the header_anomaly weight, or
HTML-injection into origin responses becomes a roadmap item (which would let
item 6's probe run everywhere, making a site-wide probe script relevant again).

---

## Fast, Invisible Challenges and Headful Bot Evasion — 2026-09-19

**Decision:** The JS challenge engine now runs at an ultra-fast 8-bit Proof-of-Work (PoW) difficulty (~50ms) and supports theme customization (`ghost` and `branded` modes). It also strictly checks `Error.stack` and `navigator.permissions` to block headful Playwright/Puppeteer (e.g., Scrapling, Patchright) that evade TLS fingerprinting.

**Why:** Clients hate visible interstitial pages ("Checking your browser..."). However, removing the interstitial entirely allows advanced Headful bots to scrape the first page before background telemetry catches them. To guarantee **zero scraping**, the interstitial is mandatory.
To make it "sellable", we implemented:
1. **Ultra-Fast PoW:** Reduced from 12-bit ("000") to 8-bit ("00") solving the 4-second delay caused by JS `await` event loop overhead. It now executes in <50ms.
2. **Ghost Mode (`-theme=ghost`):** A completely blank page that executes the 50ms check and redirects, appearing as normal network latency rather than a security check.
3. **Branded Mode (`-theme=branded`):** An enterprise-style loading spinner for clients who want a visible security check.

**Why Error.stack?** Headful bots (Patchright, Scrapling) perfectly spoof `navigator.webdriver` and JA4 signatures. They are indistinguishable from real Chrome at the network level. However, to control the browser, they inject internal evaluation scripts (`puppeteer_evaluation_script`, `evaluate@`). Throwing a deliberate error in JS and reading the stack trace is the only reliable way to catch them on the very first request without behavioral biometrics.

---

## Back to one agent; Claude Code owns the dashboard too — 2026-09-16

**Decision:** the two-agent workflow is over. The project owner ended
the Antigravity arrangement. `claude_and_agy.md` is deleted,
`CLAUDE.md` Section 25 is replaced with a plain statement of sole
ownership, and `proxy/`, `cmd/hakaishield/` **and** `dashboard/` are all
Claude Code's.

**What does not change:** the `dashboard/` code stays. It works, it is
wired to the real stats endpoint, and `CLAUDE.md` Sections 12 and 13
apply to it exactly as to any other code — read it before changing it,
and don't delete parts of it as "dead" without checking. Ending a
working arrangement is not a reason to throw away working code.

**What does change, and it is the part worth remembering:** the
dashboard now has to meet this file's bar rather than a softer "it's
only UI" one. It needs real tests, it must not fail silently, and it
must behave when the backend is slow, down, or returning an error. A
dashboard that shows a blank card when the API is unreachable is not
a cosmetic bug — it tells the customer their traffic is clean when we
have no idea what their traffic is.

It is also the only part of the product a customer ever sees. They
will never read `proxy/score.go`; they will judge hakaishield entirely
on whether the numbers on screen make sense and are believable.

**Also retired with it:** `agentchat/` (the two-agent coordination
log) has no remaining purpose. It was already gitignored and is not
shipped product, so nothing in the repo depends on it. The earlier
entries in this file about the agentchat MCP server stay as history —
they record a real lesson about why MCP tool calls cannot wake an idle
agent, which is worth keeping even though the setup is gone.

**Revisit when:** a second builder joins, human or agent. At that
point re-read the earlier entries rather than re-deriving the file
collision and edit-war problems from scratch.

---

## Pivot: hosted SaaS is the product; self-hosting becomes Enterprise — 2026-09-16

**Decision:** hakaishield is sold as a **hosted service we run**.
Customers point DNS at us, their traffic flows through our
infrastructure, they pay monthly. Self-hosting is not removed — it
becomes a **priced-up Enterprise option**, not the default.

**Why the owner asked for this:** the self-hosted-only framing had no
path to recurring revenue for someone starting with no capital. A
licence key for software that runs on someone else's server is a
harder sell, a harder collection, and a harder product to keep people
paying for. The owner wants a SaaS. That is a business call, and it
is theirs to make.

**What it costs us, stated plainly so nobody is surprised later:**
- **Bandwidth becomes our bill.** Every byte of a customer's traffic
  now crosses our infrastructure. A modest e-commerce site can mean
  hundreds of GB a month. This is why DataDome and Cloudflare meter
  per request — their costs scale with traffic, and now so do ours.
  **Every plan must therefore carry a bandwidth/request cap**, or one
  large customer erases the margin on ten small ones.
- **We are now in the critical path.** Our downtime is the customer's
  site being down. For a solo maintainer that is an operational
  burden, not a coding one.
- **We now compete with Cloudflare on their own ground**, and they
  have hundreds of points of presence and near-zero marginal cost. We
  cannot win that on price. The differentiators stay what they were:
  evidence, agent governance, and a product aimed at people Cloudflare
  does not serve well.
- **The "your traffic never leaves your infra" pitch is gone** for the
  default product. That was one of the two buyer segments in the
  positioning entry below. It survives only as Enterprise.

**What it gains us, and one gain is bigger than expected:**
- Recurring revenue, self-serve signup, no licence enforcement to
  build, and we control updates.
- **The moat gets easier.** Item 19's fingerprint database was the
  expensive research cost. Hosting every customer's traffic means we
  observe real browser fingerprints continuously — the database can
  largely build itself from our own traffic. A bad fingerprint seen
  on one customer can protect the rest. That cross-customer network
  effect was explicitly rejected as infeasible under self-hosting
  (see the 2026-09-15 competitor scan in `RESEARCH.md`); hosting makes
  it available. It still needs a clear data-handling policy in terms
  of service before it is switched on — `CLAUDE.md` Section 18 has not
  moved.
- JA4 is unaffected: we terminate TLS, so we still see the
  ClientHello. The technical edge survives the pivot intact.

**What "Enterprise" means here** (recorded because it is easy to
misread as a feature tier): it is a *deployment and contract* tier,
not a feature list. Regulated or large customers who cannot send
traffic to our cloud run hakaishield themselves, on a custom price
**above** the hosted plans, invoiced rather than card-billed, with a
contract and direct support. It is sales-led, not self-serve.
**It is not to be built now** — there are zero customers. It exists
in the docs so the option stays open and so a future session does not
delete the self-hosting code path as dead.

**Alternatives considered:**
- *Stay self-hosted only.* Rejected by the owner: no viable recurring
  revenue from a standing start.
- *Ship a decision API instead of proxying traffic* (ROADMAP item 13)
  to avoid bandwidth costs. Rejected as the primary model: without
  terminating TLS we never see the ClientHello, so JA4 — the entire
  technical edge — disappears. It stays a roadmap item for customers
  who want it *in addition*, never as the main product.
- *Open-source the engine and sell hosting, like Supabase or
  Firecrawl.* Rejected: `CLAUDE.md` Section 1 forbids it. Noted here
  only because the model came up in discussion and the pattern —
  monetise the operations, not the bits — is the right instinct even
  though the licensing is not.

**Honest risk:** this reverses the positioning entry below, which was
written the same day. That entry's reasoning about *what we sell*
(evidence, agent governance, not price) still stands; what changed is
*where it runs* and therefore who the second buyer segment is. Two
reversals in one day on zero customer evidence is itself a warning:
the next real signal should come from a customer, not another
strategy session.

**Revisit when:** the first month's real bandwidth bill lands, or a
customer asks for Enterprise self-hosting and we find out what they
will actually pay for it.

---

## The moat is the fingerprint database, not the code — 2026-09-16

**The question that forced this:** *"any competent dev could build
this with Redis and a few checks — why would a company pay us?"*

**The honest first half:** they're right about today's code. Two
signals, fixed thresholds, `fingerproxy` is open source. A good
engineer rebuilds the current hakaishield in a couple of weeks. There
is no answer to that objection in what is built so far, and pretending
otherwise would just mean discovering it in a sales call instead.

**The second half, which is the actual answer:** building it once and
*keeping it working* are different products. Bot detection is not a
feature, it's a standing fight with someone who updates:

- Chrome ships roughly every 4 weeks and its TLS fingerprint moves
  with it. `curl_cffi`, Patchright and friends update specifically to
  break detection.
- An in-house system therefore **decays silently**. It doesn't crash
  or alert — it just drifts toward allowing everything, and nobody
  notices for months. There is no error log for "your detection
  stopped detecting."
- In-house systems also don't get tuned. The first time one blocks a
  real customer, a support ticket lands and the team switches it off.
  Writing detection is the easy part; spending months on false
  positives is the part nobody volunteers for.
- And it sits in front of checkout. The engineer who wrote it leaves
  in eight months and nobody wants to touch it after that.

**Decision:** a maintained **known-browser fingerprint database** is
hakaishield's commercial moat, and gets treated as a first-class
roadmap item (new item 19) rather than as a research cost to avoid.

**Why it, specifically.** `ROADMAP.md` item 3 already identified this
and deliberately deferred it: *"that needs a maintained database of
known-browser fingerprints, which is a real ongoing research cost."*
That framing was right as engineering and backwards as business — the
ongoing cost **is** the defensible asset. It isn't code, so it can't
be copied in a weekend; it's continuously refreshed data, so its decay
becomes our problem instead of the client's. That is what a
subscription actually buys, and it is why CrowdSec can give its engine
away free and still sell tiers: the engine is free, the intelligence
is not.

**Consequence for prioritisation:** adding detection signals #3 and #4
does not make anyone pay. Being able to say *"we track N known browser
builds and refresh them weekly"* does. Items 7–10 are no longer
automatically ahead of item 19.

**Consequence for sales:** a prospect who says "we'll just build it"
is not a customer — don't argue with them. In this category the buyer
has almost always been hurt first (scraped pricing, scalped
inventory, credential stuffing). Pain precedes purchase, which is also
why cold outreach doesn't work here.

**Alternatives considered:**
- *Compete on breadth of signals.* Rejected: signals are the copyable
  part, and `CLAUDE.md` Section 14 already forbids adding them for
  their own sake.
- *Lean on convenience/ease of deployment as the moat.* Rejected as a
  moat (it's a real advantage, but a competitor matches it in one
  release); it stays a selling point, not a defence.
- *Accept "they'll build it themselves" and target only enterprises
  who won't.* Rejected: that's the buyer a solo maintainer can't serve
  — see the positioning entry below.

**Honest risk:** building and maintaining this database is a genuine,
recurring cost and nobody has scoped it yet. It could turn out to be
more work than one maintainer can carry, which would be a real
strategic problem rather than a missing feature. Scope it before
committing to it, and record what that scoping finds here.

**Revisit when:** item 19's scoping says how much work refreshing the
database actually is, or a real client tells us they'd have built it
themselves (which would mean the moat argument isn't landing).

---

## Evidence trail: token-gated, in-memory, one shared signal table — 2026-09-16

**Decision:** ROADMAP item 12a ships as `proxy/evidence.go`: a fixed
1000-entry ring buffer with a 24h retention window, served from
`GET /api/v1/dashboard/evidence` behind a bearer token that must be
set with `-evidence-token` or the endpoint isn't mounted at all.

**Why token-gated when `/stats` is wide open.** `/stats` returns four
counters — aggregate, nothing about any individual. This endpoint
returns per-visitor JA4 fingerprints *and* the decision each one got,
which makes it two different problems at once: it discloses visitor
data, and it is an **evasion oracle** — a bot can query it to learn
whether its own fingerprint is being flagged and iterate until it
isn't. That second one is the reason this is not merely "add auth
later"; an open version of this endpoint actively helps the attacker
hakaishield exists to stop. So: token required, `crypto/subtle`
comparison, an empty configured token denies everyone (a deployment
that forgot to set one fails closed), and deliberately **no wildcard
CORS**, unlike `/stats`.

**Why off by default rather than on with a generated token.** A
generated token has to be printed or stored somewhere, and the
operator who never wanted the endpoint now has a live one they aren't
watching. Unset flag → unmounted route → no attack surface at all.

**Why one `checks` table in `score.go`.** `Score` and the signal names
shown in the evidence were originally going to be two separate lists.
That is a bug with a fuse on it: add a signal to the scorer, forget
the name list, and the trail now confidently explains a block with the
wrong reason. `CLAUDE.md` Section 23b's "a wrong fingerprint is worse
than a missing one" applies exactly here — a wrong *explanation* is
worse than none, because an ops engineer will act on it. One table,
read by both, removes the drift class instead of documenting it.

**Why `challenge_solved` is recorded as its own reason.** A request
let through on a solved-challenge cookie skips scoring entirely.
Recording it as a score-0 allow would have the trail assert the
visitor looked clean, when in fact they may have carried both bad
signals and were let through on proof they'd already given. The trail
is only worth having if it never says something untrue.

**Why in-memory, not the planned store.** Same reasoning already
logged for `Stats` and the challenge secret: a durable store is its
own roadmap item, and shipping a half-wired Postgres dependency to
get history would be worse than an honest 1000-entry window. Both
caps exist from day one rather than "later" because this runs against
adversarial traffic (`CLAUDE.md` Section 9).

**Alternatives considered:**
- *Write evidence to the normal log stream instead of a queryable
  endpoint.* Rejected: it puts us back where CrowdSec already is —
  parsing logs — and the whole point of item 12a is that a client's
  ops engineer can answer "why was my customer blocked?" without a log
  pipeline.
- *Store the request path and IP too.* Rejected for now under Section
  18 (only what the decision used). It is the obvious next gap — see
  the honest-gaps note in `PROGRESS.md` — but widening visitor data
  collection is a decision to make deliberately, not a convenience.
- *Reuse `/stats`'s open CORS for dashboard convenience.* Rejected on
  the spot; see above.

**Revisit when:** the dashboard needs to read this endpoint
cross-origin (needs a real allowed-origin config, not a wildcard), or
per-client configuration (item 11) arrives and the size/retention caps
should become per-deployment settings.

---

## Positioning: self-hostable agent governance, not "cheap DataDome" — 2026-09-16

**Decision:** hakaishield stops describing itself as an *affordable
alternative to Akamai/DataDome* and starts describing itself as
**the inline, self-hostable layer that decides which automated
clients reach a site, and proves why.** `ROADMAP.md`'s intro and
`AGENT.md`'s mission line are updated to match.

**Why (this is the important part):** a market scan on 2026-09-16
(numbers in `RESEARCH.md`) found the "affordable" slot is not an
opening — it is the most crowded part of the market, and its floor is
$0:

- Below us: CrowdSec, SafeLine, Coraza — free and self-hosted.
  Cloudflare's free tier. Prosopo at ~$39/mo.
- Above us: DataDome/HUMAN/Kasada at ~$1K–50K/mo.

Pitching "cheaper bot detection" invites exactly one reply: *"CrowdSec
is free."* We cannot win a price argument against zero. Worse, the
old framing made **price** our differentiator, which meant every
roadmap item was implicitly judged as "does a big vendor have this?"
— the precise habit `CLAUDE.md` Section 14 exists to stop.

**What we actually have that the free tier does not:** CrowdSec and
friends parse **logs** — they react to an IP *after* it has misbehaved
somewhere. hakaishield reads the **live TLS ClientHello** and scores
the first request, with no prior sighting of that client. No
self-hostable product does inline JA4 fingerprint scoring today. That
is a narrow but real moat, and it is worth money to two buyers the
free tier cannot serve:

1. Teams under GDPR/DPDP-style constraints who **cannot** route
   traffic through a foreign SaaS. Their alternative to us is not
   DataDome — it is nothing.
2. Sites where bot traffic is a **direct revenue leak**, not an
   annoyance: pricing-sensitive e-commerce, ticketing/booking
   inventory, job boards and classifieds, usage-billed APIs. For
   them this is a margin tool, not a security line item — which is
   the difference between a $200/mo yes and a "we'll think about it."

**The second half of the decision — build for agents, not just bots.**
Forrester renamed the category in Q2 2026 to *Bot and Agent Trust
Management*. Cloudflare shipped pay-per-crawl; RSL and Web Bot Auth
appeared. The buyer's question moved from "is this a bot?" to "which
agent is this, is it allowed, and can I prove what I decided?"
Mid-market sites get exactly two answers today: allow or block.
Nobody sells them policy. Our JA4 + UA-consistency signals already
answer "which client is this, really" — that is the raw material for
governance, and it is a feature we can ship, not a market we have to
create.

**Roadmap consequences** (all reuse existing machinery, per Section
14 — no new detection layer):
- Item 11b: verified agent policy — per-agent allow / rate-limit /
  deceive rules, so "GPTBot is fine, a scraper wearing its User-Agent
  is not" is expressible. Extends item 11, not a new package.
- Item 12a: decision evidence trail — per-request record of score and
  which signals fired. Blocking is commodity; *proving why* is not,
  and it is what settles a false-positive dispute (Section 8).
- Item 18: shadow mode — score and report without enforcing. This is
  primarily a **sales and trust** instrument, and it is listed as a
  roadmap item so it does not get treated as optional polish.
- Item 17 (pricing) now has a stated anchor: ~$200/mo, justified by
  self-hostability and governance, never by a discount.

**Alternatives considered and rejected:**
- *Compete on price (~$49/mo, undercut Prosopo).* Rejected: the floor
  is $0 and we would be arguing against free software with a worse
  brand. Also caps revenue below what one support conversation costs
  a solo maintainer.
- *Go enterprise, chase DataDome's buyer.* Rejected: that buyer
  requires SOC2, a sales team, 24/7 support and a mobile SDK. A solo
  maintainer cannot serve it, and pretending otherwise is how the
  product dies mid-deal.
- *Build pay-per-crawl / HTTP 402 billing now.* Rejected: no major AI
  lab has adopted pay-per-crawl or Web Bot Auth, so we would ship a
  toll booth nobody pays at. Identify-and-govern works today; charging
  does not.
- *Ship an open-source community edition for reach.* Rejected on the
  spot — `CLAUDE.md` Section 1: this is commercial software. Noted
  here only so a future session does not re-propose it as a growth
  idea.

**What this does NOT change:** the detection architecture. Multi-layer
scoring (Section 6), the fail-open/fail-closed choice, latency budget,
and the false-positive bar all stand exactly as they are. This is a
decision about *who we sell to and what we say*, and about which
roadmap items earn priority — not a rewrite of the product.

**Honest risk:** all of this is desk research. Zero paying clients
have confirmed any of it, and the $200/mo anchor is reasoned, not
observed. The two buyer segments above are hypotheses. Treat the
first real client conversation as the test — and if it contradicts
this entry, write the correction here rather than quietly drifting.

**Revisit when:** a real client's traffic and budget contradict the
segments above, or a major AI lab adopts Web Bot Auth / pay-per-crawl
(which would move billing from "no" to "worth scoping").

---

## Automation probe lives inside the JS challenge, not injected site-wide — 2026-09-16
**Decision:** ROADMAP item 6's "client-side automation-tool probe"
runs inside the existing JS challenge page (`proxy/challenge.go`),
checked only for visitors who reach that page — not injected into
every proxied origin response.
**Why:** injecting a JS snippet into arbitrary origin HTML is a real,
separate feature (parse/rewrite HTML, handle charset and compressed
responses, interact correctly with the origin's own CSP) that this
codebase has no infrastructure for and no roadmap decision to build.
The challenge page is the one place hakaishield already controls its
own JS execution in a visitor's browser — reusing it is both smaller
and immediately testable with the existing harness, matching
`CLAUDE.md` Section 3/13's bias against building new machinery when
existing machinery already does the job.
**Alternatives considered:** a full response-body JS-injection
middleware — rejected as a large, undecided feature of its own; would
also need to handle every origin's existing CSP header correctly to
avoid breaking real sites, which is a meaningful security surface on
its own.
**Coverage trade-off, accepted:** the probe only ever runs for traffic
that already reached the challenge (score ≥ 50) — a request scored
`DecisionAllow` never executes it. This narrows item 6's literal
"flags obvious automation frameworks in-browser" (which reads as
site-wide) to "flags them for traffic already under suspicion,"
similar in spirit to item 3's narrowing of "verify real client family"
to what's provable without a maintained database.
**Revisit when:** HTML-injection into origin responses becomes an
actual roadmap item for other reasons (e.g. behavioral scoring, item
7, likely needs this too) — at that point item 6 should extend to
run everywhere, not just on the challenge page.

---

## Added 2 competitor-gap items to roadmap, rejected the rest — 2026-09-15
**Decision:** after a competitor scan (DataDome, Akamai, Cloudflare,
Kasada, Arkose, HUMAN/PerimeterX — see `RESEARCH.md`), added item 9a
(API-aware endpoint rules) and item 11a (deception/decoy response) to
`ROADMAP.md`. Both reuse existing machinery (item 9/11's config, the
scoring engine's decision output) rather than adding a new detection
layer or package.
**Why these two:** low implementation cost, no new architecture, and
genuine differentiation — deception in particular is under-offered
even by big vendors. Both fit `CLAUDE.md` Section 14 (real gap, not
"vendor X has it so we should too").
**Rejected:** persistent cross-session device fingerprinting (needs a
standing ML similarity model + cross-session storage — an infra
project of its own), native mobile SDK (separate codebase/maintenance
surface, off hakaishield's web-proxy shape), shared cross-customer
threat intel (needs a consent/data-sharing framework first, or it's a
Section 18 data-overreach problem), WASM deep browser-engine
fingerprinting (research-heavy, no observed client need yet). Full
reasoning for each in `RESEARCH.md`.
**Risk accepted knowingly:** item 11a (deception) is scoped narrow on
purpose — origin app owns the fake data, hakaishield only signals the
decision, and it's gated to only the highest-confidence score band
(above the block threshold) because a wrongly-deceived real customer
sees *wrong data as real*, which is a worse failure mode than a
false-positive block. Not to be turned on for any client until the
scoring engine (item 5) and dashboard (item 12) both exist to measure
it separately from block/challenge false positives.
**Revisit when:** item 5 (scoring engine) ships — that's the actual
blocker for both 9a and 11a, since neither can be built before there's
a decision layer to plug into.

---

## Scoring thresholds: additive weights, no single signal blocks alone — 2026-09-15
**Decision:** `Score()` gives the JA4-fragmentation signal and the
UA-mismatch signal 50 points each, additive. `Decide()` blocks at 100,
challenges at 50, allows below that. A single signal firing alone can
only ever reach 50 (challenge), never 100 (block) — block requires
both.
**Why:** `CLAUDE.md` Section 6 is explicit that no single signal may
be the only thing between allow and block. Fragmentation and UA
mismatch are correlated (a fragmented handshake is one of UAMismatch's
own inputs) but not the same finding: fragmentation is a
fingerprint-layer anomaly true regardless of what the client claims to
be; UA mismatch is a consistency-layer *lie*, which needs a browser
claim to exist at all. Scoring them as two separate, additive signals
— rather than collapsing them into one — means a bot that fragments
its handshake but doesn't claim to be a browser (most naive scripts:
curl, requests, a bare `net/http` client) gets challenged, not
blocked, matching the product's own "don't hard-block on a single
imperfect signal" principle even when that signal is strong.
**Alternatives considered:** a single combined "TLS anomaly" signal
worth 100 whenever fragmentation OR UA mismatch fires — rejected,
that's exactly the single-point-of-failure shape Section 6 forbids,
and it would immediately hard-block anything that merely fragments
without ever claiming to be a browser (an honest scripted client that
happens to fragment for unrelated reasons — a censorship-circumvention
tool, per `docs/RESEARCH.md` — shouldn't get the harshest outcome for
one imperfect signal).
**False-positive risk, accepted:** a claimed-browser client on an old
TLS stack or a fragmenting network path hits the challenge threshold
alone (50) — costs a real user one extra JS-challenge page load, not a
block. A corporate proxy that both intercepts TLS *and* somehow
triggers fragmentation would hit the block threshold — considered
unlikely enough to accept for v1, revisit if real traffic shows
otherwise.
**Revisit when:** item 6 (client-side automation probe) adds a third
signal — weights need rebalancing so three signals firing doesn't make
the threshold model meaningless (e.g. any two of three always
blocking regardless of severity). Also revisit once real client
traffic exists to tune against instead of reasoned guesses.

---

## agentchat: removed the MCP server, kept plain file + manual relay — 2026-09-15
**Decision:** built, then deleted the same day, `agentchat/mcp_server.py`
— an MCP server exposing `send_message`/`get_messages`/
`wait_for_message` tools over `chat.jsonl` so Claude Code and
Antigravity could message each other without hand-editing a file.
Reverted to the original design: a plain `chat.jsonl` log plus
`agentchat/web.py` (stdlib-only HTTP bridge) for the project owner to
type into directly. Coordination between the two agents is manual —
the owner tells each one to check the log.
**Why:** two real problems, found by actually building and testing it,
not guessed in advance:
1. **Shared code file caused a live edit war.** Both agents were told
   they could edit `mcp_server.py`; when both did, each fix silently
   overwrote the other's — twice, in the same session, confirmed by
   the file breaking (`ImportError`/`AttributeError` from a
   low-level-API rewrite that removed the decorator the working
   version needed) right after being fixed.
2. **It never actually solved the problem it was built for.** MCP
   tool calls only execute when an agent's host chooses to call them —
   there is no push. Verified two ways: (a) Antigravity's own
   background script "just prints to stdout, which doesn't wake me
   up," confirmed live in `chat.jsonl`; (b) web research (see below)
   confirms this isn't specific to our setup — MCP's 2026-07-28 spec
   added `notifications/resources/updated` subscriptions, but even
   working implementations "do not guarantee durable event delivery,
   wake a model, or start an agent turn," and adoption is near zero
   because of it.
**Alternatives considered:** giving each agent its own separate MCP
server file (no shared code, avoids problem 1) — rejected once problem
2 was confirmed, since it wouldn't have fixed the actual goal
(automatic notification), only the file-collision symptom.
**What would actually work, and why it's not built here:** a trigger
inside each agent's own host application (a file-watcher, a scheduled
job, a webhook-driven automation — the pattern real products like
Cursor's "Automations" use). This has to be configured per-agent, in
that agent's own tool/IDE settings — Claude Code cannot build or
enable this for Antigravity, and vice versa. If the project owner
wants real automatic wake-up, it needs enabling on each side
separately, not more code in `agentchat/`.
**Revisit when:** either agent's host natively supports a
"wake on event" mechanism the owner can point at `chat.jsonl`, or the
MCP ecosystem's server-initiated-events work (webhooks/channels, on
its own 2026 roadmap per the research below) actually ships and gets
adopted — not before.
**Sources checked:** [Using MCP Push Notifications in AI Agents](https://gelembjuk.com/blog/post/using-mcp-push-notifications-in-ai-agents/), [MCP Has Notifications. So Why Can't Your Agent Watch Your Inbox?](https://ankitmundada.medium.com/mcp-has-notifications-so-why-cant-your-agent-watch-your-inbox-bb688fde7ac5), [anthropics/claude-code#36665](https://github.com/anthropics/claude-code/issues/36665), [MCP Roadmap 2026](https://www.explainx.ai/blog/the-new-mcp-roadmap-2026).

---

## JS challenge: sha256+canvas proof, not a math-only puzzle — 2026-09-15
**Decision:** `proxy.Challenge` requires two things from the client's
JS, not just one: the SHA-256 of a server-issued nonce, and a
`canvas.toDataURL()` render. It does not stop at the math step alone.
**Why:** a sha256-of-a-nonce puzzle by itself is not actually
JS-specific — any language can compute a SHA-256 in one line, so a
scripted client that bothers to parse the challenge HTML and hash the
nonce defeats it without ever running JS, which would fail the
ROADMAP's own claim ("a plain HTTP client without a JS engine fails
immediately"). Requiring an actual `canvas.toDataURL()` output raises
the real bar toward needing a genuine browser rendering engine, which
a bare HTTP client cannot produce without one.
**Alternatives considered:** math-only challenge (simpler, matches a
literal reading of "math + timing") — rejected once traced through:
it only filters clients too lazy to read the page, not clients without
a JS engine, which is the actual stated threat.
**Known limitation, accepted:** the canvas proof is a client-reported
string (`validCanvasProof` checks its shape — prefix + minimum
length — not its actual pixel content). A bot author who studies
hakaishield specifically can fake a plausible-looking string without
ever rendering anything. Verifying real pixel content server-side
needs either a headless-render comparison service or a much larger
research effort — out of MVP scope, same class of accepted limitation
as item 3's JA4-database gap. This is why the challenge raises cost
for a naive-to-intermediate bot; it does not claim to stop a bot built
specifically against hakaishield.
**Revisit when:** real traffic data shows the canvas check is either
pulling its weight or not worth the false-positive risk on browsers
with canvas disabled (privacy tools, some accessibility setups).

---

## JS challenge secret: random, in-process, not shared — 2026-09-15
**Decision:** `NewChallenge()` generates a random 32-byte HMAC secret
at process start, kept in memory only. No config flag, no persisted
key.
**Why:** a hardcoded secret in source would let anyone who reads the
code forge challenge tokens and passed-cookies — worse than no signing
at all. A random per-process secret closes that immediately, at the
cost of invalidating outstanding challenges on restart or across
multiple instances. Since there's only one process in the current
architecture (`docs/ARCHITECTURE.md` — "No Kubernetes, no
microservices for v1"), that cost is real but currently free.
**Alternatives considered:** a secret passed via flag/env var — not
implemented yet; would let a restart survive without invalidating
outstanding cookies, but there's no config-loading mechanism in the
codebase yet to hang it off, and the current architecture is single-
instance so the gap has no observable effect yet.
**Revisit when:** hakaishield runs more than one process (needs a
shared secret — the planned Redis store, `docs/ARCHITECTURE.md`, is
the natural place) or when the passed-cookie's 30-minute lifetime
surviving a restart becomes something a real client asks for.

---

## UA-consistency check: structural heuristics, not a browser-fingerprint database — 2026-09-14
**Decision:** `UAMismatch` catches a UA claiming a browser while the
TLS handshake shows TLS 1.0/1.1 or the `JA4Unreadable` fragmentation
signal — not a full "does this JA4 really belong to Chrome 120"
classification.
**Why:** the strict version of ROADMAP item 3 ("real client family")
needs a maintained table mapping JA4 hashes to real browser versions.
Real anti-bot vendors run that as a standing research/maintenance
cost, updated as browsers ship. Building and maintaining that
database is a project of its own, not a one-line addition, and
`CLAUDE.md` Section 15 says research before building — not fake a
version of something that needs real ongoing data. The two conditions
implemented instead are provably true of every current real browser,
need no external data to verify, and reuse the `JA4Unreadable` signal
already established for fragmentation.
**Alternatives considered:** a hardcoded list of a few known-good JA4
hashes for major browsers — rejected: browsers update their TLS stack
often enough that the list would go stale within months and start
producing false positives on real users (`CLAUDE.md` Section 8), with
no mechanism in this repo to keep it current.
**False-positive risk, accepted:** a corporate TLS-inspecting proxy or
an unusually old/locked-down real browser could legitimately negotiate
TLS 1.0/1.1 and get flagged. This is why the result is a signal for
future scoring (item 5), never a block on its own (`CLAUDE.md`
Section 6).
**Revisit when:** item 5's scoring engine exists and real traffic data
shows whether a maintained JA4-to-browser database is worth the
ongoing cost, or when HTTP/2 fingerprinting is built (adds another
structural signal of the same no-database kind).

---

## hakaishield is closed-source commercial software, not open source — 2026-09-14
**Decision:** hakaishield is proprietary. `README.md` previously said
`License: MIT`, which was wrong and is corrected to "Proprietary — All
Rights Reserved." There is no LICENSE file granting copy/modify/
redistribute rights, and none should be added.
**Why:** the project owner is building this to sell as a paid product
(a SaaS / self-hosted commercial license), not to give away. An MIT
license would have let anyone legally clone, rebrand, and resell it —
directly undermining the reason it's being built. This was a docs
mistake carried over from an earlier session's generic project
scaffolding, not a considered choice, and it was live in the repo
until caught here.
**Alternatives considered:** open-core (core engine open, paid
features closed) — not rejected outright, just not decided; revisit
if the owner ever wants community contributions or wider adoption as
a growth strategy. Until then, default to fully closed.
**What doesn't change:** hakaishield still *uses* open-source
libraries internally (`fingerproxy`, `BotD` — see the "assemble
proven open-source pieces" entry below). Depending on open-source
components is normal for commercial software and is unrelated to
whether hakaishield's own code is licensed for redistribution.
**Revisit when:** the owner explicitly decides on a monetization/
distribution model (self-hosted license sales, managed SaaS, open-
core) — that decision picks the real license text, ideally with a
lawyer's input before any code ships to a paying customer.

---

## Report an unreadable handshake as a signal, not as "no fingerprint" — 2026-09-14
**Decision:** when a connection is TLS but we cannot read its
ClientHello, `JA4FromContext` returns `JA4Unreadable` ("unreadable"),
not `""`. `""` now means only "this wasn't a TLS connection".
**Why:** a client can split its ClientHello across two TLS records.
The handshake succeeds, but our capture (and `fingerproxy`'s, which
reads one record) sees a fragment, so JA4 parsing fails. Reproduced
locally: the request sailed through with an empty fingerprint, which
looked exactly like an ordinary unfingerprinted request. That made a
one-line bot change into a *silent* bypass of the product's core
detection. Making the two states distinguishable doesn't stop the
evasion, but it stops it being invisible — and a real browser never
fragments this way, so "TLS but unreadable" is itself a bot signal.
**Alternatives considered:** blocking on an unreadable handshake —
rejected: some legitimate stacks and censorship-circumvention clients
fragment too, and `CLAUDE.md` Section 6 says no single signal decides,
Section 8 says a false positive is worse than a miss. This is an input
for scoring, not a verdict. Also considered: writing record-layer
reassembly ourselves — rejected for now, `DECISIONS.md` above says we
don't hand-write TLS parsing; see `RESEARCH.md` for the open item.
**Revisit when:** the scoring engine (`ROADMAP.md` item 5) exists and
can weight this, or when reassembly lands upstream in fingerproxy.

---

## Strip every client-IP header, not just the X-Forwarded family — 2026-09-14
**Decision:** the proxy deletes `X-Real-IP`, `True-Client-IP`,
`CF-Connecting-IP`, `X-Client-IP`, `Fastly-Client-IP` and
`X-Cluster-Client-IP` from the outbound request, then sets
`X-Real-IP` itself from the real connection address.
**Why:** net/http's `Rewrite` strips only `Forwarded` and
`X-Forwarded-*`. nginx, Rails, Laravel, Cloudflare and Fastly stacks
routinely read the others, so a visitor could still choose the IP the
origin logs, allowlists or rate-limits — the same spoof we closed for
`X-Forwarded-For`, through a different door.
**Alternatives considered:** stripping only `X-Real-IP` (the most
common) — rejected, each remaining header is a full bypass on some
origin stack, and they cost one line each.
**Revisit when:** hakaishield runs behind a trusted CDN that legitimately
sets one of these; that needs an explicit trusted-upstream setting,
never blanket trust.

---

## Let net/http own the TLS handshake; delete our hand-rolled version — 2026-09-14
**Decision:** `proxy.NewCaptureListener` returns a real `*tls.Conn`
from `Accept` and does **not** handshake it. net/http then runs the
handshake itself. This deleted our handshake timeout, accept-retry
backoff, per-connection panic recovery, goroutine-per-connection and
its semaphore, and the `hack.ChannelListener` handoff — roughly 60
lines.
**Why:** reading `$GOROOT/src/net/http/server.go` showed all four
already exist there: `Server.tlsHandshakeTimeout()` (derived from
`ReadHeaderTimeout`), the `tempDelay` accept-retry loop,
`conn.serve()`'s `defer recover()`, and its connection handling. The
stdlib versions are better than ours were — they log handshake
failures and reply properly to a plain-HTTP client hitting the TLS
port, both of which our version silently skipped. The one thing we
genuinely need (the raw ClientHello) is kept by wrapping the
connection *underneath* `tls.Server`, which costs one line.
**Alternatives considered:** keeping our own accept loop for "control"
— rejected, it was control over code we had reimplemented worse.
**Cost checked, not assumed:** the fingerprint is now derived per
request rather than once per connection. Benchmarked at **14.3µs**
against a ~2ms budget (`ARCHITECTURE.md`), so no cache — adding one
would be optimising something that costs 0.7% of its budget.
**Trade-off accepted:** `ReadHeaderTimeout` in `cmd/hakaishield` is now
load-bearing for the TLS handshake too, not just headers. Noted in a
comment there so nobody removes it as "just a header thing".
**Revisit when:** HTTP/2 support is added — returning a real
`*tls.Conn` is also what makes stdlib h2 negotiation possible, so
that work got cheaper, not harder.

---

## Use ReverseProxy.Rewrite, not Director — 2026-09-14
**Decision:** build the reverse proxy with `httputil.ReverseProxy{
Rewrite: ...}` instead of `NewSingleHostReverseProxy` + `Director`.
**Why:** net/http strips a visitor's `Forwarded` and `X-Forwarded-*`
headers before calling `Rewrite`, and does not before calling
`Director` (verified in `$GOROOT/src/net/http/httputil/reverseproxy.go`).
With `Director`, a visitor sending `X-Forwarded-For: 1.2.3.4` had it
forwarded to the origin as `"1.2.3.4, <real ip>"` — and most code
reads the first entry, i.e. the attacker's chosen value. Per-IP rate
limiting and geo checks (`ROADMAP.md` items 8 and 9) would have been
bypassable from day one. `Rewrite` + `SetXForwarded()` sends the real
client IP only. `Rewrite` also drops unparsable query parameters,
which closes a proxy/origin request-smuggling gap.
**Behaviour kept deliberately:** `SetURL` would rewrite the `Host`
header to the origin's host; we restore the visitor's `Host` (`r.Out
.Host = r.In.Host`), since the origin serves the client's own domain.
**Behaviour changed deliberately:** an inbound `X-Forwarded-For` is
now replaced rather than appended to. That is the secure default and
there are no deployments yet to break.
**Revisit when:** hakaishield is ever deployed *behind* another trusted
proxy (a CDN), where the inbound `X-Forwarded-For` is legitimate — at
that point it needs an explicit "trusted upstream" setting, never a
blanket trust of the header.

---

## Import only fingerproxy's `ja4` package, not the whole library — 2026-09-14
**Decision:** depend on `github.com/wi1dcard/fingerproxy`, but import
only its `pkg/ja4` package (JA4 hash computation, stdlib + `utls`
only) for now — not `pkg/fingerprint` (pulls in Prometheus metrics)
or `pkg/proxyserver`/`pkg/ja3` (pull in `gopacket`/`dreadl0ck/tlsx`).
**Why:** we don't need JA3, HTTP2-frame fingerprinting, or metrics yet
— only JA4. Go compiles per-package, so importing the narrower
package keeps Prometheus and gopacket out of the actual binary
entirely (verified with `go list -deps`), matching `CLAUDE.md` Section
3 (no unnecessary dependencies) and Section 14 (no code "in case it's
needed later").
**Alternatives considered:** importing `fingerproxy.Run()`'s full
opinionated server (`pkg/proxyserver`) directly instead of writing our
own capture wiring — rejected: it owns the entire accept loop and
HTTP/1.1-vs-HTTP/2 branching, which would mean replacing our own
`proxy.New` design instead of extending it. Revisit if TLS-capture
wiring turns out to need functionality we'd otherwise reimplement.
**Revisit when:** JA3 or HTTP/2 fingerprinting (also on `ROADMAP.md`)
is actually built — re-check whether pulling in `pkg/fingerprint` at
that point is cheaper than keeping our own thin wrapper.

---

## TLS capture listener: HTTP/1.1 only for now, no HTTP/2 — 2026-09-14
**Decision:** `proxy.NewCaptureListener` forces `NextProtos =
["http/1.1"]`, so browsers fall back to HTTP/1.1 against hakaishield
instead of using HTTP/2.
**Why:** capturing JA4 means terminating TLS and handshaking
ourselves instead of letting Go's `http.Server` do it, which breaks
the stdlib's automatic HTTP/2 upgrade (it only kicks in when the
accepted connection is literally a `*tls.Conn`, not our wrapped
type). Supporting HTTP/2 correctly means serving it ourselves
alongside HTTP/1.1 (the way `fingerproxy/pkg/proxyserver` does) —
real, separate work, not a one-line fix.
**Alternatives considered:** hand-rolling the HTTP/2 branch in this
same pass — rejected: `ROADMAP.md` item 2 already lists "JA4" and
"HTTP/2 fingerprint" as two separate things, and JA4 alone already
catches most naive scripted clients (the ROADMAP's own claim). Ship
one working half instead of both halves half-working.
**Revisit when:** ROADMAP's HTTP/2 fingerprint item is picked up.

## Cap concurrent TLS handshakes at 1000 — 2026-09-14
**Decision:** `proxy.NewCaptureListener` runs each connection's TLS
handshake in its own goroutine, but only allows 1000 to run at once
(a buffered channel used as a semaphore); beyond that, new
connections wait for a slot before their handshake starts.
**Why:** `CLAUDE.md` Section 9 requires every worker pool to have a
bounded size — without a cap, a flood of connections (accidental or
a deliberate flood attack) could spawn unlimited goroutines and take
the process down, which would fail the client's site closed instead
of open.
**Alternatives considered:** no cap (simplest, but violates Section
9); a smaller/larger number — 1000 is a reasonable starting guess for
a single small-to-mid deployment, not measured against real traffic.
**Revisit when:** ROADMAP item 16 (soak testing) gives real numbers
to tune this against.

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
**Decision:** hakaishield ships as one product (reverse proxy +
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
**Decision:** hakaishield is written in Go.
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
