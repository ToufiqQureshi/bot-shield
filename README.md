<p align="center"><b>hakaishield</b></p>
<p align="center"><a href="https://hakaishield.com">hakaishield.com</a></p>

# hakaishield

**HakaiShield** is an enterprise-grade, inline bot protection proxy that decides which automated clients reach your site — and proves exactly why it made that decision in real-time. 

Whether you're fighting credential stuffing, scalpers, aggressive scrapers, or API abuse, HakaiShield sits directly in the request path and evaluates the very first request from any client using advanced JA4 TLS fingerprinting.

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

hakaishield is neither. It sits in the request path, reads the live TLS
ClientHello, and scores the **first request** from a client it has
never seen — then tells you exactly why it decided what it decided.

That matters most when bots cost you money directly rather than just
noise: scraped pricing, hoarded ticket inventory, copied listings,
usage-billed API calls.

**Enterprise:** if you can't send traffic to someone else's cloud
(regulated sector, data-residency rules), the same product runs in
your own infrastructure. That's a contract, not a signup — talk to
us.

hakaishield is closed-source, commercial software (a paid service —
see License below). Internally it uses proven open-source
*libraries* (TLS/JA4 fingerprinting, behavioral scoring, JS
challenges) instead of reinventing hard, already-solved problems.

## What it is

hakaishield is one product: a reverse proxy + dashboard that sits in
front of your site. We run it — you point DNS at us and there is
nothing to install. (Enterprise customers run the same binary
themselves.) Internally the code is split into small, focused packages
(fingerprinting, scoring, challenge, etc.) for the usual reasons —
easier to test, easier to read, easier to fix — not because it's
meant to be reused elsewhere.

## Try it locally

The hosted service is how customers use hakaishield. The commands below
run the same binary on your own machine for development.

```bash
cd backend && go build -o hakaishield .

# plain HTTP — proxies traffic, no fingerprinting (nothing to capture)
./hakaishield -addr :8080 -target http://127.0.0.1:9000

# with TLS — terminates TLS and fingerprints every connection
./hakaishield -addr :8443 -target http://127.0.0.1:9000 \
            -tls-cert cert.pem -tls-key key.pem
```

| Flag | Meaning |
|---|---|
| `-addr` | Address to listen on (default `:8080`) |
| `-target` | The origin server to protect, e.g. `http://127.0.0.1:9000` |
| `-tls-cert`, `-tls-key` | Your certificate and key. **Fingerprinting only works with these** — hakaishield has to terminate TLS to see the handshake. |
| `-evidence-token` | Bearer token for the per-request evidence endpoint. Leave it unset and that endpoint does not exist at all. |
| `-mode` | `enforce` (default) acts on scores. `shadow` scores and records everything but blocks nothing — see below. Any other value refuses to start. |
| `-db-url` | PostgreSQL URL (e.g. `postgres://user:pass@host:5432/hakaishield`). Required, along with `-jwt-secret`, for the dashboard's account/domains/rules/settings API. |
| `-jwt-secret` | HMAC secret for dashboard session JWTs. Required, along with `-db-url`, for that same API. Deliberately not auto-generated (unlike the challenge secret) — sessions signed with a random per-restart secret would all invalidate on every restart. |

| Env var | Meaning |
|---|---|
| `SENTRY_DSN` | Optional. A handler panic is always recovered and logged either way (the process never crashes); setting this also reports it to Sentry so it surfaces as an alert instead of a line in a log nobody is watching. Unset by default — no signup required to run hakaishield. |

### Shadow mode

`-mode shadow` runs the full scoring pipeline and records what it
*would* have done, while forwarding every request to your origin
untouched. Nothing your visitors do can be broken by a score while it
is on, which makes it the safe way to see what hakaishield finds in
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
| `X-HakaiShield-JA4` | The caller's JA4 fingerprint, e.g. `t13d1516h2_8daaf6152771_e5627efa2ab1` |
| `X-HakaiShield-JA4: unreadable` | TLS, but the handshake couldn't be read — unusual, and worth treating as suspicious |
| *(absent)* | Not a TLS connection, so there was nothing to fingerprint |
| `X-Real-IP`, `X-Forwarded-For` | The real caller's address |

hakaishield strips all of these from the incoming request before setting
its own, so a visitor can't forge them.

## Dashboard (frontend)

`dashboard/` is a React + Vite UI (19 pages: marketing, auth, billing,
and the operator dashboard itself — evidence logs, mitigation rules,
protection settings, domains).

**Wired to a real backend**, over `dashboard/src/lib/api.ts`:
account signup/signin (JWT), domains, mitigation rules, protection
settings, live dashboard stats, top-offender JA4s, and evidence logs.
See `docs/DECISIONS.md`'s 2026-09-21 dashboard-wiring entry for exactly
what's real vs. still a placeholder — in short: payments, email
(so no verification/reset emails), SIEM export, WAF toggles, and the
traffic-over-time chart are not built. Those sections say so in the UI
rather than showing fake data.

```bash
cd dashboard
npm install
npm run dev        # local dev server (set VITE_API_BASE_URL — see .env.example)
npm run typecheck  # tsc --noEmit
npm run build      # production build
```

The backend side needs `-db-url` and `-jwt-secret` set to serve this
API at all (see `docs/PROGRESS.md`'s dashboard-wiring entry); without
both, `hakaishield` still runs as a proxy, just without the account
API.

## Documentation

Start with `docs/AGENT.md` (why this exists), then
`docs/ARCHITECTURE.md` (our enterprise-grade pipeline architecture), then `docs/ROADMAP.md` (Upcoming Enterprise Innovations).
`CLAUDE.md` holds the enterprise stability guidelines every change must follow.

## Responsible use

hakaishield is a defensive security tool. It is built to protect
websites from unwanted automated traffic — it is not, and will never
include, tooling whose purpose is to help automation evade detection.

## License

**Proprietary — All Rights Reserved.** hakaishield is closed-source
commercial software. No license to copy, modify, distribute, or use
this code is granted except as agreed directly with the owner. It is
not an open-source project, even though it uses open-source libraries
internally (see below).
