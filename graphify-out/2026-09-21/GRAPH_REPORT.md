# Graph Report - bot-shield  (2026-09-21)

## Corpus Check
- 113 files · ~113,907 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 6 file(s) not represented in the graph (top: (none) 5, .css 1)

## Summary
- 691 nodes · 1508 edges · 42 communities (32 shown, 10 thin omitted)
- Extraction: 93% EXTRACTED · 7% INFERRED · 0% AMBIGUOUS · INFERRED: 101 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `e1835f47`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- App.tsx
- guard_test.go
- proxy.go
- NewTrail
- go_pkg_testing
- testing.T
- CLAUDE.md
- Store
- honeypot_test.go
- challenge.go
- What You Must Do When Invoked
- challenge_test.go
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
- graphify reference: extra exports and benchmark
- graphify reference: query, path, explain
- graphify reference: add a URL and watch a folder
- graphify reference: commit hook and native CLAUDE.md integration
- graphify reference: incremental update and cluster-only
- graphify reference: GitHub clone and cross-repo merge
- graphify reference: transcribe video and audio
- .claude/CLAUDE.md
- extraction-spec.md

## God Nodes (most connected - your core abstractions)
1. `useTheme()` - 31 edges
2. `NewOriginProxy()` - 21 edges
3. `NewStore()` - 21 edges
4. `NewGuard()` - 19 edges
5. `react` - 18 edges
6. `NewChallenge()` - 18 edges
7. `lucide-react` - 17 edges
8. `Store` - 17 edges
9. `main()` - 15 edges
10. `newTestRedis()` - 15 edges

## Surprising Connections (you probably didn't know these)
- `NewOriginProxy()` --calls--> `JA4FromContext()`  [INFERRED]
  backend/pkg/core/proxy.go → backend/pkg/core/capture.go
- `TestNewOriginProxy()` --calls--> `NewOriginProxy()`  [INFERRED]
  backend/pkg/core/proxy_test.go → backend/pkg/core/proxy.go
- `TestNewOriginProxy_InvalidTarget()` --calls--> `NewOriginProxy()`  [INFERRED]
  backend/pkg/core/proxy_test.go → backend/pkg/core/proxy.go
- `deceiveResp()` --calls--> `WithDecision()`  [INFERRED]
  backend/pkg/core/proxy_test.go → backend/pkg/core/proxy.go
- `TestHoneypotScoresBelowBlockOnItsOwn()` --calls--> `Analyze()`  [INFERRED]
  backend/pkg/signals/honeypot_test.go → backend/pkg/signals/score.go

## Import Cycles
- None detected.

## Communities (42 total, 10 thin omitted)

### Community 0 - "App.tsx"
Cohesion: 0.06
Nodes (58): App(), Layout(), tabs, tenants, Theme, ThemeContext, ThemeContextType, ThemeProvider() (+50 more)

### Community 1 - "guard_test.go"
Cohesion: 0.06
Nodes (76): OffenderStats, statsResponse, main(), authorized(), DashboardEvidenceHandler(), DashboardStatsHandler(), TestDashboardEvidenceHandler(), TestDashboardStatsHandler() (+68 more)

### Community 2 - "proxy.go"
Cohesion: 0.09
Nodes (32): deceiveResponse(), DecisionFromContext(), readBody(), deceiveResp(), TestDeceiveResponseDoesNotBufferLargeBodies(), TestDeceiveResponseIgnoresUndeceivedTraffic(), TestDeceiveResponseInjectsIntoHTML(), TestDeceiveResponseLeavesNonHTMLUntouched() (+24 more)

### Community 3 - "NewTrail"
Cohesion: 0.07
Nodes (33): BenchmarkTrailRecent(), BenchmarkTrailRecord(), BenchmarkTrailRecordAndRead(), Trail, NewTrail(), newTrail(), TestTrailConcurrentRecords(), TestTrailLimit() (+25 more)

### Community 4 - "go_pkg_testing"
Cohesion: 0.06
Nodes (46): ConnContext(), JA4FromContext(), NewCaptureListener(), TestJA4FromContext(), TestNewCaptureListener(), ScoreFromContext(), GetTenant(), Init() (+38 more)

### Community 5 - "testing.T"
Cohesion: 0.08
Nodes (56): HeaderAnomaly(), TestHeaderAnomalyCaseInsensitiveHeaderLookup(), TestHeaderAnomalyCrawlerExempt(), TestHeaderAnomalyEmptyUA(), TestHeaderAnomalyHonestScriptingTool(), TestHeaderAnomalyMissingFetchMetadataFires(), TestHeaderAnomalyRealBrowserNavigation(), TestHeaderAnomalyScriptedClientFakingBrowser() (+48 more)

### Community 6 - "CLAUDE.md"
Cohesion: 0.08
Nodes (24): 10. Detection Architecture, 12. Mutation Verification, 14. False Positives Are a Production Problem, 16. Multi-Tenant Security, 17. Visitor-Controlled Input, 18. Security Review, 19. Resource and Cost Awareness, 1. Autonomous Engineering Rule (+16 more)

### Community 7 - "Store"
Cohesion: 0.09
Nodes (20): Mode, PolicyMode, ParseMode(), ParsePolicy(), TestModeString(), TestParseMode(), Stats, newStats() (+12 more)

### Community 8 - "honeypot_test.go"
Cohesion: 0.27
Nodes (16): honeypotKey(), HoneypotTripped(), RecordHoneypotTrip(), sweepHoneypotLocked(), BenchmarkHoneypotTrippedEmpty(), resetHoneypot(), TestHoneypotConcurrentAccess(), TestHoneypotIgnoresIncompleteIdentity() (+8 more)

### Community 9 - "challenge.go"
Cohesion: 0.15
Nodes (14): Challenge, randomNonce(), safeRedirectPath(), validCanvasProof(), validPoW(), WithDecision(), challengeData, go_pkg_crypto_hmac (+6 more)

### Community 10 - "What You Must Do When Invoked"
Cohesion: 0.08
Nodes (24): For /graphify add and --watch, For /graphify query, For the commit hook and native CLAUDE.md integration, For --update and --cluster-only, /graphify, Honesty Rules, Interpreter guard for subcommands, Part A - Structural extraction for code files (+16 more)

### Community 11 - "challenge_test.go"
Cohesion: 0.29
Nodes (18): fetchPage(), newChallenge(), postVerify(), solvePoW(), TestChallengeHandlerMethodNotAllowed(), TestChallengePageDetectsAdvancedAutomation(), TestChallengePageObfuscatesAutomationTells(), TestChallengeRealFlowPasses() (+10 more)

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

## Knowledge Gaps
- **171 isolated node(s):** `graphify`, `Usage`, `What graphify is for`, `Step 0 - GitHub repos and multi-path merge (only if a URL or several paths)`, `Step 1 - Ensure graphify is installed` (+166 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 227 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **10 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `mockConn` connect `NewTrail` to `go_pkg_testing`?**
  _High betweenness centrality (0.012) - this node is a cross-community bridge._
- **Why does `Challenge` connect `challenge.go` to `guard_test.go`, `challenge_test.go`?**
  _High betweenness centrality (0.012) - this node is a cross-community bridge._
- **Why does `Stats` connect `Store` to `guard_test.go`?**
  _High betweenness centrality (0.011) - this node is a cross-community bridge._
- **Are the 3 inferred relationships involving `NewOriginProxy()` (e.g. with `JA4FromContext()` and `TestNewOriginProxy()`) actually correct?**
  _`NewOriginProxy()` has 3 INFERRED edges - model-reasoned connections that need verification._
- **What connects `graphify`, `Usage`, `What graphify is for` to the rest of the system?**
  _171 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `App.tsx` be split into smaller, more focused modules?**
  _Cohesion score 0.05818395533352924 - nodes in this community are weakly interconnected._
- **Should `guard_test.go` be split into smaller, more focused modules?**
  _Cohesion score 0.06138975966562173 - nodes in this community are weakly interconnected._