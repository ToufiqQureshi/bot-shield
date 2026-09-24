# First client pilot: release gate (2026-09-24)

## Scope and honest claim

This branch prepares a **single-node, single-domain pilot**. Start in shadow
mode and show the client real evidence before enabling blocking. No production
traffic or independently labelled bot sample has been measured yet, so the
requested 70–80% catch rate cannot be claimed. A scripted ladder is a regression
tool, not a population-level detection estimate. The current backend is not a
multi-region, SLA-backed hosted service.

## Source review of all five `inspired/` repositories

The reference code was inspected for threat and operational ideas. No reference
code was copied into the product.

| Repo | Useful finding | Pilot decision | Later work |
|---|---|---|---|
| `horizon` (FCaptcha) | `HARDENING.md` and server admission/replay code stress stable 32-byte signing keys, bounded state, and Redis `noeviction`. | Require a stable challenge secret, retain local spent markers, use persistent no-eviction Redis. | Admission quotas and multi-node fail-closed replay state need load and outage tests. |
| `nexus` (Anubis) | Policy recipes under `data/common/` exempt robots, sitemap, favicon and `/.well-known/` routes and distinguish APIs from pages. | Keep verified crawlers and known-good paths out of aggressive global defaults; first client policy must be reviewed for their origin. | Endpoint-specific policy presets with tenant preview and accessibility tests. |
| `vertex` (bot-signal) | `src/server/analysis.ts` uses corroborating browser, client-hint, platform and TLS evidence rather than a single claim. | Add bounded Chromium UA/client-hint major mismatch as **shadow-only** evidence. | Evaluate mobile/platform consistency and browser variants on real traffic before any score weight. |
| `quantum` (Brotector) | `brotector.js` catalogs WebDriver/CDP/debugger/stack artifacts and aggressive prototype hooks. | Use the catalog as lab test cases only. Do not ship debugger traps, crash behavior, invasive hooks or a lone CDP block. | Browser automation regression suite after the first client's normal browser sample. |
| `zenith` (go-away) | Go conditions/actions and challenge pass/fail paths show why policy and challenge state need explicit outcomes. | Existing versioned policy and signed challenge stay the integration point. | More policy action coverage and session continuity after live shadow review. |

## Code gate complete in this branch

- Tenant-scoped Redis counters for navigation/asset velocity, JA4 velocity,
  and distinct-path crawl. Cross-tenant tests cover each key family.
- Public puzzle-minting route closed in the production mux; Guard can issue
  only after resolving a tenant. Replay and short-secret regressions pass.
- Stable secret, evidence token, public host and origin are required by Compose.
  Evidence token/secret travel in environment variables, not process arguments.
- Redis keeps nonce keys until TTL (`noeviction`) and persists them in an AOF
  volume. At capacity, the local nonce store rejects new solves.
- Chromium client-hint mismatch appears as `shadowSignals` in evidence and
  an aggregate counter. It has no score or enforcement weight.
- Bot ladder's crawl rung now sends enough browser-claiming distinct paths to
  exercise `crawl_pattern`.

## Deployment gate for this evening

Local verification on 2026-09-24: `go test ./... -count=1`, `go vet ./...`,
`go build ./...`, Linux/amd64 CGO-off cross-build of both binaries,
`golangci-lint run ./...` (0 issues), Python syntax parse, and Compose config
with dummy required values passed. The Docker image build could not fetch
`golang:1.25-alpine` because this workstation could not resolve
`auth.docker.io`. Windows `go test -race` could not start because the C
compiler is absent. Run the Linux CI race suite and build the image on the
deployment host before calling the release verified.
The tenant-isolation, nonce-cap, short-secret and public puzzle-route tests
were observed failing against the previous behavior and passing after the
corresponding fixes; the shadow-signal test was compile-red before its code
was added.

1. Obtain the actual domain, origin URL, and a server close to the origin.
   Point the domain to the server and issue a valid TLS certificate. Keep the
   origin restricted to the proxy where possible; otherwise direct-origin
   access bypasses all decisions.
2. Fill `deploy/.env` from the example, generate each token with
   `openssl rand -hex 32`, and keep the file out of Git. Run
   `docker compose -f deploy/docker-compose.yml config --quiet` before `up`.
3. Start in `HAKAISHIELD_MODE=shadow`. Confirm TLS/JA4 varies by client,
   correct Host routes, real browser/origin functionality, health endpoint,
   Redis health, evidence authentication, restart recovery, and cert renewal.
4. Run `bot-testing/ladder/ladder.py` only against our own pilot site. Review
   real visitor samples for false positives across mobile, privacy browsers,
   search crawlers, accessibility tools, checkout and login. Keep label
   collection off during the attack simulation.
5. Switch to enforce only after the client approves the impact and a reviewed
   shadow sample supports the policy. Keep the previous Compose mode and
   image/commit available for quick rollback. Monitor 403/challenge rates,
   origin errors, Redis errors, latency and challenge solve failures.

## Known limits before client handoff

- In-memory evidence and per-node shadow statistics do not survive restart.
  This limits audit/history and policy activation evidence.
- A Redis outage falls back to per-node challenge nonce state; replay across
  nodes during that outage is not prevented. Keep the pilot to one proxy node.
- AOF with `everysec` can lose recent writes on a host crash; fully durable
  replay guarantees need a different fail-closed design.
- The shadow client-hint candidate has no measured precision or recall. New
  Phase 3 behavior, asset fidelity and HTTP/2 intelligence remain research
  work, not pilot protection claims.
- No live origin, domain, certificate, load test or real-browser smoke result
  exists in this workspace yet. The release cannot be called live until these
  gates are executed on the chosen host.
