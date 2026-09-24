# Session handoff — 2026-09-24

## Current status

This session prepared HakaiShield for a **managed, single-client pilot**. It is not deployed or ready to receive client traffic yet. There is no purchased domain, server, production API origin, TLS certificate, or labeled client traffic in place. Do not claim a 70–80% bot detection rate: effectiveness has not been measured against a representative labeled dataset.

Work is on branch `release/client-pilot-hardening`. Before this report, the latest pushed commit was `23f7d286` (`docs: record pilot hardening progress`). The session changes are committed and pushed; check `git status` and the remote before continuing.

## What changed in this session

### Backend and pilot access

- Disabled self-service `POST /api/v1/domains` for the pilot. Authenticated requests get HTTP 503 with the managed-setup message; unauthenticated requests still get HTTP 401. This avoids accepting domain claims before ownership, TLS, and origin checks exist.
- Added focused coverage in `backend/internal/api/domains_pilot_test.go` for those responses.
- Added `HAKAISHIELD_DASHBOARD_ORIGIN` so production CORS can allow the configured dashboard origin rather than `*`. The local/test default remains permissive when that variable is unset.
- Made Compose require `DATABASE_URL`, `SUPABASE_URL`, and `HAKAISHIELD_DASHBOARD_ORIGIN`, along with the existing stable pilot secrets and origin/domain settings.
- Corrected setup guidance so DNS points to the server before Certbot's HTTP-01 challenge. Renewal hooks and Compose commands explicitly load `deploy/.env`.
- Documented manual tenant binding and the single-client pilot setup.

### Dashboard honesty and stability

- Reworked public pages and pilot flows to accurately describe a managed pilot: access is requested, then an operator verifies domain/DNS/TLS/origin, runs shadow mode, and approves enforcement.
- Removed fake signup/trial, CNAME onboarding, pricing caps, ROI controls, rule controls, and unsupported SIEM/chart claims. Replaced fabricated legal and commercial promises with explicit review/pending language.
- Removed unused `dashboard/src/data/mockData.ts`, replaced the irrelevant Snake Game README, simplified `index.html`, removed external FontAwesome loading, and removed wildcard `postMessage` runtime-error reporting.
- Added `domainState()` behavior that only presents a domain as protected when its status is exactly `active`; the domain switcher lists active domains only. Cleared stale overview statistics on domain changes and removed placeholder time-series data.
- Production dashboard builds now require `VITE_API_BASE_URL`; only development uses the localhost fallback. The contact form uses `VITE_PILOT_CONTACT_EMAIL` and does not pretend to send when it is unset.
- Added a Node test for domain status, `npm test`, and a dashboard CI job (Node 24, `npm ci`, typecheck, test, build).

### Packaging, tests, and docs

- Stabilized `TestGuardVelocityLimitsPassedSession`: it now sends requests until the actual rate limit is reached rather than assuming a one-second window. Full Go tests and `go vet` passed after the change.
- Built the production Docker Compose image successfully and ran its CLI help (`docker run --rm deploy-hakaishield:latest -h`). This verifies packaging, not a live HTTPS deployment.
- Updated `dashboard/README.md`, `deploy/README.md`, `docs/CLIENT_PILOT_RELEASE.md`, `docs/ARCHITECTURE.md`, `docs/DECISIONS.md`, `docs/DEPLOYMENT.md`, `docs/ROADMAP.md`, and `docs/PROGRESS.md` to match the pilot scope and current setup.

## Verification completed

- `go test ./... -count=1` — passed.
- `go vet ./...` — passed.
- Dashboard `npm run typecheck` — passed.
- Dashboard `npm test` — passed (one domain-status test).
- Dashboard `npm run build` — passed; output bundle was about 439 KB JS / 122.75 KB gzip.
- Compose configuration rendered successfully with the required variables supplied.
- Local browser smoke check covered `/landing`, `/contact`, `/pricing`, `/sign-up`, `/terms`, and `/privacy`; no page errors were observed and the screenshot was reviewed.
- Docker production image built and CLI help ran successfully.

### Verification limits

- No production HTTPS request-path smoke test was completed. A temporary certificate/container harness attempt was blocked by the environment's automatic safety review.
- No real server, domain, client origin, TLS renewal, client browser, load test, failover test, or real traffic test was available.
- The new dashboard GitHub Actions job had not yet been observed running on GitHub during this session.
- On Windows, `npm ci` first hit an `EPERM` while cleaning a native dependency; `npm install --no-audit --no-fund` restored dependencies with cleanup/deprecation warnings, then typecheck/test/build passed. A clean Ubuntu CI run remains useful confirmation.

## Required before any client traffic

1. **Resolve legal and support details.** Provide the legal entity and jurisdiction, a working support/legal email, agreed pilot terms, SLA/support hours, and evidence/log retention period. The Terms and Privacy pages intentionally state that these require review; do not present them as approved legal documents.
2. **Obtain the host and domain.** Point the domain's DNS A record to the server before running the HTTP-01 certificate flow. Keep Cloudflare proxying disabled (DNS-only) if terminating TLS directly on this host.
3. **Prepare production secrets and configuration.** Fill `deploy/.env` on the server; never commit or paste secret values. Current required settings include `HAKAISHIELD_DOMAIN`, `HAKAISHIELD_ORIGIN`, `HAKAISHIELD_MODE=shadow`, a random stable `CHALLENGE_SECRET` (at least 32 bytes), a random `EVIDENCE_TOKEN`, `DATABASE_URL`, `SUPABASE_URL`, and the exact `HAKAISHIELD_DASHBOARD_ORIGIN`.
4. **Issue TLS and start the service.** Follow `deploy/README.md` and `docs/CLIENT_PILOT_RELEASE.md`. Run the setup script after DNS is correct, use the documented `--env-file deploy/.env` Compose commands, then check health and certificate renewal.
5. **Build and configure the dashboard.** Set `VITE_SUPABASE_URL`, `VITE_SUPABASE_ANON_KEY` (public anon key only), `VITE_API_BASE_URL=https://<pilot-host>/api/v1`, and `VITE_PILOT_CONTACT_EMAIL`. Configure Supabase's redirect allowlist for the dashboard URL.
6. **Bind the pilot tenant manually.** Create the client's Supabase Auth user and follow the exact SQL in `docs/CLIENT_PILOT_RELEASE.md`, checking for an existing tenant/domain first. Domain POST is intentionally disabled during this pilot; the operator owns onboarding.
7. **Run shadow validation with the real client.** Verify DNS, TLS, Host/forwarded-IP handling, health, origin reachability, and normal customer journeys. Compare decisions against labeled human-reviewed requests and tune false positives before enforcement. Record sample size, recall, and false-positive rate; make no detection-rate claim until measured.
8. **Enable enforcement only after sign-off.** Confirm rollback and support contacts with the client, then move from shadow mode deliberately. Pilot scope is one domain and one node.

## Product limitations to keep visible

- There is no empirical detection benchmark or production traffic evidence. A 70–80% catch rate is currently an unverified target, not a result.
- Domain ownership verification and automated multi-domain ACME onboarding are not implemented; onboarding is operator-managed.
- Billing, usage metering, self-service plan limits, and production signup are not implemented. Do not sell or imply them.
- Legal terms, privacy wording, SLA, and retention commitments need owner/legal approval.
- Some evidence/statistics are in-memory and not durable. Redis degradation/fail-open and node-local replay behavior need explicit operational review; multi-node failover is not established.
- A real client production test, representative load test, restart/recovery test, and TLS renewal exercise remain outstanding.

## Existing workspace changes to preserve

At the time of the session review, the following unrelated local changes were present and intentionally left untouched. Do not reset, delete, or stage them as part of this handoff without inspecting ownership and intent:

```text
 D docs/PHASE1_POLICY.md
 D docs/PHASE1_PRODUCTION_REVIEW.md
 D docs/PHASE2_STATUS.txt
 M graphify-out/.graphify_labels.json
 M graphify-out/.graphify_labels.json.sig
 M graphify-out/GRAPH_REPORT.md
 M graphify-out/graph.html
 M graphify-out/graph.json
 M graphify-out/manifest.json
 M patchright_test.py
?? bot_shield_test.py
?? graphify-out/2026-09-23/
?? graphify-out/2026-09-24/
```

Re-check `git status --short` before making changes: this list is a snapshot and may have changed.

## Suggested next-agent entry point

1. Read `CLAUDE.md`, `docs/AGENT.md`, this handoff, `docs/PROGRESS.md`, `docs/CLIENT_PILOT_RELEASE.md`, `docs/DECISIONS.md`, and `docs/ROADMAP.md`.
2. Run `git status --short` and inspect the branch/remote before editing. Preserve the pre-existing local changes listed above.
3. Continue the managed pilot path. First unblock on domain/server and owner-provided legal/support values; then configure, deploy, and collect labeled shadow traffic. Don't merge the release branch or claim launch readiness without actual deployment evidence and user approval.
4. Follow `CLAUDE.md` for Go commands (run them from `backend`) and branch/commit practices.

