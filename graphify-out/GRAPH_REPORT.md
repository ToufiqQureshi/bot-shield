# Graph Report - bot-shield  (2026-09-21)

## Corpus Check
- 113 files · ~113,907 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 6 file(s) not represented in the graph (top: (none) 5, .css 1)

## Summary
- 629 nodes · 1456 edges · 33 communities (27 shown, 6 thin omitted)
- Extraction: 93% EXTRACTED · 7% INFERRED · 0% AMBIGUOUS · INFERRED: 101 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `910c315b`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- App.tsx
- guard_test.go
- proxy.go
- NewTrail
- go_pkg_testing
- newTestRedis
- CLAUDE.md
- Store
- honeypot_test.go
- challenge.go
- score_test.go
- testing.T
- package.json
- dependencies
- compilerOptions
- patchright_test.py
- devDependencies
- analyze_server.py
- patchright_sync_api
- scripts
- vite.config.js
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

## God Nodes (most connected - your core abstractions)
1. `useTheme()` - 31 edges
2. `NewOriginProxy()` - 21 edges
3. `NewStore()` - 21 edges
4. `NewGuard()` - 19 edges
5. `react` - 18 edges
6. `NewChallenge()` - 18 edges
7. `lucide-react` - 17 edges
8. `Store` - 17 edges
9. `newTestRedis()` - 15 edges
10. `main()` - 15 edges

## Surprising Connections (you probably didn't know these)
- `DashboardTopOffendersHandler()` --calls--> `authorized()`  [INFERRED]
  backend/pkg/api/report.go → backend/pkg/api/handlers.go
- `TestHoneypotScoresBelowBlockOnItsOwn()` --calls--> `Analyze()`  [INFERRED]
  backend/pkg/signals/honeypot_test.go → backend/pkg/signals/score.go
- `TestHoneypotScoresBelowBlockOnItsOwn()` --calls--> `Score()`  [INFERRED]
  backend/pkg/signals/honeypot_test.go → backend/pkg/signals/score.go
- `deceiveResp()` --calls--> `WithDecision()`  [INFERRED]
  backend/pkg/core/proxy_test.go → backend/pkg/core/proxy.go
- `TestNewOriginProxy()` --calls--> `NewOriginProxy()`  [INFERRED]
  backend/pkg/core/proxy_test.go → backend/pkg/core/proxy.go

## Import Cycles
- None detected.

## Communities (33 total, 6 thin omitted)

### Community 0 - "App.tsx"
Cohesion: 0.06
Nodes (58): App(), Layout(), tabs, tenants, Theme, ThemeContext, ThemeContextType, ThemeProvider() (+50 more)

### Community 1 - "guard_test.go"
Cohesion: 0.06
Nodes (74): OffenderStats, statsResponse, main(), DashboardEvidenceHandler(), DashboardStatsHandler(), TestDashboardEvidenceHandler(), TestDashboardStatsHandler(), DashboardTopOffendersHandler() (+66 more)

### Community 2 - "proxy.go"
Cohesion: 0.11
Nodes (26): deceiveResponse(), DecisionFromContext(), readBody(), ScoreFromContext(), deceiveResp(), TestDeceiveResponseDoesNotBufferLargeBodies(), TestDeceiveResponseIgnoresUndeceivedTraffic(), TestDeceiveResponseInjectsIntoHTML() (+18 more)

### Community 3 - "NewTrail"
Cohesion: 0.08
Nodes (24): TestNewCaptureListener(), BenchmarkTrailRecent(), BenchmarkTrailRecord(), BenchmarkTrailRecordAndRead(), Trail, NewTrail(), newTrail(), TestTrailConcurrentRecords() (+16 more)

### Community 4 - "go_pkg_testing"
Cohesion: 0.06
Nodes (48): ConnContext(), JA4FromContext(), NewCaptureListener(), TestJA4FromContext(), ParseJA4(), TestJA4FingerprintRejectsGarbage(), TestParseJA4(), IsGoodBotClaim() (+40 more)

### Community 5 - "newTestRedis"
Cohesion: 0.13
Nodes (27): CrawlPatternSuspected(), isStaticAsset(), TestCrawlPatternExemptsAssets(), TestCrawlPatternExemptsNonBrowser(), TestCrawlPatternFiresOnManyDistinctPaths(), TestCrawlPatternIsolatedPerIP(), TestCrawlPatternNoRedisFailsOpen(), TestCrawlPatternRepeatedSamePathDoesNotFire() (+19 more)

### Community 6 - "CLAUDE.md"
Cohesion: 0.08
Nodes (24): 10. Detection Architecture, 12. Mutation Verification, 14. False Positives Are a Production Problem, 16. Multi-Tenant Security, 17. Visitor-Controlled Input, 18. Security Review, 19. Resource and Cost Awareness, 1. Autonomous Engineering Rule (+16 more)

### Community 7 - "Store"
Cohesion: 0.09
Nodes (20): Mode, PolicyMode, ParseMode(), ParsePolicy(), TestModeString(), TestParseMode(), Stats, newStats() (+12 more)

### Community 8 - "honeypot_test.go"
Cohesion: 0.29
Nodes (15): honeypotKey(), HoneypotTripped(), RecordHoneypotTrip(), sweepHoneypotLocked(), BenchmarkHoneypotTrippedEmpty(), resetHoneypot(), TestHoneypotConcurrentAccess(), TestHoneypotIgnoresIncompleteIdentity() (+7 more)

### Community 9 - "challenge.go"
Cohesion: 0.15
Nodes (15): authorized(), DashboardExportHandler(), Challenge, randomNonce(), safeRedirectPath(), validCanvasProof(), validPoW(), challengeData (+7 more)

### Community 10 - "score_test.go"
Cohesion: 0.14
Nodes (23): WithDecision(), Analyze(), Decide(), DecideWithPolicy(), Decision, Score(), facts(), realBrowserHeaders() (+15 more)

### Community 11 - "testing.T"
Cohesion: 0.12
Nodes (39): fetchPage(), newChallenge(), postVerify(), solvePoW(), TestChallengeHandlerMethodNotAllowed(), TestChallengePageDetectsAdvancedAutomation(), TestChallengePageObfuscatesAutomationTells(), TestChallengeRealFlowPasses() (+31 more)

### Community 12 - "package.json"
Cohesion: 0.11
Nodes (17): name, private, type, canvas-confetti, date-fns, @dnd-kit/core, @dnd-kit/sortable, @dnd-kit/utilities (+9 more)

### Community 13 - "dependencies"
Cohesion: 0.14
Nodes (14): dependencies, canvas-confetti, date-fns, @dnd-kit/core, @dnd-kit/sortable, @dnd-kit/utilities, framer-motion, lucide-react (+6 more)

### Community 14 - "compilerOptions"
Cohesion: 0.14
Nodes (13): compilerOptions, allowImportingTsExtensions, esModuleInterop, isolatedModules, jsx, lib, module, moduleResolution (+5 more)

### Community 15 - "patchright_test.py"
Cohesion: 0.29
Nodes (8): asyncio, patchright_async_api, main(), test_scenario_1_stealth_bypass(), test_scenario_2_human_mouse_simulation(), test_scenario_3_multitab_crawling(), test_scenario_4_in_browser_api_flood(), time

### Community 16 - "devDependencies"
Cohesion: 0.20
Nodes (10): devDependencies, tailwindcss, @tailwindcss/vite, @types/canvas-confetti, @types/react, @types/react-dom, @types/uuid, typescript (+2 more)

### Community 17 - "analyze_server.py"
Cohesion: 0.25
Nodes (7): index(), route, report(), home(), route, flask, json

### Community 19 - "scripts"
Cohesion: 0.50
Nodes (4): scripts, build, dev, typecheck

### Community 20 - "vite.config.js"
Cohesion: 0.50
Nodes (3): @tailwindcss/vite, vite, @vitejs/plugin-react

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

## Knowledge Gaps
- **129 isolated node(s):** `CLAUDE.md — hakaishield Engineering Operating System`, `Product principle`, `Product positioning`, `1. Autonomous Engineering Rule`, `Act without asking when:` (+124 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 175 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **6 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `mockConn` connect `NewTrail` to `go_pkg_testing`?**
  _High betweenness centrality (0.014) - this node is a cross-community bridge._
- **Why does `Challenge` connect `challenge.go` to `guard_test.go`, `testing.T`?**
  _High betweenness centrality (0.014) - this node is a cross-community bridge._
- **Why does `Stats` connect `Store` to `guard_test.go`, `score_test.go`?**
  _High betweenness centrality (0.014) - this node is a cross-community bridge._
- **Are the 3 inferred relationships involving `NewOriginProxy()` (e.g. with `JA4FromContext()` and `TestNewOriginProxy()`) actually correct?**
  _`NewOriginProxy()` has 3 INFERRED edges - model-reasoned connections that need verification._
- **What connects `CLAUDE.md — hakaishield Engineering Operating System`, `Product principle`, `Product positioning` to the rest of the system?**
  _129 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `App.tsx` be split into smaller, more focused modules?**
  _Cohesion score 0.05818395533352924 - nodes in this community are weakly interconnected._
- **Should `guard_test.go` be split into smaller, more focused modules?**
  _Cohesion score 0.06265389876880985 - nodes in this community are weakly interconnected._