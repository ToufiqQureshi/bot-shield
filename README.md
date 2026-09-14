<p align="center"><b>bot-shield</b></p>

# bot-shield

An affordable, self-hostable bot detection & mitigation service — a
practical alternative to Akamai Bot Manager / DataDome / PerimeterX for
companies that can't justify enterprise pricing.

> 🚧 **Early development.** What works today: a TLS-terminating reverse
> proxy that fingerprints every connection (JA4) and passes that to
> your origin. It **labels** traffic; it does not block anything yet.
> Scoring, challenges and the dashboard are still to come — see
> `docs/ROADMAP.md`.

## Why this exists

Akamai, DataDome, and PerimeterX charge $1,500–$50,000+/month and target
large enterprises. Most mid-size companies (e-commerce, ticketing, job
portals, SaaS) get scraped and abused by bots too, but can't afford
those tools — so they run with weak or no protection.

bot-shield is closed-source, commercial software (a paid, self-hosted
product — see License below). Internally it uses proven open-source
*libraries* (TLS/JA4 fingerprinting, behavioral scoring, JS
challenges) so a small team can build and price it affordably,
instead of reinventing hard, already-solved problems.

## What it is

bot-shield is one deployable product: a reverse proxy + dashboard a
client stands up in front of their site. No Go knowledge required to
run it. Internally the code is split into small, focused packages
(fingerprinting, scoring, challenge, etc.) for the usual reasons —
easier to test, easier to read, easier to fix — not because it's
meant to be reused elsewhere.

## Try it

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

Your origin then receives each request with:

| Header | Meaning |
|---|---|
| `X-BotShield-JA4` | The caller's JA4 fingerprint, e.g. `t13d1516h2_8daaf6152771_e5627efa2ab1` |
| `X-BotShield-JA4: unreadable` | TLS, but the handshake couldn't be read — unusual, and worth treating as suspicious |
| *(absent)* | Not a TLS connection, so there was nothing to fingerprint |
| `X-Real-IP`, `X-Forwarded-For` | The real caller's address |

bot-shield strips all of these from the incoming request before setting
its own, so a visitor can't forge them.

Note: bot-shield currently serves **HTTP/1.1 only** — HTTP/2
fingerprinting isn't built yet, so h2 isn't offered.

## Documentation

Start with `docs/AGENT.md` (why this exists), then
`docs/ARCHITECTURE.md` (what it's made of, and what's actually built
versus planned), then `docs/ROADMAP.md` (what's next).
`CLAUDE.md` holds the engineering rules every change must follow.

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
