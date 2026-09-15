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
