<p align="center"><b>bot-shield</b></p>

# bot-shield

**Bot-Shield** is an enterprise-grade, inline bot protection proxy that decides which automated clients reach your site — and proves exactly why it made that decision in real-time. 

Whether you're fighting credential stuffing, scalpers, aggressive scrapers, or API abuse, Bot-Shield sits directly in the request path and evaluates the very first request from any client using advanced JA4 TLS fingerprinting.

### Key Capabilities:
- **Zero-Trust TLS Fingerprinting (JA4):** Instantly analyzes the TLS ClientHello handshake to identify headless browsers, scripts, and faked user-agents without relying on log analysis or shared blocklists.
- **Real-Time Scoring Engine:** Evaluates every incoming request and takes immediate action — Allow, Serve JS Challenge, or Block Outright.
- **Shadow Mode Testing:** Run the full scoring and fingerprinting pipeline in the background to see exactly what *would* be blocked on your production traffic, guaranteeing zero impact on real users during deployment.
- **Evidence-Based Decisions:** Transparent live stats and an evidence endpoint (`/api/v1/dashboard/evidence`) tell you exactly which signals triggered a block.
- **Enterprise Ready:** Available as a hosted CNAME solution (zero installation) or deployed within your own infrastructure to meet strict data-residency regulations.

## Why this exists

Bot protection today comes in two shapes, and neither fits a mid-size
company:

- **Enterprise** (Akamai, DataDome, HUMAN, Kasada): $1,500–$50,000+/
  month, quote-only, and your traffic runs through their cloud.
- **Free and self-hosted** (CrowdSec, Coraza, ModSecurity): genuinely
  free, but they parse **server logs** — they react to an IP *after*
  it has already misbehaved somewhere, and lean on shared blocklists.

bot-shield is neither. It sits in the request path, reads the live TLS
ClientHello, and scores the **first request** from a client it has
never seen — then tells you exactly why it decided what it decided.

That matters most when bots cost you money directly rather than just
noise: scraped pricing, hoarded ticket inventory, copied listings,
usage-billed API calls.

**Enterprise:** if you can't send traffic to someone else's cloud
(regulated sector, data-residency rules), the same product runs in
your own infrastructure. That's a contract, not a signup — talk to
us.

bot-shield is closed-source, commercial software (a paid service —
see License below). Internally it uses proven open-source
*libraries* (TLS/JA4 fingerprinting, behavioral scoring, JS
challenges) instead of reinventing hard, already-solved problems.

## What it is

bot-shield is one product: a reverse proxy + dashboard that sits in
front of your site. We run it — you point DNS at us and there is
nothing to install. (Enterprise customers run the same binary
themselves.) Internally the code is split into small, focused packages
(fingerprinting, scoring, challenge, etc.) for the usual reasons —
easier to test, easier to read, easier to fix — not because it's
meant to be reused elsewhere.

## Try it locally

The hosted service is how customers use bot-shield. The commands below
run the same binary on your own machine for development.

```bash
go build -o botshield ./cmd/botshield

# plain HTTP — proxies traffic, no fingerprinting (nothing to capture)
./botshield -addr :8080 -target http://127.0.0.1:9000

# with TLS — terminates TLS and fingerprints every connection
./botshield -addr :8443 -target http://127.0.0.1:9000 \
            -tls-cert cert.pem -tls-key key.pem
```

| Flag | Meaning |
|---|---|
| `-addr` | Address to listen on (default `:8080`) |
| `-target` | The origin server to protect, e.g. `http://127.0.0.1:9000` |
| `-tls-cert`, `-tls-key` | Your certificate and key. **Fingerprinting only works with these** — bot-shield has to terminate TLS to see the handshake. |
| `-evidence-token` | Bearer token for the per-request evidence endpoint. Leave it unset and that endpoint does not exist at all. |
| `-mode` | `enforce` (default) acts on scores. `shadow` scores and records everything but blocks nothing — see below. Any other value refuses to start. |

### Shadow mode

`-mode shadow` runs the full scoring pipeline and records what it
*would* have done, while forwarding every request to your origin
untouched. Nothing your visitors do can be broken by a score while it
is on, which makes it the safe way to see what bot-shield finds in
your real traffic before enforcing anything.

It is deliberately hard to miss that it is on: a startup log line,
`"mode":"shadow"` on every stats response, `"enforced":false` on every
evidence record, and in the dashboard a status badge plus a banner —
with the counters relabelled "Would block" / "Would challenge" /
"Would pass".

Two read-only endpoints are served alongside your traffic:

| Endpoint | What it gives you |
|---|---|
| `GET /api/v1/dashboard/stats` | Running totals: requests seen, passed, challenged, blocked, plus `mode` and `enforcing` so the counts can't be read out of context. No per-visitor data, so it needs no token. |
| `GET /api/v1/dashboard/evidence` | The last 1000 decisions (24h max), newest first: timestamp, JA4, which signals fired, score, decision, and whether it was `enforced`. Accepts `?limit=N`. **Requires `Authorization: Bearer <-evidence-token>`.** |

The evidence endpoint is off unless you set a token, and it never gets
wildcard CORS — it returns visitor fingerprints, and left open it would
also tell a bot whether its own fingerprint is being flagged. Run it
over TLS; on a plain-HTTP deployment the token travels in the clear.

Your origin then receives each request with:

| Header | Meaning |
|---|---|
| `X-BotShield-JA4` | The caller's JA4 fingerprint, e.g. `t13d1516h2_8daaf6152771_e5627efa2ab1` |
| `X-BotShield-JA4: unreadable` | TLS, but the handshake couldn't be read — unusual, and worth treating as suspicious |
| *(absent)* | Not a TLS connection, so there was nothing to fingerprint |
| `X-Real-IP`, `X-Forwarded-For` | The real caller's address |

bot-shield strips all of these from the incoming request before setting
its own, so a visitor can't forge them.

## Documentation

Start with `docs/AGENT.md` (why this exists), then
`docs/ARCHITECTURE.md` (our enterprise-grade pipeline architecture), then `docs/ROADMAP.md` (Upcoming Enterprise Innovations).
`CLAUDE.md` holds the enterprise stability guidelines every change must follow.

## Responsible use

bot-shield is a defensive security tool. It is built to protect
websites from unwanted automated traffic — it is not, and will never
include, tooling whose purpose is to help automation evade detection.

## License

**Proprietary — All Rights Reserved.** bot-shield is closed-source
commercial software. No license to copy, modify, distribute, or use
this code is granted except as agreed directly with the owner. It is
not an open-source project, even though it uses open-source libraries
internally (see below).
