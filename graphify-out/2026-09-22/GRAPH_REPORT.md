# Graph Report - bot-shield  (2026-09-22)

## Corpus Check
- 144 files · ~149,479 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 8 file(s) not represented in the graph (top: (none) 5, .example 2, .css 1)

## Summary
- 1479 nodes · 2948 edges · 99 communities (84 shown, 15 thin omitted)
- Extraction: 92% EXTRACTED · 8% INFERRED · 0% AMBIGUOUS · INFERRED: 223 edges (avg confidence: 0.86)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `ff471581`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- App.tsx
- velocity_test.go
- RESEARCH.md — Reference Notes
- challenge_test.go
- identity.go
- go_pkg_time
- CLAUDE.md
- Store
- testing.T
- challenge.go
- What You Must Do When Invoked
- HakaiShield: Complete Pages Overview
- package.json
- HakaiShield: Fake Content Removed & Honest Positioning Added
- Trust Builders - Investor & Client Confidence Features
- main
- API Endpoints
- analyze_server.py
- patchright_sync_api
- PROGRESS.md — What Was Done, When, and Why
- vite.config.js
- HakaiShield Backend Implementation Plan
- github.com/ToufiqQureshi/hakaishield
- 13. Test Quality — Do Not Let Tests Lie
- 15. Production Safety
- 2. Decision Rules — When to Act and When to Ask
- 0. Mission
- 32. Operating Principle
- 3. Documentation Is Project Memory
- 8. Go Standards
- 11. Testing Is Part of Implementation
- 20. Standard Library First
- 21. Existing Code and Dead Code
- graphify reference: extra exports and benchmark
- graphify reference: query, path, explain
- graphify reference: add a URL and watch a folder
- graphify reference: commit hook and native CLAUDE.md integration
- graphify reference: incremental update and cluster-only
- graphify reference: GitHub clone and cross-repo merge
- graphify reference: transcribe video and audio
- .claude/CLAUDE.md
- extraction-spec.md
- score_test.go
- Changes Made to Remove AI-Generated Feel
- Codex Continuation Handoff
- HakaiShield Backend Wiring Documentation
- dependencies
- compilerOptions
- HakaiShield Dashboard - Light Mode Implementation
- honeypot_test.go
- middleware_test.go
- main
- DECISIONS.md — Why We Chose What We Chose
- hakaishield
- proxy.go
- Page-by-Page Wiring Guide
- devDependencies
- 2026-09-16 — Market scan; repositioned the product away from "affordable alternative"
- 2026-09-16 — Pivot to hosted SaaS; self-hosting becomes Enterprise
- mockData.ts
- Upcoming Enterprise Innovations
- Data Models
- 2026-09-16 — Decision evidence trail (ROADMAP item 12a, done)
- 2026-09-16 — Shadow mode (ROADMAP item 18, mode done); Antigravity ended; dashboard is ours now
- go_pkg_net_http
- 2026-09-16 — Answered "why not just build it yourself?"; named the moat
- 2026-09-17 — /code-review on my own shadow-mode work; two real bugs fixed
- 2026-09-16 — HANDOFF: PR #4 merged; next session's job is to break it
- 1. Landing Page (`/landing`)
- jwt_test.go
- Authentication System
- Deployment Checklist
- Error Handling
- scripts
- 2026-09-20 — Consolidated PR #10 + PR #11 into one hardened change
- 2. Pricing Page (`/pricing`)
- dashboard/README.md
- botshiel_frontend_src_index
- go_pkg_testing
- main.go
- go_pkg_strings
- Decision
- Inc
- go_pkg_github_com_toufiqqureshi_hakaishield_pkg_account
- go_pkg_golang_org_x_crypto_bcrypt
- go_pkg_net_mail
- go_pkg_context
- hakaishield — Production-Grade Full Software Audit
- newTestRedis
- 3. What's MISSING — documented gaps, not guesses
- Verifier
- main_test.go
- hakaishield Architecture
- AGENT.md — Mission Brief for Any Coding Agent
- RequireAuth
- 2. Critical Issues (P0)
- Prioritized Action Plan
- FAILUROFTHISPRODUCT.MD

## God Nodes (most connected - your core abstractions)
1. `PROGRESS.md — What Was Done, When, and Why` - 58 edges
2. `DECISIONS.md — Why We Chose What We Chose` - 44 edges
3. `useTheme()` - 33 edges
4. `main()` - 30 edges
5. `NewOriginProxy()` - 29 edges
6. `NewStore()` - 28 edges
7. `Verifier` - 24 edges
8. `NewChallenge()` - 22 edges
9. `Store` - 22 edges
10. `NewGuard()` - 21 edges

## Surprising Connections (you probably didn't know these)
- `2026-09-21 — Migrated auth to Supabase (created the project, deleted custom bcrypt+JWT)` --references--> `request()`  [INFERRED]
  docs/PROGRESS.md → dashboard/src/lib/api.ts
- `Where things stand` --references--> `main()`  [INFERRED]
  docs/PROGRESS.md → patchright_test.py
- `3. Security Findings` --references--> `RequireAuth()`  [INFERRED]
  docs/AUDIT_REPORT_2026-09-22.md → dashboard/src/components/RequireAuth.tsx
- `9. Architecture / File Structure Issues` --references--> `RequireAuth()`  [INFERRED]
  docs/AUDIT_REPORT_2026-09-22.md → dashboard/src/components/RequireAuth.tsx
- `Dashboard wiring: real account/domains/rules/settings API, same binary, no separate control-plane service — 2026-09-21` --references--> `RequireAuth()`  [INFERRED]
  docs/DECISIONS.md → dashboard/src/components/RequireAuth.tsx

## Import Cycles
- None detected.

## Communities (99 total, 15 thin omitted)

### Community 0 - "App.tsx"
Cohesion: 0.07
Nodes (65): App(), Layout(), LayoutContext, tabs, Theme, ThemeContext, ThemeContextType, ThemeProvider() (+57 more)

### Community 1 - "velocity_test.go"
Cohesion: 0.17
Nodes (18): TestEvaluateRunsStatefulChecksOnce(), checkJA4VelocitySpike(), checkVelocitySpike(), redis.Client, InitRedis(), redisRequestAllowed(), TestCheckJA4VelocitySpikeExemptsCommonBrowsers(), TestCheckJA4VelocitySpikeOverLimit() (+10 more)

### Community 2 - "RESEARCH.md — Reference Notes"
Cohesion: 0.07
Nodes (26): 2026-09-18 — How commercial vendors actually get to high block rates: layered, continuous, 2026-09-18 — Real-Chrome stealth automation (Patchright, Scrapling) beats every existing signal, 2026-09-20 — Open-source anti-bot landscape scan (what to learn from, what to take), ClientHello fragmentation — a cheap way past TLS fingerprinting, Competitor feature scan — 2026-09-15, Cost asymmetry: bandwidth is the wrong lever, CPU is the right one, Finding, How Akamai / DataDome / Cloudflare actually detect bots (+18 more)

### Community 3 - "challenge_test.go"
Cohesion: 0.28
Nodes (22): fetchPage(), newChallenge(), postVerify(), solvePoW(), TestChallengeHandlerMethodNotAllowed(), TestChallengePageDetectsAdvancedAutomation(), TestChallengePageObfuscatesAutomationTells(), TestChallengeRealFlowPasses() (+14 more)

### Community 4 - "identity.go"
Cohesion: 0.08
Nodes (35): ConnContext(), JA4FromContext(), NewCaptureListener(), TestJA4FromContext(), TestNewCaptureListener(), canonicalHost(), canonicalIP(), forwardedIPs() (+27 more)

### Community 5 - "go_pkg_time"
Cohesion: 0.19
Nodes (14): cacheBotResult(), hostnameMatchesDomain(), IsGoodBotClaim(), IsVerifiedGoodBot(), TestHostnameMatchesDomainRequiresBoundary(), TestIsGoodBotClaim(), TestIsVerifiedGoodBot_Spoofed(), TestIsVerifiedGoodBot_Success() (+6 more)

### Community 6 - "CLAUDE.md"
Cohesion: 0.08
Nodes (24): 10. Detection Architecture, 12. Mutation Verification, 14. False Positives Are a Production Problem, 16. Multi-Tenant Security, 17. Visitor-Controlled Input, 18. Security Review, 19. Resource and Cost Awareness, 1. Autonomous Engineering Rule (+16 more)

### Community 7 - "Store"
Cohesion: 0.05
Nodes (41): Mode, PolicyMode, ParseMode(), ParsePolicy(), TestModeString(), TestParseMode(), BenchmarkTrailRecent(), BenchmarkTrailRecord() (+33 more)

### Community 8 - "testing.T"
Cohesion: 0.06
Nodes (91): TestDashboardEvidenceHandler(), TestDashboardStatsHandler(), newTestStore(), TestDashboardStatsHandler_MethodNotAllowed(), TestDashboardStatsHandler_Success(), TestDashboardStatsHandler_UnknownTenant(), TestDashboardTopOffendersHandler_Success(), TestDashboardTopOffendersHandler_Unauthorized() (+83 more)

### Community 9 - "challenge.go"
Cohesion: 0.07
Nodes (28): canonicalHost(), Challenge, redis.Client, newLocalNonceStore(), NewRedisNonceStore(), randomNonce(), safeRedirectPath(), validCanvasProof() (+20 more)

### Community 10 - "What You Must Do When Invoked"
Cohesion: 0.08
Nodes (24): For /graphify add and --watch, For /graphify query, For the commit hook and native CLAUDE.md integration, For --update and --cluster-only, /graphify, Honesty Rules, Interpreter guard for subcommands, Part A - Structural extraction for code files (+16 more)

### Community 11 - "HakaiShield: Complete Pages Overview"
Cohesion: 0.04
Nodes (45): About Page (`/about`), 📄 All Pages (19 Total), Analytics, Authentication, Authentication Flow, Authentication Pages, Backend Integration, ✅ Build Status (+37 more)

### Community 12 - "package.json"
Cohesion: 0.11
Nodes (17): name, private, type, canvas-confetti, date-fns, @dnd-kit/core, @dnd-kit/sortable, @dnd-kit/utilities (+9 more)

### Community 13 - "HakaiShield: Fake Content Removed & Honest Positioning Added"
Cohesion: 0.05
Nodes (37): 1. **Changelog Page** (`/changelog`), 1. **"Founding Customer Program"**, 1. **Hero Section**, 2. **"Building in Public"**, 2. **Case Studies Section**, 2. **Docs Page** (`/docs`), 3. **Contact Page** (`/contact`), 3. **ROI Section** (+29 more)

### Community 14 - "Trust Builders - Investor & Client Confidence Features"
Cohesion: 0.05
Nodes (36): 1. **Interactive Demo Section**, 1. **Proves ROI**, 1. **Shows Market Validation**, 1. **Social Proof**, 2. **Authority**, 2. **Case Studies with Real Metrics**, 2. **Demonstrates Technical Depth**, 2. **Shows Transparency** (+28 more)

### Community 15 - "main"
Cohesion: 0.15
Nodes (15): asyncio, Built-in `.env` loader instead of a dependency; found a plaintext-password-in-logs bug while wiring it — 2026-09-21, 2026-09-14 — Zip landed in real repo; PR merged, 2026-09-15 — Session handoff: item 3 done, item 4 next, PR #2 about to be merged, 2026-09-16 — Dashboard wired to the real backend; dead code removed; merged a landed upstream PR, 2026-09-18 — CI pipeline added (there was none); panic recovery + optional Sentry reporting, 2026-09-21 — Got the Supabase DB password; closed the "never run against real Supabase Postgres" gap, found and fixed a password-leak bug, 2026-09-21 — Product rename: hakaishield → HakaiShield (on this branch, after main-history reconciliation) (+7 more)

### Community 16 - "API Endpoints"
Cohesion: 0.06
Nodes (36): API Endpoints, Authentication Endpoints, Base URL, Contact Form Endpoints, Dashboard Endpoints, Domains & SIEM Endpoints, GET /auth/me, GET /dashboard/stats (+28 more)

### Community 17 - "analyze_server.py"
Cohesion: 0.25
Nodes (7): index(), route, report(), home(), route, flask, json

### Community 19 - "PROGRESS.md — What Was Done, When, and Why"
Cohesion: 0.05
Nodes (40): 2026-09-14 — Audited against the standard library; deleted ~60 lines we shouldn't have written, 2026-09-14 — Backfilled RESEARCH.md and DECISIONS.md, 2026-09-14 — Comments simplified; CLAUDE.md comment rule tightened, 2026-09-14 — Corrected a real licensing mistake: MIT → proprietary, 2026-09-14 — Docs audit: fixed 5 defects, brought every file back in sync, 2026-09-14 — Fixed 2 real production bugs in the capture listener, 2026-09-14 — Independent security review found 2 issues I'd missed, 2026-09-14 — JA4 fingerprint computation (ROADMAP P0 item 2, partial) (+32 more)

### Community 20 - "vite.config.js"
Cohesion: 0.50
Nodes (3): @tailwindcss/vite, vite, @vitejs/plugin-react

### Community 21 - "HakaiShield Backend Implementation Plan"
Cohesion: 0.12
Nodes (16): Current Backend Baseline, Current Status Snapshot — 2026-09-22, Definition of Done for Each Feature, Explicitly Avoided, HakaiShield Backend Implementation Plan, Implementation Order, Non-Negotiable Product Rules, Phase 0 — Correctness and Security Foundation (+8 more)

### Community 23 - "13. Test Quality — Do Not Let Tests Lie"
Cohesion: 0.25
Nodes (8): 13. Test Quality — Do Not Let Tests Lie, Garbage accepted as valid, Never write a test merely to make the suite green, Only happy-path input, Stale tests, Tests of mocks instead of production code, Vacuous assertions, Weak error assertions

### Community 24 - "15. Production Safety"
Cohesion: 0.33
Nodes (6): 15. Production Safety, Cancellation, Concurrency, Failure behavior, Latency, Storage

### Community 25 - "2. Decision Rules — When to Act and When to Ask"
Cohesion: 0.50
Nodes (4): 2. Decision Rules — When to Act and When to Ask, Act without asking when:, Ask the owner when:, Research first when:

### Community 26 - "0. Mission"
Cohesion: 0.67
Nodes (3): 0. Mission, Product positioning, Product principle

### Community 27 - "32. Operating Principle"
Cohesion: 0.67
Nodes (3): 32. Operating Principle, graphify, When to use graphify vs archify

### Community 28 - "3. Documentation Is Project Memory"
Cohesion: 0.67
Nodes (3): 3. Documentation Is Project Memory, Document responsibilities, Mandatory documentation before stopping

### Community 29 - "8. Go Standards"
Cohesion: 0.67
Nodes (3): 8. Go Standards, Files, Naming

### Community 33 - "graphify reference: extra exports and benchmark"
Cohesion: 0.22
Nodes (8): graphify reference: extra exports and benchmark, Step 6b - Wiki (only if --wiki flag), Step 7 - Neo4j export (only if --neo4j or --neo4j-push flag), Step 7a - FalkorDB export (only if --falkordb or --falkordb-push flag), Step 7b - SVG export (only if --svg flag), Step 7c - GraphML export (only if --graphml flag), Step 7d - MCP server (only if --mcp flag), Step 8 - Token reduction benchmark (only if total_words > 5000)

### Community 34 - "graphify reference: query, path, explain"
Cohesion: 0.33
Nodes (5): For /graphify explain, For /graphify path, graphify reference: query, path, explain, Step 0 — Constrained query expansion (REQUIRED before traversal), Step 1 — Traversal

### Community 35 - "graphify reference: add a URL and watch a folder"
Cohesion: 0.50
Nodes (3): For /graphify add, For --watch, graphify reference: add a URL and watch a folder

### Community 36 - "graphify reference: commit hook and native CLAUDE.md integration"
Cohesion: 0.50
Nodes (3): For git commit hook, For native CLAUDE.md integration, graphify reference: commit hook and native CLAUDE.md integration

### Community 37 - "graphify reference: incremental update and cluster-only"
Cohesion: 0.50
Nodes (3): For --cluster-only, For --update (incremental re-extraction), graphify reference: incremental update and cluster-only

### Community 42 - "score_test.go"
Cohesion: 0.15
Nodes (23): Analyze(), Decide(), DecideWithPolicy(), Evaluate(), Decision, Score(), facts(), realBrowserHeaders() (+15 more)

### Community 43 - "Changes Made to Remove AI-Generated Feel"
Cohesion: 0.13
Nodes (14): 10. **Code-Style Comments**, 1. **Removed Excessive Icons**, 2. **Simplified Navigation**, 3. **Metrics Cards - Less Visual Noise**, 4. **Shadow Mode Banner**, 5. **Section Headers**, 6. **Landing Page - Complete Rewrite**, 7. **Pricing Page - Stripped Down** (+6 more)

### Community 44 - "Codex Continuation Handoff"
Cohesion: 0.12
Nodes (15): 1. Host/SNI and tenant lifecycle validation, 2. Postgres tenant-isolation integration coverage, 3. Observability before tuning limits, Codex Continuation Handoff, Documentation state, Later backlog from the implementation plan, Next work: Phase 1, Phase 0 closeout completed in this pass (+7 more)

### Community 45 - "HakaiShield Backend Wiring Documentation"
Cohesion: 0.14
Nodes (13): Architecture Overview, Email Services, Email Templates, Environment Variables, HakaiShield Backend Wiring Documentation, Overview, Payment Integration, Real-time Features (+5 more)

### Community 46 - "dependencies"
Cohesion: 0.14
Nodes (14): dependencies, canvas-confetti, date-fns, @dnd-kit/core, @dnd-kit/sortable, @dnd-kit/utilities, framer-motion, lucide-react (+6 more)

### Community 47 - "compilerOptions"
Cohesion: 0.14
Nodes (13): compilerOptions, allowImportingTsExtensions, esModuleInterop, isolatedModules, jsx, lib, module, moduleResolution (+5 more)

### Community 48 - "HakaiShield Dashboard - Light Mode Implementation"
Cohesion: 0.11
Nodes (17): 1. Theme System (`src/context/ThemeContext.tsx`), 2. CSS Variables (`src/index.css`), 3. Layout Component (`src/components/Layout.tsx`), 4. All Pages Updated, Build Status, Changes Made, Component Updates, CSS Variables Structure (+9 more)

### Community 49 - "honeypot_test.go"
Cohesion: 0.29
Nodes (15): honeypotKey(), HoneypotTripped(), RecordHoneypotTrip(), sweepHoneypotLocked(), BenchmarkHoneypotTrippedEmpty(), resetHoneypot(), TestHoneypotConcurrentAccess(), TestHoneypotIgnoresIncompleteIdentity() (+7 more)

### Community 50 - "middleware_test.go"
Cohesion: 0.36
Nodes (9): testClaims, testAuth, jwt.RegisteredClaims, newTestAuth(), TestRequireAuth_AcceptsValidTokenAndSetsUserID(), TestRequireAuth_OptionsPassesWithoutToken(), TestRequireAuth_RejectsInvalidToken(), TestRequireAuth_RejectsMissingToken() (+1 more)

### Community 51 - "main"
Cohesion: 0.23
Nodes (24): envelope, toggleRuleRequest, main(), callerDomain(), EvidenceLogsHandler(), TopOffendersHandler(), DomainsHandler(), DashboardStatsHandler() (+16 more)

### Community 52 - "DECISIONS.md — Why We Chose What We Chose"
Cohesion: 0.05
Nodes (41): Added 2 competitor-gap items to roadmap, rejected the rest — 2026-09-15, agentchat: removed the MCP server, kept plain file + manual relay — 2026-09-15, Assemble proven open-source pieces, don't reinvent TLS/fingerprint parsing — 2026-09-14, Automation probe lives inside the JS challenge, not injected site-wide — 2026-09-16, Back to one agent; Claude Code owns the dashboard too — 2026-09-16, Cap concurrent TLS handshakes at 1000 — 2026-09-14, Client IP comes from X-Forwarded-For only behind configured proxy CIDRs — 2026-09-22, Consolidating PR #10 and PR #11; dropping the forensics analyzer — 2026-09-20 (+33 more)

### Community 53 - "hakaishield"
Cohesion: 0.18
Nodes (10): Dashboard (frontend), Documentation, hakaishield, Key Capabilities:, License, Responsible use, Shadow mode, Try it locally (+2 more)

### Community 54 - "proxy.go"
Cohesion: 0.07
Nodes (39): createRuleRequest, ClientIPFromContext(), DecisionFromContext(), ScoreFromContext(), WithClientIP(), WithDecision(), CreateDomain(), GetTenant() (+31 more)

### Community 55 - "Page-by-Page Wiring Guide"
Cohesion: 0.20
Nodes (10): 10. Subscription Page (`/subscription`), 11. Payment Page (`/payment`), 3. Sign Up Page (`/sign-up`), 4. Sign In Page (`/sign-in`), 5. Onboarding Page (`/onboarding`), 6. Dashboard Overview (`/`), 7. Evidence Logs Page (`/evidence-logs`), 8. Mitigation Rules Page (`/mitigation-rules`) (+2 more)

### Community 56 - "devDependencies"
Cohesion: 0.20
Nodes (10): devDependencies, tailwindcss, @tailwindcss/vite, @types/canvas-confetti, @types/react, @types/react-dom, @types/uuid, typescript (+2 more)

### Community 57 - "2026-09-16 — Market scan; repositioned the product away from "affordable alternative""
Cohesion: 0.22
Nodes (9): 2026-09-16 — Market scan; repositioned the product away from "affordable alternative", How this was checked, New roadmap items — what they are and the risk on each, Next session should, What changed, file by file, What is NOT covered / honest gaps, What the research found, Why every doc, not just one (+1 more)

### Community 58 - "2026-09-16 — Pivot to hosted SaaS; self-hosting becomes Enterprise"
Cohesion: 0.22
Nodes (9): 2026-09-16 — Pivot to hosted SaaS; self-hosting becomes Enterprise, Files changed, Honest gaps, How this was checked, Next session should, The item worth reading twice, The upside I had not seen at first, What I made sure was understood before writing it down (+1 more)

### Community 59 - "mockData.ts"
Cohesion: 0.12
Nodes (13): asns, decisions, domains, exceptions, geos, ips, ja4Hashes, managedRules (+5 more)

### Community 60 - "Upcoming Enterprise Innovations"
Cohesion: 0.22
Nodes (8): Done, How to read this list, P0 — MVP (prove the core idea works), P0-SaaS — required before anyone can pay us, P1 — makes it meaningfully harder to bypass, Product direction: multi-signal scoring proxy, Upcoming Enterprise Innovations, What hakaishield is

### Community 61 - "Data Models"
Cohesion: 0.25
Nodes (8): Applications Table, Data Models, Domains Table, Rules Table, SIEM Integrations Table, Subscriptions Table, Traffic Logs Table, Users Table

### Community 62 - "2026-09-16 — Decision evidence trail (ROADMAP item 12a, done)"
Cohesion: 0.25
Nodes (8): 2026-09-16 — Decision evidence trail (ROADMAP item 12a, done), Honest gaps (Section 16), Next session should, Section 24 check, Tested how — nine mutations, all caught, The decision that shaped it, What NO test covers right now, What was built

### Community 63 - "2026-09-16 — Shadow mode (ROADMAP item 18, mode done); Antigravity ended; dashboard is ours now"
Cohesion: 0.25
Nodes (8): 2026-09-16 — Shadow mode (ROADMAP item 18, mode done); Antigravity ended; dashboard is ours now, A pre-existing lie I found and fixed, Antigravity ended, Honest gaps, Next session should, Shadow mode — what was built, Tested how — eight mutations, all caught, What NO test covers

### Community 64 - "go_pkg_net_http"
Cohesion: 0.15
Nodes (17): OffenderStats, statsResponse, authorized(), DashboardEvidenceHandler(), DashboardExportHandler(), DashboardTopOffendersHandler(), Init(), Middleware() (+9 more)

### Community 65 - "2026-09-16 — Answered "why not just build it yourself?"; named the moat"
Cohesion: 0.29
Nodes (7): 2026-09-16 — Answered "why not just build it yourself?"; named the moat, Honest gaps, How this was checked, Next session should, The reframe worth remembering, What changed, What prompted it

### Community 66 - "2026-09-17 — /code-review on my own shadow-mode work; two real bugs fixed"
Cohesion: 0.29
Nodes (7): 2026-09-17 — /code-review on my own shadow-mode work; two real bugs fixed, Bug 1 — the shadow-mode lie, inverted (`dashboard/src/lib/stats.ts`), Bug 2 — two sources of truth for the mode (`proxy/stats.go`), Cleanups (Section 24a), Tested how — three more mutations, all caught, What NO test covers, What this says about the earlier session

### Community 68 - "2026-09-16 — HANDOFF: PR #4 merged; next session's job is to break it"
Cohesion: 0.33
Nodes (6): 2026-09-16 — HANDOFF: PR #4 merged; next session's job is to break it, Report back with, Standing instruction from the project owner, What NOT to do, Where I would point an auditor first (honest list), Where things stand

### Community 69 - "1. Landing Page (`/landing`)"
Cohesion: 0.40
Nodes (5): 1. Landing Page (`/landing`), "Apply for founding access" button, "Get started" button (nav), Interactive demo "Run through hakaishield" button, "Start free trial" button (hero)

### Community 70 - "jwt_test.go"
Cohesion: 0.36
Nodes (12): testJWKS, newTestJWKS(), TestVerify_RecoversFromKeyRotation(), TestVerify_RejectsAlgNone(), TestVerify_RejectsExpiredToken(), TestVerify_RejectsGarbage(), TestVerify_RejectsMissingSubject(), TestVerify_RejectsTokenFromDifferentKey() (+4 more)

### Community 71 - "Authentication System"
Cohesion: 0.50
Nodes (4): Authentication System, Flow Diagram, JWT Token Structure, Middleware

### Community 72 - "Deployment Checklist"
Cohesion: 0.50
Nodes (4): Deployment Checklist, Performance, Pre-deployment, Security

### Community 73 - "Error Handling"
Cohesion: 0.50
Nodes (4): Error Codes, Error Handling, Error Handling Middleware, Standard Error Response Format

### Community 74 - "scripts"
Cohesion: 0.50
Nodes (4): scripts, build, dev, typecheck

### Community 75 - "2026-09-20 — Consolidated PR #10 + PR #11 into one hardened change"
Cohesion: 0.50
Nodes (4): 2026-09-20 — Consolidated PR #10 + PR #11 into one hardened change, Bugs found and fixed, Dropped, What shipped

### Community 76 - "2. Pricing Page (`/pricing`)"
Cohesion: 0.67
Nodes (3): 2. Pricing Page (`/pricing`), "Start free trial" buttons (all plans), "Talk to sales" button (Enterprise)

### Community 79 - "go_pkg_testing"
Cohesion: 0.12
Nodes (21): isolationAuth, claims, jwk, jwksResponse, normalizeOrigin(), TestNormalizeOrigin(), newIsolationAuth(), TestDashboardAPIsRespectDomainOwnership() (+13 more)

### Community 80 - "main.go"
Cohesion: 0.10
Nodes (19): addDomainRequest, ctxKeyUserID, domainJSON(), newDomainID(), safeTestDatabaseURL(), go_pkg_bufio, go_pkg_bytes, go_pkg_crypto_rand (+11 more)

### Community 81 - "go_pkg_strings"
Cohesion: 0.18
Nodes (7): claimsBrowser(), TestUAMismatch(), UAMismatch(), go_pkg_fmt, go_pkg_github_com_wi1dcard_fingerproxy_pkg_ja4, go_pkg_path, go_pkg_strings

### Community 82 - "Decision"
Cohesion: 0.67
Nodes (3): Decision, 2026-09-15 — Scoring engine v1 (ROADMAP P0 item 5, done); Antigravity joins the project, 2026-09-19 — Repair the broken test suite left behind by the PoW/theme change; wire header_anomaly; remove dead probe endpoint

### Community 83 - "Inc"
Cohesion: 0.48
Nodes (6): CountersHandler(), Inc(), resetCountersForTest(), Snapshot(), TestCountersSnapshotAndHandler(), go_pkg_sync_atomic

### Community 87 - "go_pkg_context"
Cohesion: 0.26
Nodes (11): AddCommonBrowserPrefix(), AddKnownScraperJA4(), redis.Client, isCommonBrowserJA4(), IsKnownScraperJA4(), StartJA4Sync(), syncJA4FromRedis(), TestJA4DynamicDatabase() (+3 more)

### Community 88 - "hakaishield — Production-Grade Full Software Audit"
Cohesion: 0.15
Nodes (12): 10. Dead / Redundant Code, 11. Quick Wins (kam effort, achha payoff), 12. High-Priority Fixes (production se pehle zaroori), 13. Long-Term Improvements, 1. Executive Summary (2 minute ka TL;DR), 4. Cloud Cost / Waste Findings, 5. Database Problems, 6. Performance & Scalability (+4 more)

### Community 89 - "newTestRedis"
Cohesion: 0.35
Nodes (11): CrawlPatternSuspected(), isStaticAsset(), TestCrawlPatternExemptsAssets(), TestCrawlPatternExemptsNonBrowser(), TestCrawlPatternFiresOnManyDistinctPaths(), TestCrawlPatternIsolatedPerIP(), TestCrawlPatternNoRedisFailsOpen(), TestCrawlPatternRepeatedSamePathDoesNotFire() (+3 more)

### Community 90 - "3. What's MISSING — documented gaps, not guesses"
Cohesion: 0.17
Nodes (11): 1. Scoring model (how all of this combines), 2. Signals currently BUILT and running, 3. What's MISSING — documented gaps, not guesses, 4. Quick answer: "what would a sophisticated bot get past today?", Allowlist / exemption logic (reduces false positives, not a score signal), Built, but narrower than the roadmap originally scoped, Config / policy gaps (not signals, but they blunt the signals that exist), Detection signals not built at all (+3 more)

### Community 91 - "Verifier"
Cohesion: 0.29
Nodes (6): decodeECPublicKey(), Verifier, NewVerifier(), TestNewVerifier_RejectsEmptyURL(), crypto/ecdsa.PublicKey, net/http.Client

### Community 92 - "main_test.go"
Cohesion: 0.25
Nodes (10): loadDotEnv(), redactCredentials(), TestLoadDotEnv(), TestLoadDotEnv_DoesNotOverrideExistingEnv(), TestLoadDotEnv_MissingFileIsNotAnError(), TestRedactCredentials(), TestRedactCredentials_NeverLeaksPasswordSubstring(), TestRedactCredentials_UnparseableInputDoesNotPanic() (+2 more)

### Community 93 - "hakaishield Architecture"
Cohesion: 0.20
Nodes (9): hakaishield Architecture, How a request flows today — BUILT, How it's delivered — Hosted Enterprise SaaS, Known architectural limits — BUILT code only, Request path budget, System overview, Tech stack, What the origin receives — BUILT (+1 more)

### Community 94 - "AGENT.md — Mission Brief for Any Coding Agent"
Cohesion: 0.29
Nodes (6): AGENT.md — Mission Brief for Any Coding Agent, How the docs fit together, What does success look like?, What to actively watch for (own initiative, not just when asked), Why are we building this?, You are hakaishield's principal engineer and de facto CTO

### Community 95 - "RequireAuth"
Cohesion: 0.33
Nodes (6): RequireAuth(), 3. Security Findings, 9. Architecture / File Structure Issues, Dashboard wiring: real account/domains/rules/settings API, same binary, no separate control-plane service — 2026-09-21, Dropped Go 1.22+ method-prefixed route patterns for the dashboard API — 2026-09-21, 2026-09-21 — Playwright end-to-end check found and fixed a real CORS bug, plus fake billing UI

### Community 96 - "2. Critical Issues (P0)"
Cohesion: 0.40
Nodes (5): 2.1 Honeypot trap aur healthz production mein unreachable hain (ROUTE SHADOWING), 2.2 JA4 aggregate velocity cap real browsers ko challenge karega, 2.3 Unknown Host header → har request pe uncached Postgres query, 2.4 SSRF + origin validation missing (dashboard → proxy), 2. Critical Issues (P0)

### Community 97 - "Prioritized Action Plan"
Cohesion: 0.40
Nodes (5): P0 — Turant fix, P1 — Production/high traffic se pehle, P2 — Jaldi improve, P3 — Nice to have, Prioritized Action Plan

### Community 98 - "FAILUROFTHISPRODUCT.MD"
Cohesion: 0.50
Nodes (3): Bot Shield Product Quality Testing, Current Testing Failure, Testing Objective

## Knowledge Gaps
- **599 isolated node(s):** `github.com/ToufiqQureshi/hakaishield`, `addDomainRequest`, `statsResponse`, `ctxKeyUserID`, `OffenderStats` (+594 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 689 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **15 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `PROGRESS.md — What Was Done, When, and Why` connect `PROGRESS.md — What Was Done, When, and Why` to `App.tsx`, `2026-09-16 — Answered "why not just build it yourself?"; named the moat`, `2026-09-17 — /code-review on my own shadow-mode work; two real bugs fixed`, `2026-09-16 — HANDOFF: PR #4 merged; next session's job is to break it`, `2026-09-20 — Consolidated PR #10 + PR #11 into one hardened change`, `main`, `Decision`, `RequireAuth`, `2026-09-16 — Market scan; repositioned the product away from "affordable alternative"`, `2026-09-16 — Pivot to hosted SaaS; self-hosting becomes Enterprise`, `2026-09-16 — Decision evidence trail (ROADMAP item 12a, done)`, `2026-09-16 — Shadow mode (ROADMAP item 18, mode done); Antigravity ended; dashboard is ours now`?**
  _High betweenness centrality (0.037) - this node is a cross-community bridge._
- **Why does `2026-09-21 — Wired the dashboard to a real account/domains/rules/settings API` connect `App.tsx` to `PROGRESS.md — What Was Done, When, and Why`, `RequireAuth`?**
  _High betweenness centrality (0.023) - this node is a cross-community bridge._
- **Why does `RequireAuth()` connect `RequireAuth` to `App.tsx`?**
  _High betweenness centrality (0.020) - this node is a cross-community bridge._
- **What connects `github.com/ToufiqQureshi/hakaishield`, `addDomainRequest`, `statsResponse` to the rest of the system?**
  _599 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `App.tsx` be split into smaller, more focused modules?**
  _Cohesion score 0.06615240766713418 - nodes in this community are weakly interconnected._
- **Should `RESEARCH.md — Reference Notes` be split into smaller, more focused modules?**
  _Cohesion score 0.07407407407407407 - nodes in this community are weakly interconnected._
- **Should `identity.go` be split into smaller, more focused modules?**
  _Cohesion score 0.08305647840531562 - nodes in this community are weakly interconnected._