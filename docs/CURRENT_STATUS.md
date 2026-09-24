# HakaiShield: current status and remaining work

**Snapshot:** 2026-09-24, branch `release/client-pilot-hardening` at
`fdd28614` before this documentation cleanup. Read this file first for the
handoff; `CLIENT_PILOT_RELEASE.md` is the operational release checklist,
`SIGNAL_COVERAGE.md` is the signal inventory, and `BACKEND_IMPLEMENTATION_PLAN.md`
is the longer-term implementation plan. Code and tests remain the final source
of truth.

## Honest product status

The code supports a **managed, one-domain, one-node shadow pilot**. It is not a
live customer deployment in this workspace: the client domain, server, origin,
production TLS, dashboard URL, real browser smoke test, and labelled production
traffic have not been supplied or verified here. **70–80% bot detection is a
target, not a measured result.** A local build and bot ladder cannot establish
population-level recall or false-positive rate. Do not promise production
availability or enable enforcement from this document alone.

The release branch was pushed; it has not been merged to `main` by this work.
There are unrelated pre-existing workspace changes in `graphify-out/`,
`patchright_test.py`, `bot_shield_test.py`, and `CODEX_TODO_bot_detection.txt`.
Inspect `git status` before staging and do not sweep those into a docs commit.

## What is implemented

| Area | Current behavior | Limit that matters |
|---|---|---|
| Inline proxy | Go terminates TLS, captures ClientHello/JA4, checks Host/SNI, forwards to an origin, and supports `shadow` or `enforce`. | The pilot is one node; live TLS, latency and recovery are unmeasured. |
| Scored detection | Nine scored checks cover TLS/UA/header mismatch, known bad JA4, openly named scripting tools, per-IP and per-JA4 velocity, distinct-path crawling, and a tenant honeypot. Redis keys are tenant-scoped. | A real browser with plausible headers and slow requests can evade these checks. Existing thresholds need client traffic calibration. |
| Challenge and continuous trust | Signed, host-bound challenge/cookies, bounded proof-of-work and canvas checks, nonce replay protection, adaptive difficulty, and rescoring after a passed cookie. Fresh high-confidence evidence can still block; velocity/crawl can rate-limit. | A scripted client can forge browser telemetry and PNG proof. In proxy shadow mode visitors never see this challenge. |
| Additional observations | Chromium client-hint contradictions are recorded as `shadowSignals` during normal proxy shadow traffic. Four more candidates—WebGPU f16 absence, duplicate canvas output, pointer inactivity, and legacy automation globals—are recorded only after a valid **enforced** challenge solve. | None changes score or action. The four challenge-only candidates collect **no real-visitor samples in initial proxy shadow mode**. |
| Verified bots and false-positive controls | Search crawler claims use bounded reverse/forward DNS verification. Policies have explicit owner checks, versioning, shadow preview, rollback, and an activation gate. | The policy gate uses local aggregates and needs real traffic review. The dashboard's legacy rule/settings pages do not control live policy. |
| Evidence and dashboard | Tenant-scoped evidence, stats and offenders; selected-domain queries are ownership checked. Dashboard distinguishes scored signals from yellow observed candidates and cancels stale domain-switch requests. | Evidence and several aggregates are in memory and disappear on restart; UI test coverage is still small. |
| Learned scoring | Candidate label pipeline and pure-Go trainer/model exist. Loaded model predictions are recorded alongside rule decisions. | Model is **shadow-only**. No representative, independently reviewed labels or measured model quality exist. Collected challenge/honeypot candidates are not ground truth. |
| Managed onboarding and packaging | Self-service domain creation is disabled; operator binds the verified client domain. Compose requires stable secrets, Postgres, Supabase and dashboard origin. Certbot key is copied to a restricted directory for the non-root container; the renewal hook refreshes it before restart. | Domain ownership, TLS and origin checks are manual for this pilot; automatic multi-domain onboarding and billing are not built. |

The five `inspired/` repositories were reviewed for ideas, with no code copied.
`nexus` informed policy/challenge handling; `horizon` replay and input forensics;
`vertex` corroborated browser/client-hint evidence; `quantum` automation
regression cases; `zenith` conditions, outcomes and resource checks. Risky
debugger/crash hooks and unmeasured timing thresholds were not adopted. See
`CLIENT_PILOT_RELEASE.md` for the per-repository decision table.

## Work completed on 2026-09-24

- Hardened passed-cookie handling so each later request is scored again;
  closed a challenge bypass and improved tenant-scoped Redis counters.
- Added client-hint GREASE and major/platform/mobile contradictions as
  observation-only signals; added WebGL platform mismatch checks on the
  challenge page and four bounded challenge telemetry candidates. Those four
  do **not** increase pilot detection coverage in proxy shadow mode.
- Deferred DNSBL/IP reputation for the low-cost pilot. It could flag known bad
  source IPs but introduces list quality, false-positive, latency and provider
  cost questions; revisit only if labelled traffic shows a gap it can close.
- Reworked the dashboard into an honest managed-pilot flow, removed fake
  payment/trial claims and placeholder data, and required a production API URL.
- Fixed domain switching for evidence/offenders and made unowned domain IDs
  fail closed. Exposed observed evidence separately from scored signals.
- Fixed Docker TLS-key access for the non-root image by mounting a restricted
  certificate copy rather than Certbot's root-only tree.

Relevant commits: `da9b44b3`, `0a886dcc`, `2094da41`, `3f64e5c3`,
`0f5478b7`, `0e4b3674`, `12115dae`, `5f773e98`. The short commit index is
`PROGRESS.md`; commit messages hold the detailed tests and trade-offs.

## What remains, in execution order

### 1. Before the first client request — launch gate

1. Owner supplies the actual domain, protected origin, a server near that
   origin, legal entity/jurisdiction, working support/legal email, approved
   pilot terms, retention period and support/rollback contact.
2. Point DNS to the server before Certbot HTTP-01. Keep any CDN in DNS-only
   mode if it would terminate TLS before HakaiShield; otherwise JA4 loses its
   visitor ClientHello. Restrict direct origin access where practical.
3. Fill `deploy/.env` on the server with stable generated secrets and the
   required domain/origin/Postgres/Supabase/dashboard origin values. Run
   `deploy/setup.sh`, Compose config/build/up and certificate renewal checks
   exactly as in `deploy/README.md`. Never commit secrets.
4. Build the static dashboard with its Supabase and API URLs, configure Auth
   redirects, manually bind the verified pilot tenant using the SQL in
   `CLIENT_PILOT_RELEASE.md`, and test the authenticated dashboard.
5. Smoke-test the **real** HTTPS request path: SNI/Host, JA4 variation across
   clients, origin routing, normal login/checkout/API journeys, good crawlers,
   challenge and rollback paths, health, Redis outage/recovery, reboot and
   renewal. Measure p95/p99 latency and origin errors at expected load.
6. Start in `shadow`, review real visitor evidence with the client and label a
   representative sample of humans and bots. Record sample size, bot recall,
   human false-positive rate and challenge solve/failure rates by browser and
   device. Tune policy before any scoped enforcement; get client sign-off and
   keep a tested immediate rollback path.

These are release gates, not evidence that the current code is broken. They
require the real client environment; none is verified by local unit tests.

### 2. Detection coverage after the pilot baseline

| Gap | Why it matters | Next safe step |
|---|---|---|
| **Client-side behavioural analysis** | Slow automation in a real browser can look normal in JA4, headers and request rate. | Choose an opt-in first-party collection path for normal page views; collect bounded interaction timing, scroll/navigation cadence and session continuity, without raw coordinates or keystrokes. Start as evidence only, measure across mobile, accessibility tools and privacy browsers, then review any score weight. |
| Challenge-only probes | Current four candidates have no samples during proxy shadow mode. | Run a separately reviewed challenge cohort after the client agrees; measure false positives before promotion. A solve is not proof of a human. |
| Asset/referrer fidelity and API patterns | A scraper can load pages slowly while skipping assets or enumerating API pages. | Establish legitimate per-endpoint baselines, then add bounded session-level evidence and endpoint-aware rules in shadow. |
| Maintained browser JA4 knowledge and HTTP/2 | Current JA4 blocklist is narrow and the listener lacks HTTP/2 fingerprinting. | Build a curated, versioned, expiry-aware fingerprint feed; unknown fingerprints stay neutral. Validate protocol changes against real browsers before scoring. |
| Session consistency and optional IP context | Proxy/fingerprint pipelines may claim inconsistent language, timezone or network context. | Corroborate multiple signals; keep geography/IP lists out of single-signal blocking. DNSBL stays deferred until measured need. |
| Learned model | Candidate labels are biased and can be forged; model has no validated recall/FPR. | Curate independent labels, hold out evaluation data, compare shadow predictions by tenant/browser and keep enforcement off until explicit review. |

Behavioural analysis is **not built for ordinary page traffic**. Existing
`velocity_spike`/`crawl_pattern` describe server-side request behaviour, and
the challenge's pointer-active flag is only a coarse, visitor-reported bit
after an enforced challenge. Neither equals mouse/click/scroll behavioural
scoring. Building this well needs an agreed collection surface, privacy limits,
bounded storage and real-user false-positive testing; it is not a safe
same-day hard-block switch.

### 3. Product/platform work after one-domain validation

Automated domain ownership verification and ACME, durable evidence and shadow
aggregates, dashboard editing of tenant policy, usage metering/billing, support
operations, multi-node replay/failover, load/soak testing, and reviewed legal
documents remain. These are requirements for a self-service hosted product,
not capabilities of the managed single-client pilot. The detailed backlog is
in `BACKEND_IMPLEMENTATION_PLAN.md` and `ROADMAP.md`.

## Verification actually completed

On the current release branch, local checks passed: backend
`go test ./... -count=1`, `go vet ./...`, `go build ./...`, and
`golangci-lint run ./...` (0 issues); dashboard `npm run typecheck`,
`npm test` (3 tests), and `npm run build`; Compose config with dummy values;
production Docker image build and binary `-h`; and shell syntax plus a TLS
permission/renewal fixture test that confirmed UID/GID 65532 can read the key
while an unrelated user cannot. Focused tenant and UI tests were run red before
fixes and with meaningful mutations. A previous local browser smoke covered
public pages, but **no browser smoke was rerun after the latest dashboard
change**. No production HTTPS, representative load, cert renewal on the target
host, or measured bot-catch test has passed.

## Documentation cleanup and next-agent entry point

This file replaces the dated `SESSION_HANDOFF_2026-09-24.md`. The generic
`go_expert.txt` tool list and obsolete Phase 1 review/Phase 2 status snapshots
were removed; their lasting decisions and open work are recorded here, in the
implementation plan, the topic docs, and Git history. The Phase 1 API contract
is consolidated into `BACKEND_IMPLEMENTATION_PLAN.md`.

For the next session: read `CLAUDE.md` and `AGENT.md`, then this file and
`CLIENT_PILOT_RELEASE.md`; run `git status` and check the release branch before
touching concurrent edits. Finish the real launch gate before promising a
client a catch rate. Keep behavioural/browser candidates in observation mode
until labelled traffic supports a safe enforcement decision.
