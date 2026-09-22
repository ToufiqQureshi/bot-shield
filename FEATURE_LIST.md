# HakaiShield Feature List

Generated from the current repository code surface: Go backend functions, React dashboard pages/API client, Python test harnesses, and the existing project docs.

## Summary

HakaiShield is an inline bot-protection reverse proxy with a React operator dashboard. The backend protects traffic in the request path, scores bot signals, serves challenges, records evidence, and exposes authenticated dashboard APIs. The dashboard provides auth, onboarding, domains, evidence logs, mitigation-rule management, protection-settings forms, and marketing/billing pages.

## Backend Request-Path Features

### 1. Reverse Proxy Protection

- Protects an origin server through an inline Go reverse proxy.
- Adds HakaiShield context headers such as JA4, resolved client IP, and deception decision metadata.
- Strips spoofable forwarding headers before forwarding.
- Supports public-origin validation for SaaS/customer-created origins to reduce SSRF risk.

Code backing:
- `backend/main.go`: `main`, `loadDotEnv`, `redactCredentials`
- `backend/pkg/core/proxy.go`: `NewOriginProxy`, `NewPublicOriginProxy`, `ValidatePublicOrigin`, `publicOriginTransport`, `blockedOriginIP`, `WithClientIP`, `ClientIPFromContext`, `WithDecision`, `DecisionFromContext`, `ScoreFromContext`
- `backend/pkg/core/identity.go`: `NewClientIPResolver`, `ClientIP`, `requestClientIP`, `canonicalIP`, `requestHost`, `validatedRequestHost`, `hostMatchesTLS`

### 2. TLS JA4 Fingerprinting

- Captures TLS ClientHello-derived JA4 fingerprints on HTTPS traffic.
- Stores JA4 in request context before request handling.
- Treats unreadable/fragmented handshakes as a signal.
- Parses JA4 strings for scoring and tests.

Code backing:
- `backend/pkg/core/capture.go`: `NewCaptureListener`, `Accept`, `ConnContext`, `WithJA4`, `JA4FromContext`
- `backend/pkg/signals/fingerprint.go`: `ParseJA4`

### 3. Multi-Tenant Domain Routing

- Maps incoming hostnames to tenants.
- Supports wildcard/default tenant routing.
- Loads database-backed tenants and domains.
- Negative-caches unknown hosts.
- Prevents cross-tenant host mapping.

Code backing:
- `backend/pkg/tenant/tenant.go`: `NewStore`, `Add`, `GetByHost`, `GetByID`, `fetchFromDB`, `addFromDBRow`, `negativeHostFresh`, `rememberNegativeHost`, `routesTraffic`
- `backend/pkg/db/db.go`: `Init`, `initSchema`, `GetTenant`, `GetTenantByID`, `ListDomains`, `CreateDomain`

### 4. Guard Decision Pipeline

- Handles every protected request.
- Resolves tenant, JA4, IP, and request facts.
- Allows verified good bots.
- Allows already-challenged sessions while still enforcing post-challenge velocity limits.
- Scores requests and decides `allow`, `challenge`, `block`, or `deceive`.
- Records stats and evidence for each decision.
- Supports enforce and shadow modes.

Code backing:
- `backend/pkg/core/guard.go`: `NewGuard`, `NewGuardWithClientIPResolver`, `ServeHTTP`
- `backend/pkg/config/mode.go`: `ParseMode`, `Mode.String`, `ParsePolicy`, `PolicyMode.String`

### 5. Scoring Engine

- Combines request facts into an additive risk score.
- Produces both score and fired signal names in one evaluation pass.
- Uses policy modes:
  - Balanced: score `0` allows, `1-99` challenges, `>=100` blocks.
  - Strict: all non-blocked traffic receives a challenge.
- Supports deception instead of block when enabled.

Code backing:
- `backend/pkg/signals/score.go`: `Evaluate`, `Score`, `Analyze`, `Decide`, `DecideWithPolicy`, `Decision.String`

### 6. Detection Signals

- `fragmented_handshake`: JA4 was unreadable.
- `ua_mismatch`: browser-looking UA conflicts with TLS/JA4 facts.
- `header_anomaly`: browser-looking UA lacks expected modern browser fetch/client-hint headers.
- `ja4_blocklist`: JA4 is a known scraper fingerprint.
- `scripting_tool`: UA names automation tools such as curl, wget, python requests, playwright, puppeteer, patchright, etc.
- `velocity_spike`: per-IP request burst over Redis-backed rate windows.
- `ja4_velocity_spike`: one JA4 fingerprint bursts across IPs.
- `crawl_pattern`: one IP touches too many distinct paths in a time window.
- `honeypot_trap`: caller fetched the invisible trap path.

Code backing:
- `backend/pkg/signals/useragent.go`: `claimsBrowser`, `IsScriptingTool`, `UAMismatch`
- `backend/pkg/signals/headers.go`: `HeaderAnomaly`
- `backend/pkg/signals/ja4db.go`: `StartJA4Sync`, `syncJA4FromRedis`, `AddKnownScraperJA4`, `AddCommonBrowserPrefix`, `IsKnownScraperJA4`
- `backend/pkg/signals/velocity.go`: `InitRedis`, `VelocityExceeded`, `checkVelocitySpike`, `checkJA4VelocitySpike`, `velocityBucket`, `redisRequestAllowed`
- `backend/pkg/signals/pattern.go`: `CrawlPatternSuspected`, `isStaticAsset`
- `backend/pkg/signals/honeypot.go`: `RecordHoneypotTrip`, `HoneypotTripped`, `sweepHoneypotLocked`
- `backend/pkg/signals/redis_circuit.go`: `allow`, `success`, `failure`, `reset`

### 7. Verified Good Bot Bypass

- Detects claimed search-engine bots.
- Verifies bot IP through reverse DNS and forward DNS.
- Uses exact domain-boundary matching to reject lookalike domains.
- Caches verification results and bounds concurrent DNS lookups.

Code backing:
- `backend/pkg/signals/goodbots.go`: `IsGoodBotClaim`, `IsVerifiedGoodBot`, `verifyDNS`, `hostnameMatchesDomain`, `cacheBotResult`

### 8. JavaScript Challenge

- Serves an interstitial challenge for suspicious traffic.
- Uses signed challenge tokens, host binding, redirect-path safety, proof-of-work, canvas proof, and automation probes.
- Issues signed passed cookies after successful verification.
- Supports local nonce store and Redis nonce store for replay protection.
- Rejects reused, tampered, expired, cross-host, headless, and automation-flagged attempts.

Code backing:
- `backend/pkg/challenge/challenge.go`: `NewChallenge`, `SetNonceStore`, `Serve`, `handleVerify`, `Passed`, `Handler`, `setPassedCookie`, `safeRedirectPath`, `validPoW`, `validCanvasProof`, `NewRedisNonceStore`, `Consume`

### 9. Deception Mode

- Instead of blocking high-confidence bots, can forward with a deception marker.
- Can inject a hidden honeypot link into eligible HTML responses.
- Avoids injection for non-HTML, compressed, partial, or large bodies.

Code backing:
- `backend/pkg/core/proxy.go`: `deceiveResponse`, `readBody`
- `backend/pkg/deception/deception.go`: `IsInjectableContentType`, `InjectPayload`, `lastIndexFold`, `equalFold`

### 10. Evidence Trail

- Records recent decisions with timestamp, JA4, fired signals, score, decision, and enforce/shadow state.
- Uses a bounded in-memory ring buffer.
- Exposes recent evidence to dashboard APIs.

Code backing:
- `backend/pkg/evidence/evidence.go`: `NewTrail`, `Record`, `Recent`

### 11. Runtime Stats

- Tracks total, passed, challenged, blocked, and deceived request counts.
- Uses atomic counters.
- Exposes mode and enforcing status in dashboard stats.

Code backing:
- `backend/pkg/stats/stats.go`: `Record`, `Total`, `Passed`, `Challenged`, `Blocked`, `Deceived`

### 12. Observability And Panic Recovery

- Provides internal counters endpoint guarded by token.
- Recovers panics in middleware.
- Optionally reports recovered panics to Sentry.

Code backing:
- `backend/pkg/observability/counters.go`: `Inc`, `Snapshot`, `CountersHandler`
- `backend/pkg/observability/sentry.go`: `Init`, `Middleware`

## Backend Dashboard API Features

### 13. Supabase JWT Verification

- Verifies dashboard user JWTs against Supabase JWKS.
- Supports key rotation and bounded unknown-key refresh behavior.
- Rejects expired, malformed, wrong-key, missing-subject, and `alg=none` tokens.

Code backing:
- `backend/pkg/auth/jwt.go`: `NewVerifier`, `Verify`, `key`, `refresh`, `rememberUnknownKid`, `decodeECPublicKey`
- `backend/pkg/api/middleware.go`: `RequireAuth`, `UserIDFromContext`

### 14. Domains API

- Lists the authenticated user's domains.
- Adds a domain with origin validation and origin normalization.
- Creates database tenant/domain records for dashboard-managed domains.

Routes:
- `GET /api/v1/domains`
- `POST /api/v1/domains`

Code backing:
- `backend/pkg/api/domains.go`: `DomainsHandler`, `normalizeOrigin`, `newDomainID`, `domainJSON`
- `backend/pkg/db/db.go`: `ListDomains`, `CreateDomain`

### 15. Dashboard Stats API

- Returns request counters for a tenant/domain.
- Verifies authenticated user ownership when dashboard DB ownership is configured.

Route:
- `GET /api/v1/dashboard/stats?tenant=<id>`

Code backing:
- `backend/pkg/api/handlers.go`: `DashboardStatsHandler`

### 16. Evidence APIs

- Token-gated evidence endpoint for deployment/operator access.
- JWT-gated evidence logs endpoint scoped to caller domain.
- Returns recent evidence records.

Routes:
- `GET /api/v1/dashboard/evidence`
- `GET /api/v1/dashboard/evidence-logs`

Code backing:
- `backend/pkg/api/handlers.go`: `DashboardEvidenceHandler`
- `backend/pkg/api/dashboard_extra.go`: `EvidenceLogsHandler`, `callerDomain`

### 17. Top Offenders APIs

- Aggregates recent evidence by JA4 fingerprint.
- Returns worst/highest-count fingerprints first.
- Includes block counts and total counts.

Routes:
- `GET /api/v1/dashboard/top-offenders`
- `DashboardTopOffendersHandler` also exists as a token-gated handler, but the main runtime wires the JWT-gated handler.

Code backing:
- `backend/pkg/api/dashboard_extra.go`: `TopOffendersHandler`
- `backend/pkg/api/report.go`: `DashboardTopOffendersHandler`

### 18. Evidence CSV Export Handler

- Formats evidence trail as CSV.
- Includes time, JA4, score, decision, enforced flag, and signal list.
- Implemented as a handler, but not wired in `backend/main.go` in the current runtime route list.

Code backing:
- `backend/pkg/api/report.go`: `DashboardExportHandler`

### 19. Mitigation Rules API

- Lists managed rules and custom rules.
- Creates custom rules.
- Toggles custom rules.
- Stores custom conditions/actions in the database.

Routes:
- `GET /api/v1/rules`
- `POST /api/v1/rules/custom`
- `PUT /api/v1/rules/{id}/toggle`

Code backing:
- `backend/pkg/api/rules.go`: `RulesListHandler`, `CreateRuleHandler`, `ToggleRuleHandler`, `customRuleJSON`
- `backend/pkg/rules/rules.go`: `Managed`, `NewStore`, `List`, `Create`, `SetEnabled`, `encodeConditions`, `decodeConditions`

Current limitation:
- Rules are CRUD/dashboard data today; the request-path guard/scoring code does not yet apply custom dashboard rules to live traffic.

### 20. Protection Settings API

- Loads and saves per-user protection settings.
- Validates that block threshold is greater than challenge threshold.

Route:
- `GET /api/v1/settings/protection`
- `PUT /api/v1/settings/protection`

Code backing:
- `backend/pkg/api/settings.go`: `ProtectionSettingsHandler`
- `backend/pkg/settings/settings.go`: `NewStore`, `Get`, `Upsert`

Current limitation:
- Stored settings are not yet read by `signals/score.go`; scoring thresholds are still fixed in code.

### 21. JSON Response Helpers

- Provides consistent JSON decode/encode helpers and API error/message envelopes.

Code backing:
- `backend/pkg/api/response.go`: `decodeJSON`, `writeJSON`, `writeMessage`, `writeError`

## Dashboard Frontend Features

### 22. Auth And Route Protection

- Sign in, sign up, forgot-password pages.
- Supabase auth client.
- Protected dashboard routes require a valid signed-in session.

Code backing:
- `dashboard/src/components/RequireAuth.tsx`: `RequireAuth`
- `dashboard/src/lib/supabaseClient.ts`
- `dashboard/src/pages/SignIn.tsx`: `SignIn`
- `dashboard/src/pages/SignUp.tsx`: `SignUp`
- `dashboard/src/pages/ForgotPassword.tsx`: `ForgotPassword`

### 23. Dashboard Shell

- Shared authenticated layout.
- Theme context and dark/light mode support.
- Routes for overview, evidence logs, mitigation rules, protection settings, and domains/SIEM.

Code backing:
- `dashboard/src/App.tsx`: `App`
- `dashboard/src/components/Layout.tsx`: `Layout`
- `dashboard/src/context/ThemeContext.tsx`: `ThemeProvider`, `useTheme`

### 24. Backend API Client

- Attaches Supabase bearer token to dashboard API calls.
- Wraps backend API errors in `ApiError`.
- Provides typed frontend functions for domains, rules, protection settings, stats, top offenders, and evidence logs.

Code backing:
- `dashboard/src/lib/api.ts`: `isSignedIn`, `listDomains`, `addDomain`, `listRules`, `createRule`, `toggleRule`, `getProtectionSettings`, `updateProtectionSettings`, `getStats`, `getTopOffenders`, `getEvidenceLogs`

### 25. Overview Page

- Shows dashboard stats and high-level protection posture.
- Calls authenticated stats/top-offender/evidence APIs through the API client.

Code backing:
- `dashboard/src/pages/Overview.tsx`: `Overview`

### 26. Evidence Logs Page

- Displays recent request evidence.
- Includes decision badges and score coloring helpers.

Code backing:
- `dashboard/src/pages/EvidenceLogs.tsx`: `EvidenceLogs`, `getDecisionBadge`, `getScoreColor`

### 27. Mitigation Rules Page

- Displays managed and custom mitigation rules.
- Creates new custom rules.
- Toggles existing custom rules.

Code backing:
- `dashboard/src/pages/MitigationRules.tsx`: `MitigationRules`

Current limitation:
- UI/API rule state exists, but live traffic enforcement is not connected yet.

### 28. Protection Settings Page

- Displays and edits challenge/block thresholds and related protection settings.
- Saves settings through the backend API.

Code backing:
- `dashboard/src/pages/ProtectionSettings.tsx`: `ProtectionSettings`

Current limitation:
- Settings persistence exists, but the backend scoring path still uses fixed constants.

### 29. Domains And SIEM Page

- Lists domains.
- Adds domains/origins.
- Presents domain/SIEM-oriented operator UI.

Code backing:
- `dashboard/src/pages/DomainsSiem.tsx`: `DomainsSiem`

### 30. Onboarding, Billing, Marketing, And Docs Pages

- Landing, pricing, changelog, docs, contact, about, terms, privacy.
- Onboarding, subscription, and payment views.

Code backing:
- `dashboard/src/pages/Landing.tsx`: `Landing`
- `dashboard/src/pages/Pricing.tsx`: `Pricing`
- `dashboard/src/pages/Changelog.tsx`: `Changelog`
- `dashboard/src/pages/Docs.tsx`: `Docs`
- `dashboard/src/pages/Contact.tsx`: `Contact`
- `dashboard/src/pages/About.tsx`: `About`
- `dashboard/src/pages/Terms.tsx`: `Terms`
- `dashboard/src/pages/Privacy.tsx`: `Privacy`
- `dashboard/src/pages/Onboarding.tsx`: `Onboarding`
- `dashboard/src/pages/Subscription.tsx`: `Subscription`
- `dashboard/src/pages/Payment.tsx`: `Payment`

### 31. Mock Data Utilities

- Contains generated traffic/log mock helpers.
- Current source scan did not find these wired into the active API client path.

Code backing:
- `dashboard/src/data/mockData.ts`: `generateTrafficData`, `generateLogs`

## Bot Testing And Harness Features

### 32. Local Bot/Test Apps

- Flask hotel-demo app and simple analyzer endpoints exist for local/manual testing.

Code backing:
- `bot-testing/hotel_flask_app.py`: `home`
- `bot-testing/analyze_server.py`: `index`, `report`
- Other scripts: `hit_analyzer.py`, `single_test.py`, `test.py`, `igore.py`

## Implemented But Not Fully Productized

- `DashboardExportHandler` exists but is not wired in `backend/main.go`.
- Mitigation rules CRUD exists, but request-path enforcement does not consume those rules yet.
- Protection settings CRUD exists, but live scoring thresholds are fixed in `signals/score.go`.
- Dashboard custom/domain APIs require both database URL and Supabase URL; without them, proxy protection still runs but dashboard CRUD APIs are disabled.
- Domain verification lifecycle appears partial: the backend can create domains, but a complete automated verification/activation flow is not visible in the scanned functions.
- HTTP/2 fingerprinting and deeper browser-behavior signals are documented gaps.

## Tests And Verification Coverage Present In Repo

- Backend tests cover JWT verification, auth middleware, domain normalization, settings validation, rules storage, guard behavior, challenge flow, deception injection, evidence ring buffer, stats counters, tenant isolation, velocity/crawl pattern, Redis circuit behavior, good-bot verification, JA4 parsing, and scoring decisions.
- Benchmarks exist for guard serving, evidence trail, stats, and signal parsing/scoring.
- Dashboard has TypeScript/Vite project setup and docs, but this file is a feature inventory, not a fresh build/test report.

