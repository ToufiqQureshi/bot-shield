<p align="center"><b>bot-shield</b></p>

# bot-shield

An affordable, self-hostable bot detection & mitigation service — a
practical alternative to Akamai Bot Manager / DataDome / PerimeterX for
companies that can't justify enterprise pricing.

> 🚧 Early development — see `docs/ROADMAP.md` for current status.

## Why this exists

Akamai, DataDome, and PerimeterX charge $1,500–$50,000+/month and target
large enterprises. Most mid-size companies (e-commerce, ticketing, job
portals, SaaS) get scraped and abused by bots too, but can't afford
those tools — so they run with weak or no protection.

bot-shield combines proven open-source building blocks (TLS/JA4
fingerprinting, behavioral scoring, JS challenges) behind one simple
service a small team can deploy and actually afford.

## What it is

bot-shield is one deployable product: a reverse proxy + dashboard a
client stands up in front of their site. No Go knowledge required to
run it. Internally the code is split into small, focused packages
(fingerprinting, scoring, challenge, etc.) for the usual reasons —
easier to test, easier to read, easier to fix — not because it's
meant to be reused elsewhere.

## What it does (target)

- Sits in front of a client's site as a reverse proxy / middleware.
- Fingerprints each request (TLS/JA4, HTTP/2, headers).
- Scores requests using multiple independent signals (no single
  bypassable check).
- Challenges or blocks traffic that scores as automated.
- Gives the site owner a dashboard: how many bots blocked, trends, top
  offending IPs/fingerprints.

See `docs/ROADMAP.md` for the build order and `CLAUDE.md` for the
engineering rules.

## Responsible use

bot-shield is a defensive security tool. It is built to protect
websites from unwanted automated traffic — it is not, and will never
include, tooling whose purpose is to help automation evade detection.

## License

MIT
