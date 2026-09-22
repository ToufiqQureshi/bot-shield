# hakaishield — Signal & Detection Coverage Map

**Purpose:** one place that lists every detection signal actually running
in the request path today, exactly how it's implemented, and — separately
— what is documented but not built yet. This is a status snapshot, not a
design doc. Source of truth is the code in `backend/pkg/signals/`,
`backend/pkg/core/`, `backend/pkg/challenge/`, `backend/pkg/deception/`;
cross-checked against `docs/ROADMAP.md`, `docs/DECISIONS.md`,
`docs/RESEARCH.md`, `docs/PROGRESS.md` as of 2026-09-22.

Keep this updated whenever a signal is added, removed, or reweighted —
see `CLAUDE.md` Section 22.

---

## 1. Scoring model (how all of this combines)

`backend/pkg/signals/score.go` — one `checks` table, additive weights,
two policy modes.

| Threshold | Outcome |
|---|---|
| score ≥ 100 | **Block** (or **Deceive**, if the tenant has deception enabled) |
| 1–99 (Balanced policy) | **Challenge** |
| 0 (Balanced policy) | **Allow**, zero latency |
| any score, Strict policy | **Challenge** (mandatory interstitial) |

Rule enforced in code: **no single signal reaches 100 alone.** The two
100-weight checks (`ja4_blocklist`, `scripting_tool`) are each a single
independent finding, not a combination — see `docs/DECISIONS.md` for why
those two are treated as conclusive on their own while everything else
stacks.

---

## 2. Signals currently BUILT and running

| Signal | File | Weight | What it actually checks | Data source | Fails open? |
|---|---|---|---|---|---|
| `fragmented_handshake` | `fingerprint.go`, `score.go` | 50 | JA4 came back `unreadable` — ClientHello was split across TLS records, a known fingerprinting-evasion trick real browsers never do | TLS ClientHello (server-observed, unspoofable) | Yes — no TLS, no fingerprint, no penalty |
| `ua_mismatch` | `useragent.go` | 50 | UA claims Chrome/Firefox/Safari/Edge, but JA4 says TLS 1.0/1.1, is `unreadable`, or matches a known scraper library's JA4 | UA (visitor-controlled) vs JA4 (server-observed) | Yes — empty JA4 or non-browser UA exempted |
| `header_anomaly` | `headers.go` | 25 | UA claims a browser but the request carries **none** of `Sec-Fetch-*` or `Sec-CH-UA*` — every real navigation sends at least one | Request headers | Yes — non-browser UA exempted; deliberately capped below the challenge bar alone (privacy tools/devtools fetches can miss one) |
| `ja4_blocklist` | `ja4db.go`, `score.go` | 100 (alone) | JA4 matches a verified scraper-library fingerprint, from a hardcoded seed (`python-requests` verified capture) plus a Redis-fed, background-synced (30s poll) hash set | TLS fingerprint (unspoofable) | N/A — direct match |
| `scripting_tool` | `useragent.go`, `score.go` | 100 (alone) | UA literally names itself: `curl`, `wget`, `python-requests`, `go-http-client`, `locust`, `postman`, `headless`, `playwright`, `patchright`, `puppeteer` | UA (visitor-controlled, spoofable — this is a weak signal despite the weight) | N/A |
| `velocity_spike` | `velocity.go` | 50 | Per-IP request rate over a 1s fixed window, split into two buckets: 20 navigations/window vs 300 asset requests/window (asset detection by file extension) | Redis `INCR`/`EXPIRE` pipeline, 50ms timeout | Yes — Redis down → no penalty, circuit-breaker gated |
| `ja4_velocity_spike` | `velocity.go` | 50 | One non-browser JA4 fingerprint exceeding 50 requests/second **across all source IPs** — defeats residential-proxy IP rotation, since the underlying TLS client stays the same | Redis, same pipeline pattern | Yes, same circuit |
| `crawl_pattern` | `pattern.go` | 50 | One IP touching >60 distinct page paths inside a 60s window (HyperLogLog cardinality estimate, ~12KB bounded memory); static assets and non-browser-claiming UAs excluded | Redis PFADD/PFCOUNT, 50ms timeout | Yes, same circuit |
| `honeypot_trap` | `honeypot.go`, `deception.go` | 50 | Fetched the invisible (`aria-hidden`, `tabindex="-1"`, `rel=nofollow`, `display:none`) trap link injected into **deceived** HTML responses. Keyed on (tenant, IP, JA4), 6h TTL, capped at 50k entries in-memory per node | Server-injected link + server-observed fetch | N/A — absence of a fetch just means no signal |
| automation-tool probe | `challenge.go` (not `score.go`) | hard fail, not scored | Inside the JS challenge page: checks `navigator.webdriver` and known Selenium/PhantomJS/Nightmare.js globals. Fires *in addition to* the SHA-256/canvas proof — fails the challenge outright (no passed cookie), doesn't add to the score | Client-side JS, self-reported | N/A — only runs for traffic already reaching the challenge |

### Allowlist / exemption logic (reduces false positives, not a score signal)

| Mechanism | File | What it does |
|---|---|---|
| `isCommonBrowserJA4` | `ja4db.go` | Redis-fed set of genuine browser JA4 prefixes; exempts them from the JA4 aggregate velocity check |
| `IsVerifiedGoodBot` | `goodbots.go` | Reverse-DNS + forward-DNS verification for Googlebot/Bingbot/Applebot/DuckDuckBot/YandexBot/Baiduspider claims (exact domain-suffix match, not plain substring, so `evilgooglebot.com` doesn't pass), 6h cache, bounded to 100k entries and 64 concurrent DNS lookups |
| crawler UA exemption | `useragent.go`, `pattern.go` | Any UA containing `bot`/`spider`/`crawl` is treated as an honest declared crawler, not scored for browser-impersonation checks |

### Response-layer mechanisms (not signals, but part of the pipeline)

| Mechanism | File | Status |
|---|---|---|
| JS challenge (SHA-256 + canvas proof, HMAC-signed nonce/cookie) | `challenge.go` | Built, triggered by score ≥ 1 |
| Deception (HTML rewrite for deceived traffic, <512KiB, 200 responses only) | `deception.go`, `core/proxy.go` | Built, only fires above block threshold, tenant opt-in |
| Shadow mode (score + record, never block) | `mode.go` | Built |
| Evidence trail (per-decision record, why it fired) | `evidence.go` | Built, 1000-entry ring buffer, 24h retention |

---

## 3. What's MISSING — documented gaps, not guesses

Pulled from `docs/ROADMAP.md` P1/P2 and the `Known gaps` notes attached to
each shipped item. Each one is either explicitly unbuilt or built
narrower than the original spec.

### Detection signals not built at all

| Missing signal | Why it matters | Roadmap ref |
|---|---|---|
| **Known-browser JA4 fingerprint database** | Today `UAMismatch` can only prove a handshake is *broken* (old TLS, fragmented) — it cannot answer "is this actually Chrome 120?" against a maintained set of real browser builds. Explicitly called out as **the commercial moat**, not just a feature — a stale in-house version decays silently toward allowing everything as Chrome ships every ~4 weeks | Item 19 (P1) |
| **HTTP/2 fingerprinting** | Capture listener only negotiates HTTP/1.1 — no H2 SETTINGS/frame-order fingerprint | Item 2 known gap |
| **ClientHello multi-record reassembly** | A client that splits its handshake across 2 TLS records handshakes fine but reports as `JA4Unreadable` rather than being properly fingerprinted — visible but not resolved | Item 2 known gap |
| **Behavioral scoring** (mouse entropy, click timing, scroll velocity) | Targets tools driving a *real* browser engine (Patchright, Scrapling) that pass every fingerprint check but move with unnaturally perfect timing — the one class of attacker the current signal set cannot catch | Item 7 (P1), named explicitly against Patchright in `docs/RESEARCH.md` 2026-09-18 |
| **Session consistency check** | No cross-check of IP geolocation vs. declared timezone/Accept-Language vs. TLS fingerprint's likely OS — would catch mismatched proxy/fingerprint automation pipelines | Item 8 (P1) |
| **Referrer-chain analysis** | `crawl_pattern` counts distinct paths but never checks whether navigation has a plausible referrer chain | Item 9 remaining scope |
| **Per-fingerprint request rate** (not just distinct-path count) | Current `ja4_velocity_spike` is a raw count cap; no rate-shape analysis | Item 9 remaining scope |
| **API-aware endpoint rules** | One global rate limit for the whole site — no per-category (login/checkout/listing) thresholds | Item 9a (P1), named as a stated DataDome differentiator in the competitor scan |

### Built, but narrower than the roadmap originally scoped

| Item | What's built | What's missing |
|---|---|---|
| Automation-tool probe | Checks `navigator.webdriver` + known automation globals **inside the challenge page only** | Doesn't run for `DecisionAllow` (score 0) traffic at all; would need site-wide JS injection, which doesn't exist. Also: **Patchright specifically strips these markers by design** — this check cannot catch it, by construction, not oversight |
| Honeypot trap | Injected into deceived HTML only | Not injected into challenge pages yet, so it only catches bots that already crossed into deception; trips don't propagate between nodes (per-node in-memory) |
| JA4 blocklist | One verified hardcoded entry + Redis-fed dynamic set | Coverage is only as good as what's been manually verified and fed into Redis — no automated discovery pipeline |
| Good-bot verification | Covers 6 named search engines | No mechanism yet for arbitrary "verified agent" policy (allow/rate-limit/deceive/block per declared AI agent, e.g. GPTBot) — Item 11b, explicitly scoped as the next Bot/Agent Trust Management move but not started |

### Config / policy gaps (not signals, but they blunt the signals that exist)

| Gap | Risk | Roadmap ref |
|---|---|---|
| **No per-client threshold configuration** | Every tenant gets the same fixed weights/thresholds (50/50/100) — not tuned against any real production traffic yet | Item 11 (P1) |
| **Dashboard mitigation rules have zero effect on live traffic** | CRUD exists in the dashboard; nothing in `pkg/core`/`pkg/signals` reads `mitigation_rules` yet | ROADMAP item 12.3 |
| **Dashboard protection settings have zero effect** | Same shape of gap — settings are stored, scoring still uses fixed in-code thresholds | ROADMAP item 12.4 |
| **Deception mode has no dedicated false-positive tracking** | A wrongly-deceived real customer sees fabricated data as if real (wrong price/stock) and may act on it before noticing — worse than a false-positive block. No separate dashboard metric for this yet | Item 11a explicit risk note |
| **Verified-agent allowlist risk if ever added carelessly** | An over-broad "allow this UA" rule must still run the fingerprint check — never skip scoring entirely, or the allowlist becomes a bypass with a config file | Item 11b explicit risk note |

### Infrastructure / operational gaps that affect how much signal survives at scale

These aren't detection gaps, but they cap how trustworthy the signals
above are once traffic gets large — relevant given the "can this do
10M requests" question already asked in this session.

| Gap | Why it matters |
|---|---|
| **Unrecognized `Host` header triggers an uncached Postgres lookup on every request** | A visitor sending many distinct bogus Host headers can drive unbounded DB load — no negative caching yet (found in the 2026-09-18 code review, not yet fixed) |
| **`--challenge-secret` silently falls back to a random per-process secret if unset** | In a real multi-node cluster this means node A's issued tokens get rejected by node B — fails confusingly instead of refusing to start |
| **No load test has been run** | Architecture (Redis-backed distributed rate state, Postgres-backed lazy tenant config, stateless challenge secret) is *designed* for 10M+ req/scale, but `docs/PROGRESS.md` itself lists "verify end-to-end under high traffic (`wrk`)" as an open item — no real throughput/p99 numbers exist yet |
| **Redis outage circuit is shared across all rate/pattern signals** | Correct fail-open behavior, but it means a Redis outage silently disables `velocity_spike`, `ja4_velocity_spike`, and `crawl_pattern` simultaneously — worth knowing this is one blast radius, not three independent ones |

---

## 4. Quick answer: "what would a sophisticated bot get past today?"

A tool that:
1. Drives a real, unmodified browser engine (so TLS/JA4 looks genuine),
2. Sends a normal-looking UA with real `Sec-Fetch-*`/`Sec-CH-UA` headers,
3. Isn't in the JA4 blocklist,
4. Stays under the velocity/crawl-pattern thresholds,
5. Never touches the honeypot link,
6. Uses a patched automation stack (Patchright-class) that strips
   `navigator.webdriver` and friends,

...passes every signal currently built. That gap is exactly what
**behavioral scoring (item 7)** and the **known-browser fingerprint
database (item 19)** are meant to close, and neither is built yet.

---

*Last verified against source: 2026-09-22. Re-check this file whenever
`backend/pkg/signals/score.go`'s `checks` table changes — that table is
the single source of truth for what's actually scored.*
