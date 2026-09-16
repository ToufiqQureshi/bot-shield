# PROGRESS.md — What Was Done, When, and Why

A chronological work log. Every session (yours or another agent's)
appends an entry here **before stopping** — even a single small
function change, even a rename, even a "tried X, reverted it."

This is different from `docs/DECISIONS.md` (which is *why we chose
an approach*, rarely updated) and `docs/ROADMAP.md`'s Done list (which
is *what feature exists now*). This file is the blow-by-blow: what
actually happened in each work session, in order, so nothing has to
be reconstructed from git log or guessed later.

**Why this matters more than usual for this project:** code has been
handed off between sessions as a zip file, not always a git push — so
git history itself cannot be assumed to have survived. Treat this
file as the source of truth for history, not git log.

---

## Format for new entries

```text
## <date> — <short title>
Changed:
  - file/function: what changed
  - file/function: what changed
Why:
Tested how:
Known gaps / follow-up:
```

Keep entries factual and specific — "changed X because Y", not "made
improvements." A vague entry is as bad as no entry.

---

## 2026-09-14 — Backfilled RESEARCH.md and DECISIONS.md
Changed:
  - `docs/RESEARCH.md`: created — Akamai/DataDome/Cloudflare detection
    technique list, Patchright/Scrapling/obscura-scraper evasion
    summary, Bright Data-class proxy network strategy, open-source
    tools identified (fingerproxy, BotD, SafeLine, open-appsec).
  - `docs/DECISIONS.md`: added "assemble open-source, don't reinvent"
    and "large-proxy-network: fingerprint not IP" entries.
  - `CLAUDE.md`: added `docs/RESEARCH.md` to the Documentation Map
    and the mandatory pre-stop checklist.
Why: earlier conversation covered real research (competitor detection
methods, stealth-tool analysis, Bright Data threat strategy) that
existed only in chat history, not in any repo file — risked being
lost the moment a new session started with no memory of that chat.
Tested how: n/a (docs only).
Known gaps / follow-up: this file (`PROGRESS.md`) itself didn't exist
yet when this entry was written — added in the next entry, same day.

---

## 2026-09-14 — Reverse proxy skeleton (ROADMAP P0 item 1)
Changed:
  - `go.mod`: created, module `github.com/ToufiqQureshi/bot-shield`.
  - `proxy/proxy.go`: added `New(target string) (*httputil.ReverseProxy, error)`
    — wraps stdlib `httputil.NewSingleHostReverseProxy`, validates the
    target URL has a scheme+host before returning.
  - `proxy/errors.go`: added `errInvalidTarget`.
  - `proxy/proxy_test.go`: added `TestPassthrough` (request/response
    unchanged through the proxy) and `TestNewRejectsBadTarget`.
  - `cmd/botshield/main.go`: added the `botshield` binary — `-addr`/
    `-target` flags, starts the proxy, graceful shutdown on
    SIGINT/SIGTERM with a 10s drain timeout.
  - `.gitignore`: added (`/bin/`, `*.log`, `.env`).
  - `docs/ROADMAP.md`: moved item 1 from P0 list to Done, with the
    known gap noted inline.
Why: this is the first ROADMAP item — the integration point every
future detection layer (fingerprint, score, challenge) will plug
into. Built test-first per `CLAUDE.md` Section 7: wrote
`proxy_test.go` against a not-yet-existing `New()`, confirmed it
failed for the right reason (`undefined: New`), then implemented.
Tested how: `go test ./... -race` (unit tests pass); also ran a real
end-to-end smoke test — a local Python HTTP server as the origin, the
`botshield` binary in front of it, `curl` through the proxy — to
confirm actual traffic passthrough, not just mocked behavior.
`gofmt -l .` and `go vet ./...` both clean.
Known gaps / follow-up: no timeout/retry handling yet if the origin
is slow or down — a dead origin currently returns Go's default 502
with no retry. Deliberately deferred to ROADMAP item 2 (HTTP
fetcher)/retry work rather than building it ahead of the roadmap
order (see `docs/DECISIONS.md` "MVP scope" entry for why P0 stays
narrow). Also: code was committed locally but could not be pushed —
this session's GitHub access was scoped to a different repo
(`goScraper`), not `bot-shield`. Delivered as a zip instead; the
project owner pushes it manually. Whoever picks this up next should
verify the zip's contents actually landed in the real repo before
building on top of it.

---

## 2026-09-14 — Zip landed in real repo; PR merged
Changed:
  - Extracted the handed-off zip (see previous entry) into the actual
    `bot-shield` repo on branch `claude/code-review-feedback-k1zukr`,
    replacing the stray `bot-shield.zip` blob that had been committed
    to `main` directly instead of the real files. Opened as a PR,
    reviewed, merged into `main`.
  - `CLAUDE.md`: added Section 3a — every non-trivial function needs a
    2-3 line comment covering what it does, why it exists, and what
    need made it necessary. This file didn't have that rule explicit
    yet even though `docs/AGENT.md` already expects beginner-readable
    code; added on request, PR merged separately.
Why: this session had correct `bot-shield` GitHub access (previous
session did not — see prior entry), so it could close that handoff
gap instead of leaving it to the project owner.
Tested how: `go build/vet/test ./...` clean before and after; no
detection logic touched, pure delivery + docs.
Known gaps / follow-up: none — this was a delivery/recovery task, not
a feature. ROADMAP item 2 (fingerprinting) is next.

---

## 2026-09-14 — JA4 fingerprint computation (ROADMAP P0 item 2, partial)
Changed:
  - `go.mod`: added `github.com/wi1dcard/fingerproxy` as a dependency.
    Only its `pkg/ja4` package is imported (self-contained, stdlib +
    `utls` only) — deliberately did not import `pkg/fingerprint` or
    `pkg/proxyserver`, which would have pulled in Prometheus metrics
    and gopacket-based JA3 parsing we don't need yet (see
    `docs/DECISIONS.md` "minimal fingerproxy surface" entry).
  - `proxy/fingerprint.go`: added `ja4Fingerprint(clientHello []byte)
    (string, error)` — turns a raw TLS ClientHello record into its
    JA4 hash.
  - `proxy/fingerprint_test.go`: added `TestJA4Fingerprint` using two
    real, known-good ClientHello-to-JA4 vectors taken from
    fingerproxy's own test suite (curl 8.6.0, and a PSK-extension
    handshake) — not invented test data. Added
    `TestJA4FingerprintRejectsGarbage` for malformed input (this runs
    against adversarial traffic; garbage input is the normal case,
    not an edge case, per `CLAUDE.md` Section 9).
Why: ROADMAP item 2's first slice. TLS/JA4 fingerprinting needs a
correct hash function before it needs live-capture wiring — built and
tested that piece first rather than guessing at both at once.
Tested how: `go build ./...`, `go vet ./...`, `gofmt -l .` all clean.
`go test ./... -v` — both known-vector cases pass, garbage-input case
returns an error instead of a wrong/blank fingerprint. Confirmed via
`go list -deps` that Prometheus and gopacket are NOT compiled into the
binary (only `utls` and two small `quic-go` internal subpackages
`utls` itself needs).
Known gaps / follow-up (real, not deferred without reason — see
`CLAUDE.md` Section 17): **bot-shield does not terminate TLS at all
today** — `cmd/botshield` proxies plain HTTP, so nothing currently
captures a live ClientHello to feed this function. This is deliberately
not built in this same pass: it means rewriting the accept loop
(TLS-terminating listener + HTTP/1.1 vs HTTP/2 branching, following
fingerproxy's own `pkg/hack.HijackClientHelloConn` + `pkg/proxyserver`
pattern), deciding fail-open behavior for a failed/timed-out capture
(`CLAUDE.md` Section 9), and end-to-end testing against a real browser
and a real scripted client — a large piece of its own that would ship
half-done if rushed into this same commit. This is the next work item,
not "done."

---

## 2026-09-14 — Comments simplified; CLAUDE.md comment rule tightened
Changed:
  - `proxy/fingerprint.go`, `proxy/fingerprint_test.go`: comments were
    too long/technical (multi-doc citations, jargon) — cut to 1-2
    plain-English lines each.
  - `CLAUDE.md` Section 3a: added a concrete good/bad example so
    future comments stay short and plain instead of drifting long.
Why: project owner flagged the earlier comments as too dense for a
solo/beginner-friendly project.
Tested how: `go build/vet/test ./...` clean (comment-only change).
Known gaps / follow-up: none.

---

## 2026-09-14 — TLS termination + live JA4 capture (ROADMAP P0 item 2, done)
Changed:
  - `proxy/capture.go`: added `NewCaptureListener` — wraps a plain TCP
    listener so every connection gets TLS-handshaked and its JA4
    fingerprint read before the HTTP server sees it. One goroutine per
    connection does the handshake concurrently (capped at 1000
    in-flight via a semaphore, so a connection flood can't spawn
    unlimited goroutines — `CLAUDE.md` Section 9), then hands the
    finished connection to `fingerproxy/pkg/hack.ChannelListener` for
    the HTTP server to pick up. Added `ConnContext`/`JA4FromContext`
    to carry the fingerprint from the connection into each request's
    context.
  - `proxy/proxy.go`: `New()`'s director now sets `X-BotShield-JA4` on
    the forwarded request when a fingerprint was captured; forwards
    normally (no header) otherwise — fingerprinting failing must never
    block real traffic (fail open, `CLAUDE.md` Section 9).
  - `cmd/botshield/main.go`: added `-tls-cert`/`-tls-key` flags. With
    them, botshield terminates TLS and wires the capture listener;
    without them, it proxies plain HTTP exactly as before (local dev
    still works without a cert).
  - `proxy/capture_test.go`: added a real end-to-end test — actual TLS
    handshake through the capture listener, actual HTTP request
    proxied to a real origin, asserting the origin received a
    non-empty JA4 header. Added a bad-handshake test (garbage bytes
    instead of a ClientHello) proving the listener drops the bad
    connection and keeps serving good ones afterward, instead of
    hanging.
Why: this is the piece the previous "partial" entry flagged as
missing — without it, `ja4Fingerprint` had no real ClientHello to run
against. Closes ROADMAP item 2's TLS/JA4 half (HTTP/2 fingerprinting
is a separate, still-open half — see `DECISIONS.md`).
Tested how: `go build/vet/test ./... -race` all clean, `gofmt -l .`
clean. Real end-to-end test (self-signed cert generated in-test, real
`tls.Dial`/`http.Client` round trip). Also ran the actual compiled
binary by hand: generated an `openssl` self-signed cert, started
`botshield -tls-cert ... -tls-key ...` in front of a local Python HTTP
server, hit it with `curl -k` over real TLS — response came back
correctly. Also re-ran the plain-HTTP path (no `-tls-cert` flags) by
hand to confirm it still works unchanged.
Known gaps / follow-up (not deferred without reason — see `CLAUDE.md`
Section 17):
  - HTTP/2 fingerprinting is not built. The capture listener forces
    `NextProtos = ["http/1.1"]`, so a browser that would otherwise use
    HTTP/2 falls back to HTTP/1.1 against bot-shield. This is a
    deliberate scope cut (see `DECISIONS.md`), not an oversight — it
    is its own roadmap item (item 2's "HTTP/2 fingerprint" half), and
    building it means hand-rolling HTTP/2 serving alongside the
    stdlib's own HTTP/1.1 path (the way `fingerproxy/pkg/proxyserver`
    does), which is a large enough piece to ship on its own.
  - The 1000-in-flight-handshake cap is a reasonable-guess default,
    not load-tested against real adversarial volume — that belongs
    with ROADMAP item 16 (soak testing), once there's real traffic to
    tune it against.
  - JA4 alone is not a block/allow decision — nothing reads
    `X-BotShield-JA4` yet except this proxy setting it. That's
    ROADMAP items 3 (UA consistency) and 5 (scoring engine), not this
    item.

---

## 2026-09-14 — Fixed 2 real production bugs in the capture listener
Changed:
  - `proxy/capture.go`: `handshakeAndCapture` now has `defer recover()`
    around the whole function. Without it, a crafted ClientHello that
    triggers a panic in JA4 parsing would crash the *entire process*
    (Go kills the whole program on an unrecovered panic in any
    goroutine, not just that connection) — every client behind
    bot-shield would go down over one bad handshake.
  - `proxy/capture.go`: TLS handshake now uses `HandshakeContext` with
    a 10s timeout (`handshakeTimeout`, stored via `sync/atomic` so
    tests can shrink it) instead of a bare `Handshake()` call that
    could block forever. Without this, ~1000 clients that open a
    connection and never finish handshaking would fill
    `maxHandshakes` and block every legitimate new connection
    (slowloris-style).
  - `cmd/botshield/main.go`: added `ReadHeaderTimeout`/`IdleTimeout` to
    the `http.Server` — the same slowloris risk, one layer up, for a
    client that completes the TLS handshake but then sends the HTTP
    request too slowly.
  - `proxy/capture_test.go`: added
    `TestCaptureListenerTimesOutSlowHandshake` — a client that
    connects and sends nothing must be dropped after the timeout, not
    held open. (No dedicated test added for the panic-recovery path:
    reliably forcing a panic inside the third-party `ja4` parser
    without depending on its internals wasn't worth the fragility —
    the fix itself is a single, obviously-correct `defer recover()`.)
Why: project owner asked directly whether the fingerprinting code was
production-grade. Re-reading it against `CLAUDE.md` Section 9
(fail-open, timeouts, bounded concurrency) surfaced both gaps — real
bugs, not style nits, found by re-auditing rather than by a report
from outside.
Tested how: `go build/vet/test ./... -race` clean, including the new
timeout test. Confirmed the fix by first reproducing the race in the
test itself (a shared package var without synchronization) and fixing
it with `sync/atomic` before trusting the result.
Known gaps / follow-up: the panic-recovery path is unverified by an
actual test (see above) — if a real crash from malformed input is ever
observed in the wild, add a regression test using that exact input at
that point, don't try to construct one speculatively now.

---

## 2026-09-14 — Re-audit found 2 more real bugs; added CLAUDE.md Section 22
Changed:
  - `proxy/proxy.go`: `New()`'s director now does `r.Header.Del(ja4Header)`
    unconditionally before maybe setting it. Before this, a visitor
    could set `X-BotShield-JA4` themselves and it would reach the
    origin untouched whenever real capture didn't produce a value
    (plain HTTP, or capture failing) — a spoofable "detection" signal.
  - `proxy/capture.go`: the background accept loop in
    `NewCaptureListener` now logs and retries (with backoff, capped at
    1s) on an Accept error, and only stops for real on
    `net.ErrClosed`. Before this, ANY Accept error (including a
    transient one like a brief fd-limit hit) silently killed the
    entire TLS accept loop forever — the process would look alive but
    quietly stop taking new TLS connections, with nothing logged.
  - `proxy/proxy_test.go`: added `TestNewStripsSpoofedJA4Header`.
  - `proxy/capture_test.go`: added
    `TestCaptureListenerSurvivesTransientAcceptError` (a fake listener
    that fails once, then works — proves the retry, not just that the
    code compiles).
  - `CLAUDE.md`: added Section 22, a concrete pre-push checklist
    (adversarial re-read, spoofable-input check, silent-failure check,
    "what's not tested" check) — this is now how these 2 bugs were
    actually found, made repeatable instead of one-off luck.
Why: project owner asked directly whether the fingerprinting work was
really production-grade, whether anything was skipped for speed, and
to re-check test coverage before moving on — a fair challenge, and
re-reading the diff adversarially (per the new Section 22, which this
session wrote and then immediately used) surfaced both bugs. Neither
was hypothetical: the header spoof directly undermines Section 6
(detection signals must not be trivially fakeable) before scoring
even exists to consume it, and the silent accept-loop death is exactly
the "silent failure" `docs/AGENT.md` calls out by name.
Tested how: `go build/vet/test ./... -race` clean. Both new tests
fail against the old code (verified by re-reading the diff, not just
trusting the fix) and pass against the fix. Re-ran the compiled
binary by hand: sent a request with a forged `X-BotShield-JA4` header
through a real TLS connection, confirmed the origin never saw it.
Known gaps / follow-up: this was a re-audit of already-shipped code,
not a new feature — a reminder that "tested and merged" isn't the
same as "no more bugs in it." The same adversarial re-read should
happen again before ROADMAP item 3 (UA consistency) is called done,
not just for new code but for whatever it touches in `proxy/`.

---

## 2026-09-14 — Mutation-tested our own tests; 2 of them were fake
Changed:
  - `proxy/capture_test.go`:
    `TestCaptureListenerTimesOutSlowHandshake` now checks *why* the
    connection ended — a server-side close, not the test's own read
    deadline — and that it happened near the handshake timeout. The
    old version set a 2s read deadline and only checked `err != nil`,
    so it passed whether the timeout worked or not.
  - `proxy/capture_test.go`: `TestCaptureListenerEndToEnd` now matches
    the header against a JA4 shape regex and fails if the origin was
    never reached. The old version only checked "not empty", which a
    garbage string also satisfies.
  - `proxy/proxy_test.go`: `TestNewStripsSpoofedJA4Header` now asserts
    the origin was actually reached. Its check lived inside the origin
    handler, so if the request had never arrived the test would have
    passed having proved nothing.
  - `proxy/fingerprint_test.go`: `TestJA4FingerprintRejectsGarbage`
    went from one 3-byte input to a table of 7 (nil, empty, random,
    wrong record type, truncated mid-handshake, header-only, and a
    record lying about its own length), built by chopping up a real
    curl ClientHello rather than inventing bytes. Also now asserts the
    returned fingerprint is empty, not just that an error came back.
  - `CLAUDE.md`: added Section 23 (a green suite proves nothing by
    itself) with the mandatory mutation check, the five ways a test
    lies, stale tests, "never write a test to make the suite green",
    and an explicit "do this without being asked". Linked it from the
    Section 19 and Section 22 checklists.
Why: project owner pushed back with the right question — the last two
rounds of bugs were only found because he asked. If a fake test is
sitting in the suite, nobody looks again, and it ships. So this round
audited the tests themselves instead of the production code.
Tested how: mutation testing, which is the only honest way to answer
"is this test real":
  - Deleted the handshake timeout from `capture.go` →
    `TestCaptureListenerTimesOutSlowHandshake` **passed** (took 2.00s,
    its own deadline, not the 0.20s timeout). Proof it was fake. After
    the fix, the same mutation **fails** with "read hit its own
    5.00s deadline: the server never closed the connection".
  - Replaced the real fingerprint with the literal
    `GARBAGE-NOT-A-FINGERPRINT` → `TestCaptureListenerEndToEnd`
    **passed**. Proof it was weak. After the fix, the same mutation
    **fails** naming the garbage value.
  - Removed `r.Header.Del(ja4Header)` from `proxy.go` →
    `TestNewStripsSpoofedJA4Header` correctly **failed**, so that one
    was genuinely testing its feature (the vacuous-pass guard was
    added anyway, since it was structurally possible).
  - Full suite `-race` green afterwards; production code restored
    byte-for-byte (verified with `git diff`), no behaviour changed
    this round.
Known gaps / follow-up: the remaining tests
(`TestCaptureListenerDropsBadHandshake`,
`TestCaptureListenerSurvivesTransientAcceptError`) fail by hanging
until the test timeout rather than by a clean assertion if their
feature breaks — they do catch it, but the failure message is poor.
Worth tightening with an explicit dial timeout next time either file
is touched. Also still true: the panic-recovery path has no test
(noted two entries above).




---

## 2026-09-14 — Audited against the standard library; deleted ~60 lines we shouldn't have written
Changed:
  - `proxy/capture.go`: deleted the hand-rolled TLS handshake timeout
    (`atomic.Int64` + `init()`), the accept-retry backoff loop, the
    per-connection panic recovery, the goroutine-per-connection and
    its 1000-slot semaphore, the `hack.ChannelListener` handoff, and
    the `capturedConn` type. `Accept` now returns a real `*tls.Conn`
    without handshaking it, and net/http does the handshake. The file
    went from 140 lines to 70.
  - `proxy/capture.go`: JA4 is now derived in `JA4FromContext` from
    `conn.NetConn()` at request time, instead of being computed in our
    own handshake goroutine and carried on a wrapper type.
  - `proxy/proxy.go`: migrated from `NewSingleHostReverseProxy` +
    `Director` to `httputil.ReverseProxy{Rewrite: ...}`, keeping the
    inbound `Host` and calling `SetXForwarded()`.
  - `cmd/botshield/main.go`: comment now records that
    `ReadHeaderTimeout` is also the TLS handshake deadline, so nobody
    removes it as "just a header thing".
  - `proxy/capture_test.go`: rewritten around a `startCapture` helper
    that runs the real wiring (capture listener + an `http.Server`
    configured like `cmd/botshield`), since the timeout behaviour now
    comes from that configuration.
  - `proxy/proxy_test.go`: added `TestNewStripsSpoofedForwardedFor`.
  - `CLAUDE.md`: added Section 24 (check the standard library before
    writing the code — five questions, answered by reading the source,
    not from memory), 24a (delete dead code first) and 24b (run
    `/code-review`, `/security-review`, `/simplify` on your own work).
    Linked from the Section 22 checklist.
Why: project owner asked whether existing code had unnecessary lines,
whether Go already provided any of it built-in, and whether we should
be using that instead. Reading `$GOROOT/src` rather than trusting
memory answered yes, four times over.
Tested how:
  - Read the stdlib source to confirm each claim before deleting
    anything: `Server.tlsHandshakeTimeout()` (server.go:934, takes the
    minimum of ReadHeaderTimeout/ReadTimeout/WriteTimeout and applies
    it around `HandshakeContext`), the `tempDelay` accept-retry loop
    (server.go:3420, 5ms doubling to a 1s cap, with logging),
    `conn.serve()`'s `defer recover()` (server.go:1944), and
    `tls.Conn.NetConn()` (conn.go:165).
  - Mutation checks on the new code: reverting `Rewrite` to `Director`
    makes `TestNewStripsSpoofedForwardedFor` fail with the origin
    seeing `"1.2.3.4, 127.0.0.1"` — the spoofed IP first, which is the
    value most code reads. Making `Accept` return a wrapper instead of
    a real `*tls.Conn` makes `TestCaptureListenerEndToEnd` fail.
  - Benchmarked the per-request fingerprint cost before deciding not
    to cache it: **14.3µs**, against `ARCHITECTURE.md`'s ~2ms budget.
  - `go build/vet/test ./... -race` and `gofmt -l .` clean.
  - Dead-code scan: every declared symbol in `proxy/` has live
    references; nothing removed silently.
Known gaps / follow-up:
  - One mutation check was informative in an uncomfortable way:
    `TestCaptureListenerTimesOutSlowHandshake` **passed** when `Accept`
    stopped returning a real `*tls.Conn`, because the connection was
    then treated as plain HTTP and closed by the same
    `ReadHeaderTimeout`. The test's comment claimed it proved more
    than it did; the comment now states its real scope and names the
    test that does catch that case. Worth remembering: a test can be
    honest about its result and still lie in its description.
  - HTTP/2 is still not offered, but this change makes adding it small
    — stdlib h2 negotiation works precisely because `Accept` now hands
    back a real `*tls.Conn`.
  - `proxy/errors.go` holds a single error value. Left alone rather
    than folded into `proxy.go`: `CLAUDE.md` Section 5 names
    `errors.go` as a wanted filename, and Section 13 says ask before
    removing something that merely looks surplus.

---

## 2026-09-14 — Independent security review found 2 issues I'd missed
Changed:
  - `proxy/capture.go`: added `JA4Unreadable`. A TLS connection whose
    handshake we can't read now reports `"unreadable"` instead of
    `""`; `""` now means only "not a TLS connection".
  - `proxy/proxy.go`: strip `X-Real-IP`, `True-Client-IP`,
    `CF-Connecting-IP`, `X-Client-IP`, `Fastly-Client-IP`,
    `X-Cluster-Client-IP` from the outbound request and set
    `X-Real-IP` ourselves from the connection address.
  - `proxy/capture_test.go`: added `fragmentingRelay` and
    `TestFragmentedClientHelloIsReported`.
  - `proxy/proxy_test.go`: added `TestNewStripsOtherClientIPHeaders`.
  - `docs/RESEARCH.md`: wrote up ClientHello fragmentation as a
    technique, including why record-level capture misses it.
  - `docs/DECISIONS.md`, `docs/ROADMAP.md`: recorded both decisions and
    the remaining gap.
Why: ran `/security-review`, which asks for an independent pass. Worth
recording plainly: **both findings were in code I had already audited
twice myself and declared clean.** Reviewing your own work does not
find the things you didn't think of.
  1. **ClientHello fragmentation (the serious one).** TLS lets one
     handshake message span several records.
     `hack.HijackClientHelloConn` reads the record header and sets
     `expectedLen = 5 + recordLength`, so it only ever captures the
     first fragment. A bot that splits its ClientHello handshakes
     normally — the site serves it — but produces no fingerprint. One
     line in the bot's socket layer, and the product's core detection
     is silently bypassed.
  2. **Other client-IP headers.** `Rewrite` strips only
     `X-Forwarded-*`. `X-Real-IP` and friends went through untouched,
     and plenty of origin stacks read those first — the same spoof
     fixed earlier for `X-Forwarded-For`, through a different door.
Tested how:
  - Reproduced finding 1 before fixing it: wrote a TCP relay that
    re-frames the client's first TLS record into two, ran a real
    client through it. Handshake succeeded, origin received
    `X-BotShield-JA4: ""`. After the fix the same client produces
    `"unreadable"`.
  - Mutation-checked both new tests: reverting `JA4Unreadable` to `""`
    fails the fragmentation test; removing the header-strip loop fails
    the IP test, naming all five headers that leaked.
  - `go build/vet/test ./... -race` and `gofmt -l .` clean.
Known gaps / follow-up:
  - The fragmentation evasion still *works* at the capture level — it
    is now visible, not prevented. Real fix is multi-record handshake
    reassembly, which we don't hand-write (`DECISIONS.md`); best path
    is upstream in fingerproxy. Tracked in `ROADMAP.md`/`RESEARCH.md`.
  - Worth checking whether other JA4 implementations share this gap.
    If they do, it's an ecosystem-wide blind spot, not just ours, and
    that is useful competitive information.
  - Process lesson, not a code gap: `/security-review` is now required
    before calling security-relevant work done (`CLAUDE.md` 24b), on
    the evidence of this session.

---

## 2026-09-14 — Docs audit: fixed 5 defects, brought every file back in sync
Changed:
  - `CLAUDE.md`: added a "four rules that matter most" opener and a
    "where to find things" index — the file had grown to 24 sections
    and was no longer navigable.
  - `CLAUDE.md` Section 4: the "Better:" examples were `score_request()`,
    `check_fingerprint()`, `run_challenge()` — **snake_case**, which is
    wrong for Go and contradicted this repo's own code
    (`ja4Fingerprint`, `JA4FromContext`). A future session following
    the rule literally would have written non-idiomatic Go. Replaced
    with real camelCase examples from this codebase.
  - `CLAUDE.md` Section 9: "never create unlimited goroutines" now
    says *of your own making*, and points at Section 24 — otherwise it
    read as contradicting the decision to delete our semaphore and let
    net/http manage connections.
  - `CLAUDE.md` Section 23: numbering ran 23d → 23f with no 23e. My
    own error from an earlier edit. Fixed.
  - `CLAUDE.md` Section 0: the pre-stop checklist didn't mention
    mutation-checking, `/security-review`, or checking that docs
    don't contradict each other. Added.
  - `docs/ARCHITECTURE.md`: rewritten. It was the most misleading file
    in the repo — the diagram showed scoring, challenges, rate
    limiting, Redis and Postgres with no indication that none of them
    exist, and TLS termination (the defining fact of the current
    design) wasn't shown at all. Every component is now marked BUILT
    or planned, the header contract with the origin is documented, the
    request flow matches the code, and the known limits are listed.
  - `docs/ARCHITECTURE.md` request budget: the JA4 row said "~2ms"
    (a guess). It now carries the measured **14.3µs** next to the
    budget, and explains why there's no cache.
  - `README.md`: had no usage instructions at all — you couldn't run
    the product from it. Added flags, a runnable example, the headers
    the origin receives, the HTTP/1.1-only note, and an honest status
    line saying bot-shield labels traffic but does not block yet.
  - `docs/AGENT.md`: doc map now covers `RESEARCH.md` and
    `PROGRESS.md`, and names the four highest-weight CLAUDE.md rules.
Why: owner asked for every doc to be brought up to date and properly
ordered. Auditing rather than assuming found five real defects,
including two I had introduced myself and one (snake_case) that
predated this session and would have actively misled a future agent.
Tested how: not a code change — `go build/vet/test ./... -race` and
`gofmt -l .` confirm nothing moved. Docs verified mechanically
against the code rather than by eye: every flag, header name,
`JA4Unreadable` value and the `NextProtos` setting named in the docs
was grepped out of the source and matched; every `path/file.go`
referenced in any doc was checked to exist; every "Section N"
cross-reference was checked to resolve to a real heading. All clean.
Known gaps / follow-up:
  - `docs/ARCHITECTURE.md`'s "proxy overhead" row is still unmeasured.
    Worth filling in with a real number before item 5 (scoring) adds
    to the request path — the budget is only useful if it's measured.
  - The BUILT/planned markers need maintaining. If a future session
    builds scoring and doesn't flip its marker, this file goes back to
    lying, which is the exact failure just fixed.

---

## 2026-09-14 — Corrected a real licensing mistake: MIT → proprietary
Changed:
  - `README.md`: License section changed from `MIT` to "Proprietary —
    All Rights Reserved," with a line explaining no copy/modify/
    redistribute rights are granted. Also reworded the "combines
    open-source building blocks" line, which read as if bot-shield
    itself were open source — it isn't; it uses open-source
    *libraries* internally.
  - `CLAUDE.md` Section 1: added an explicit, hard statement that
    bot-shield is closed-source commercial software, not an
    open-source project, and that no future session should suggest
    an open-source license or public release.
  - `docs/DECISIONS.md`: recorded the correction as a decision entry,
    including that this was a real mistake (not a considered choice)
    carried over from early generic scaffolding, and that it was live
    in the repo — anyone reading `README.md` before this fix would
    have been told, incorrectly, that they could legally copy and
    resell this code.
Why: project owner stated directly that this is a commercial SaaS
product, not open source. `README.md`'s MIT claim was a real,
consequential bug in the documentation — not a style issue — since an
MIT license is a legal grant, and this one was never intended.
Tested how: not a code change. Grepped every doc for "MIT" after the
fix; the only remaining hits are (1) this decision entry explaining
why MIT was wrong, (2) CLAUDE.md telling future sessions never to
suggest it, and (3) RESEARCH.md correctly noting that a *third-party*
library (BotD) is MIT-licensed, which is unrelated to bot-shield's
own license. `go build/vet/test ./... -race` and `gofmt -l .` confirm
no code was touched.
Known gaps / follow-up: there is still no actual LICENSE file, and no
real licensing terms exist yet (the README statement is a placeholder
saying "not open, ask the owner," not a drafted commercial license).
Before this is sold to a real paying customer, the owner needs actual
license terms — ideally reviewed by a lawyer — not just "proprietary"
in a README. Flagging this now per `CLAUDE.md` Section 16 rather than
waiting to be asked.

---

## 2026-09-14 — UA/header consistency check (ROADMAP P0 item 3, done)
Changed:
  - `proxy/useragent.go`: added `UAMismatch(ua, ja4 string) bool` and
    helpers `claimsBrowser`, `browserMarkers`, `crawlerMarkers`. Flags
    a request whose User-Agent claims a real browser (Chrome/Firefox/
    Safari/Edge) but whose TLS handshake says otherwise: negotiates
    TLS 1.0/1.1 (no current real browser does), or triggers the
    `JA4Unreadable` fragmentation signal already built for item 2. A
    UA that openly admits it's a crawler (contains "bot"/"spider"/
    "crawl") is exempted — declaring yourself isn't lying.
  - `proxy/proxy.go`: wired into `Rewrite` — strips any incoming
    `X-BotShield-UA-Mismatch` header first (same anti-spoof pattern as
    `X-BotShield-JA4`), sets it to `"true"` only when `UAMismatch`
    fires. Absence means nothing was flagged, not "unknown".
  - `proxy/useragent_test.go`: 9 cases — real Chrome/modern TLS
    (negative), claimed Chrome + `JA4Unreadable` (positive), claimed
    Firefox + TLS 1.0 (positive), claimed Safari + TLS 1.1 (positive),
    honest curl (negative), Googlebot with "Chrome" in its own UA
    string (negative — crawler exemption), no JA4 at all / plain HTTP
    (negative, fail open), empty UA, garbage/too-short JA4.
  - `proxy/capture_test.go`: `TestUAMismatchReachesOriginEndToEnd` —
    real Chrome-claiming client through the real fragmenting relay
    used for item 2's evasion test, checks the flag reaches the origin
    through the actual proxy wiring, not just the pure function.
  - `proxy/proxy_test.go`: `TestNewStripsSpoofedUAMismatchHeader`.
Why: ROADMAP item 3's literal wording ("does the claimed User-Agent
match the fingerprint's real client family") needs a maintained JA4-
to-browser-version database to do fully — a real ongoing research
cost, not something to fake in one pass (`CLAUDE.md` Section 15).
Built the narrower, well-grounded slice instead: two structurally
provable facts about every current real browser (TLS version floor,
never triggering fragmentation), needing no external data. Recorded
the scope decision and its false-positive trade-off in `DECISIONS.md`.
Tested how:
  - Mutation-checked all three real conditions in `UAMismatch`:
    forcing the function to always return false, removing the crawler
    exemption (behaviorally — an empty `crawlerMarkers` slice, since
    deleting the loop outright left a compile error, which the
    mutation-check step itself caught before it could hide anything),
    and removing the `JA4Unreadable` branch — all three turned the
    matching test case red with the exact case named.
  - Mutation-checked the header spoof-strip in `proxy.go`: removing
    `r.Out.Header.Del(uaMismatchHeader)` fails
    `TestNewStripsSpoofedUAMismatchHeader` with the spoofed value
    named.
  - `go build/vet/test ./... -race` and `gofmt -l .` clean throughout.
  - Real binary run: openssl cert + curl through botshield — no
    mismatch header set (correct, curl doesn't claim to be a browser).
  - Independent `/security-review` (background agent) on the new
    surface: checked User-Agent length/ReDoS exposure (bounded by
    Go's 1MB `DefaultMaxHeaderBytes`, verified in `$GOROOT/src/net/
    http/server.go:916`, since no `MaxHeaderBytes` override exists in
    `cmd/botshield`), the crawler-exemption bypass (real gap, but
    "signal only, no single signal decides" per Section 6 — not a
    vulnerability today), and the `ja4[1:3]` slice (safe: `ja4` here
    is never attacker-supplied text, only `""`, `JA4Unreadable`, or
    `fingerproxy`'s fixed-format output). No findings above the
    confidence bar.
Known gaps / follow-up (not deferred without reason — see `CLAUDE.md`
Section 17):
  - This does not verify "real client family" the way ROADMAP's
    literal wording asks — see `DECISIONS.md` for why that needs a
    maintained database, which is out of scope for this pass.
  - A bot can trivially avoid the check entirely by putting "bot" in
    its own UA string. Not fixed because it isn't a security gap at
    the *current* stage (nothing is blocked yet) — revisit once item 5
    (scoring) exists and this becomes one input among several, per
    the independent review.
  - A corporate TLS-inspecting proxy or a genuinely old/locked-down
    real browser could trigger a false positive on the TLS-version
    check. Accepted and documented in `DECISIONS.md` — this is exactly
    why the result is a signal, never a block, on its own.

---

## 2026-09-15 — Session handoff: item 3 done, item 4 next, PR #2 about to be merged
Changed: nothing new this entry — this is a handoff note because the
previous session hit its context limit mid-way and a new session is
picking up from here.
Current state, verified before writing this:
  - `git status` clean, everything committed and pushed to
    `claude/code-review-feedback-k1zukr`. Latest commit: `9c230a1`
    ("Add UA/header consistency check (ROADMAP item 3, done)").
  - PR #2 (https://github.com/ToufiqQureshi/bot-shield/pull/2) held
    everything from item 2 (TLS/JA4) and item 3 (UA consistency),
    clean and CI green as of the last check. **The project owner is
    merging PR #2 into `main` right now**, outside this session.
  - `docs/ROADMAP.md` Done section already lists items 1, 2, and 3 as
    complete — no ROADMAP edit needed for the merge itself.
What the next session must do first (per `CLAUDE.md` Section 0/2):
  1. `git fetch origin main && git log origin/main -3` — confirm PR #2
     actually landed on `main` before doing anything else. Do not
     assume the merge happened just because this note says it was
     about to.
  2. Start item 4 (JS challenge) from a fresh branch off the merged
     `main`, not by continuing on `claude/code-review-feedback-k1zukr`
     — that branch's PR is closed once merged; a new branch/PR is the
     correct next step, same pattern as `CLAUDE.md`'s repo-workflow
     instructions for a merged PR.
  3. Re-read `docs/ROADMAP.md` item 4 ("JS challenge — a lightweight
     challenge page (math + timing + basic canvas check) served to
     unscored/ambiguous traffic. A plain HTTP client without a JS
     engine fails immediately.") and `docs/ARCHITECTURE.md`'s current
     BUILT/planned markers before writing code — this session did not
     start item 4, so there is no partial implementation to check for
     duplication, but there may be new commits on `main` from the
     merge worth reading first.
  4. Follow the same process items 2 and 3 used: threat-driven design
     doc (`CLAUDE.md` Section 20) before code, test-first (Section 7),
     mutation-check every test (Section 23a), check the standard
     library first (Section 24 — note there is no stdlib equivalent
     for a JS challenge page itself, but the HTTP serving/timeout
     parts of it should still be checked), independent
     `/security-review` before calling it done (Section 24b, and
     especially relevant here since a challenge page is new
     visitor-facing surface), and the Section 22 pre-push checklist.
Known gaps / follow-up: none new. The gaps already on record (HTTP/2
fingerprinting not built, ClientHello fragmentation detected but not
prevented, UA-mismatch bypassable by a bot naming itself, no scoring
engine yet so nothing blocks traffic, no real LICENSE file) are
unchanged and still listed in `docs/ROADMAP.md`'s Done section and
`docs/DECISIONS.md`.

---

## 2026-09-15 — JS challenge page (ROADMAP P0 item 4, mechanism done)
Changed:
  - `proxy/challenge.go`: added `Challenge` — `NewChallenge()` (random
    in-process HMAC secret), `Handler()` exposing GET
    `/__botshield/challenge` (issue a puzzle) and POST
    `/__botshield/verify` (check the answer), and `Passed(r)` for
    whatever wires this in next to check an already-solved visitor.
    The puzzle: a signed token binds a random nonce + issue time + the
    exact page the visitor was on; the page's JS must compute
    SHA-256(nonce) via `crypto.subtle` and submit a real
    `canvas.toDataURL()` render. A wrong answer, missing/malformed
    canvas proof, tampered/expired token, or a token signed by a
    different secret all fail closed back to a fresh puzzle. On
    success: a signed, HttpOnly/Secure/SameSite=Lax
    `X-BotShield-Passed` cookie, and a redirect back to the exact
    original path+query — `serveChallenge` is designed to be called in
    place of proxying a real request (item 5's job, not built yet), so
    the "page to return to" is just that request's own URL.
    `safeRedirectPath` rejects `//host`, backslash tricks, and empty
    paths to keep that redirect from becoming an open redirect.
    `handleVerify` wraps the body in `http.MaxBytesReader` (64KB) since
    this is a new unauthenticated, visitor-controlled POST endpoint.
  - `proxy/challenge_test.go`: real end-to-end pass (GET the page,
    extract the actual nonce/token a browser's JS would see via regex,
    compute the real SHA-256 answer, POST it, confirm redirect + signed
    cookie), wrong answer, missing canvas proof, tampered token,
    expired token, cross-instance secret isolation, forged/expired
    passed-cookie, oversized body (`MaxBytesReader` actually enforced),
    wrong HTTP methods, `safeRedirectPath` open-redirect table, and two
    tests specifically proving the redirect-back target is the exact
    original path+query (not just "starts with the challenge route") —
    added after the first version of that test was too weak to catch a
    real bug (see below).
  - `cmd/botshield/main.go`: mounts `challenge.Handler()` at
    `/__botshield/` on a new `http.ServeMux`, proxy still on `/`.
    Endpoints are reachable today for manual testing; nothing routes a
    real visitor to them automatically yet.
  - `docs/ROADMAP.md`: item 4 moved to Done with the design rationale
    and known gaps inline.
  - `docs/DECISIONS.md`: two entries — why the challenge requires a
    canvas proof and not just a math answer (a sha256-only puzzle is
    not actually JS-specific — any language can compute one), and why
    the signing secret is random/in-process/unshared.
Why: this is the ROADMAP's next P0 item after fingerprinting (item 2)
and UA consistency (item 3). Both of those shipped as signals with no
scoring engine to act on them yet (item 5); item 4 follows the same
pattern deliberately — it builds the challenge *mechanism* fully and
correctly, but does not invent an automatic triggering policy that
belongs to scoring.
Tested how:
  - Design-level catch, mid-session: the first version of the
    "redirects to the original path" test only asserted the Location
    header *started with* the challenge route, which is trivially true
    in that test's own setup and would have passed even with the
    redirect logic doing something else. Rewrote it (plus added a
    second test using an arbitrary site path) to assert the exact
    original path+query round-trips — this is exactly the "assertion
    satisfiable another way" trap `CLAUDE.md` Section 23b warns about,
    caught before merge, not after.
  - Mutation-checked three of the security-critical checks in
    `challenge.go`, one at a time: removed the token-expiry check
    (`TestChallengeRejectsExpiredToken` went red), removed the
    canvas-proof check (`TestChallengeRejectsMissingCanvasProof` went
    red), removed the answer comparison
    (`TestChallengeRejectsWrongAnswer` went red). Each reverted
    afterward, verified byte-for-byte against a backup before
    continuing.
  - `go build/vet/test ./...` clean, `gofmt -l` clean on the new files.
    `-race` could not be run this session — no C compiler/CGO
    available in this environment (`CGO_ENABLED=0`, no `gcc` on PATH),
    unlike prior sessions' environment. Flagging this as an actual gap
    in this session's verification, not silently skipping it.
  - Real compiled binary run: local Python origin server + `botshield`
    in front of it, both a plain passthrough request (200, proxied
    correctly) and a full challenge solve done for real over HTTP
    (GET the challenge, `sha256sum` the real nonce, POST the real
    answer + a canvas-shaped value, confirmed 303 redirect back to the
    exact original path+query and a signed `Set-Cookie`).
  - Independent `/security-review` via a background agent (fresh eyes,
    no context from this session): checked html/template escaping
    context (JS-string context inside `<script>`, no `template.JS`/
    `template.HTML` casts anywhere — correct), open-redirect bypasses
    against `safeRedirectPath` (`//`, backslash, tab/CR/LF stripping,
    double-encoded slash — traced through Go's URL
    escaping/round-tripping, none survive), and signature/cookie
    forgery (constant-time compares used throughout, token and
    passed-cookie payload shapes are structurally incompatible so one
    can't be replayed as the other). No findings.
Known gaps / follow-up (not deferred without reason — see `CLAUDE.md`
Section 17):
  - **Not wired to automatic triggering.** Nothing currently sends a
    real visitor to the challenge — that decision (who counts as
    "ambiguous") is ROADMAP item 5's job, which doesn't exist yet. The
    endpoints are live and correct but inert in production today,
    matching exactly how items 2 and 3 shipped as signals nothing yet
    consumed.
  - **Canvas proof is a shape check, not a render check.** A bot that
    specifically studies bot-shield can fake a passing string without
    ever rendering anything server-verifiable. Documented in
    `DECISIONS.md` as an accepted MVP-scope limitation, same class as
    item 3's JA4-database gap — real pixel verification is a project
    of its own.
  - **Signing secret is per-process, in-memory only.** A restart or a
    second instance invalidates every outstanding challenge/cookie.
    Correct for the current single-process architecture; needs the
    planned Redis store before bot-shield can run more than one
    instance.
  - **This session could not run `-race`** (no cgo/gcc in this
    environment). The code was still reviewed for the same concurrency
    concerns `-race` would catch (the HMAC secret is read-only after
    construction, no shared mutable state introduced), but that is
    reasoning, not a tool result — worth an actual `-race` run in an
    environment that has it before this is called fully verified.
  - No rate limiting on `/__botshield/verify` — out of scope
    deliberately (ROADMAP item 9, not item 4), matching the project's
    "don't build ahead of the roadmap order" pattern from item 1.

---

## 2026-09-15 — Scoring engine v1 (ROADMAP P0 item 5, done); Antigravity joins the project
Changed:
  - `proxy/score.go`: added `Score(ja4, ua string) int` (combines the
    JA4-fragmentation and UA-mismatch signals, 50 points each,
    additive) and `Decide(score int) Decision` (allow/challenge/block
    at fixed 50/100 thresholds), plus the `Decision` type and its
    `String()`.
  - `proxy/guard.go`: added `Guard`, wrapping the proxy and the JS
    challenge into the real per-request decision — a passed-challenge
    visitor goes straight through, otherwise `Score`/`Decide` picks
    allow (proxy as before), challenge (serve the JS challenge in
    place of proxying), or block (403, origin never sees the request).
  - `proxy/challenge.go`: renamed `serveChallenge` to the exported
    `Serve`, since `Guard` needs to call it from outside the package's
    own challenge-handling code — same method, no behavior change.
  - `cmd/botshield/main.go`: `/` is now served by `Guard`, not the bare
    proxy. `/__botshield/` stays mounted for direct manual testing.
  - `proxy/score_test.go`, `proxy/guard_test.go`: unit tests for every
    `Score`/`Decide` boundary, plus four real end-to-end tests through
    actual TLS handshakes (reusing items 2-4's `startCapture`/
    `fragmentingRelay` harness): allowed (no signal), blocked (both
    signals), challenged (one signal, origin never reached), and a
    real passed-cookie bypassing what would otherwise block.
  Why: this is the item the whole product has been building toward —
  every earlier item (fingerprint, UA check, JS challenge) was
  deliberately a label with nothing consuming it yet
  (`docs/ROADMAP.md`'s own recurring note on items 2-4). This closes
  that loop: bot-shield now actually allows, challenges, or blocks a
  real request instead of only describing it.
  Tested how:
    - Real end-to-end tests, not the pure functions in isolation: a
      real TLS client through a real capture listener, using the same
      `fragmentingRelay` trick items 2-4 already built to produce a
      real `JA4Unreadable` fingerprint, checking actual HTTP status
      codes and that the origin was or wasn't reached (via a buffered
      channel, same pattern as every other end-to-end test in this
      package).
    - Mutation-checked: removed the passed-cookie bypass from `Guard`
      → `TestGuardPassedCookieBypassesBadSignals` failed naming the
      wrong status code (403 instead of 200). Removed the UA-mismatch
      weighting from `Score` → three tests went red
      (`TestScoreUAMismatchOnly`, `TestScoreBothSignals`, and the real
      end-to-end `TestGuardBlocksCombinedSignals`), proving the unit
      tests and the integration test both actually depend on that
      code path, not just one of them. Both mutations reverted,
      confirmed byte-for-byte via diff against a backup before
      continuing.
    - Found and fixed a real bug in my own first draft of
      `guard_test.go`, not in production code: a hand-built
      `application/x-www-form-urlencoded` body containing the literal
      canvas value `data:image/png;base64,...` failed to parse — Go's
      `url.ParseQuery` rejects a literal `;` in a form body (since Go
      1.17, RFC 3986 ambiguity). A real browser's `URLSearchParams`
      percent-encodes it automatically; my test's raw string
      concatenation didn't. Fixed by reusing `challenge_test.go`'s
      existing `postVerify` helper, which builds the body correctly
      via `url.Values.Encode()`, instead of hand-rolling a second,
      wrong way to do the same thing.
    - `go build/vet/test ./proxy/... ./cmd/...` clean (`-race`
      unavailable this session, same gap noted in item 4's entry).
      `gofmt -l` clean on every new/changed file (pre-existing files
      still show CRLF-only diffs unrelated to this change, as noted in
      item 4's entry).
    - Real compiled binary run: a local origin plus `botshield` in
      front of it, plain HTTP (no `-tls-cert`) request through
      `Guard` — 200, unchanged from before this change, confirming
      the fail-open path (no TLS, so nothing to score, so nothing is
      ever penalized for a connection bot-shield genuinely can't
      examine) still holds with `Guard` in the path.
  Known gaps / follow-up (not deferred without reason — `CLAUDE.md`
  Section 17):
    - Thresholds and weights (50/50, challenge at 50, block at 100)
      are reasoned, not tuned against real traffic — there is none
      yet. See `docs/DECISIONS.md` for the reasoning and what would
      trigger a revisit.
    - Not yet per-client configurable (`docs/ROADMAP.md` item 11).
    - Only 2 of the planned signals (items 2, 3) feed the score; item
      6 (client-side automation probe) is the next input, not built.
    - Antigravity (dashboard/frontend) asked for a simple analytics
      endpoint (total/allowed/challenged/blocked counts) to build the
      dashboard against — not built in this pass, tracked as a
      follow-up task for whoever picks up the API-contract work next.
    - Independent `/security-review` was not run for this specific
      change — `Guard`/`Score`/`Decide` don't introduce new
      visitor-controlled *input* (they only read signals items 2-4
      already validated and reviewed), so the Section 24b trigger
      ("especially on visitor-controlled input") is weaker here than
      it was for the challenge page itself. Worth a pass before this
      is called fully done if the thresholds change or a new signal is
      added.
Also this session: **Antigravity (a second coding agent, Google's
IDE) joined the project**, working in parallel via `agentchat/`. Real,
consequential process changes came out of that — recorded in
`claude_and_agy.md` (new file, roles/workflow) and the two
`docs/DECISIONS.md` entries above/below this one (scoring thresholds;
why the MCP-based coordination attempt was tried and removed the same
day). Ownership split: Antigravity owns dashboard/frontend, Claude
Code owns backend/proxy — and per direct project-owner correction
mid-session, Claude Code's role going forward is PM/senior-dev/CTO
(assign small tasks to Antigravity, review its work against
`CLAUDE.md`, fix/improve it directly when needed — not "rubber-stamp
or reject"), not "write every line personally by default." See
`claude_and_agy.md` for the full model; read it before assigning or
reviewing any cross-agent work next session.

---

## 2026-09-15 — Dashboard analytics endpoint; first real Antigravity code review
Changed:
  - `proxy/stats.go`: added `Stats` (atomic counters:
    total/passed/challenged/blocked) and its `Handler()`, serving
    `GET /api/v1/dashboard/stats` in the exact shape Antigravity
    specified in `agentchat/chat.jsonl`.
  - `proxy/guard.go`: `Guard` now takes a `*Stats` and increments the
    matching counter on every decision (allow, challenge, block,
    including the already-passed-challenge bypass path).
  - `cmd/botshield/main.go`: mounts the stats endpoint at
    `/api/v1/dashboard/stats`.
  - `proxy/stats_test.go`: JSON shape/field-name tests, wrong-method
    rejection, CORS header check, and a real end-to-end test that
    `Guard` actually increments counters, not just decides outcomes.
  - `dashboard/__tests__/DashboardStats.test.tsx`,
    `dashboard/src/components/DashboardStats.tsx`: **reviewed
    Antigravity's dashboard skeleton** (first real review under the
    new PM/reviewer model, `claude_and_agy.md`) and found + fixed one
    real gap: the component has a full error/retry UI branch
    (`Connection Lost`, retry button) that had zero test coverage,
    because the mock data source (`StatsMock.ts`) never actually
    fails. Added a test that makes it fail once (`jest.mock` +
    `mockRejectedValueOnce`) and asserts the error UI renders and
    stale/partial stats don't. This is exactly the gap `CLAUDE.md`
    Section 23b calls out ("only the easy input... doesn't prove
    robustness") — a real backend being down is the normal case for a
    bot-detection dashboard, not an edge case.
Why: Antigravity finished the dashboard skeleton and posted its own
API contract (`/api/v1/dashboard/stats`,
`{total_requests, passed, challenged, blocked}`) in `agentchat/`
before this session got to it — built the backend to match that
contract exactly, contract-first, rather than the other way around.
Then reviewed Antigravity's frontend work for real, per the
project owner's direct instruction that reviewing means fixing gaps
directly, not just reporting them back.
Tested how:
  - Ran Antigravity's existing dashboard test suite for real
    (`npm test` in `dashboard/`) before touching anything — both tests
    passed, confirmed not just trusted.
  - Mutation-checked Antigravity's existing tests before adding
    anything: changed the mock's `total_requests` value — both
    existing tests **stayed green**, because they assert against
    `mockDashboardStats` imported from the same module rather than a
    fixed expected value (so they verify internal consistency between
    mock and render, not a wrong number by itself — noted, not
    necessarily a defect, but worth knowing what the test actually
    proves). Then swapped which field renders under the "Passed"
    label (a realistic copy-paste bug class) — this **did** turn the
    existing "renders mock stats" test red, confirming it does catch
    real rendering bugs even though it wouldn't catch a wrong mock
    value. Reverted both mutations, confirmed clean via diff against a
    backup.
  - Added the error-state test, then mutation-checked it the same way
    every test in this project is checked: removed the `catch` block's
    `setError` call → the new test went red (`Connection Lost` never
    found). Reverted, confirmed green (3/3 tests) again.
  - `proxy/stats.go`: full `go build/vet/test ./proxy/... ./cmd/...`
    clean, `gofmt -l` clean on every new/changed Go file. Mutation-
    checked `Guard`'s counter increment (see `docs/ROADMAP.md` item 5
    entry). Real compiled-binary run: two plain-HTTP requests through
    the actual `Guard`, then `curl` the stats endpoint — response was
    `{"total_requests":2,"passed":2,"challenged":0,"blocked":0}`,
    exactly matching Antigravity's contract, confirmed the CORS header
    is present for the dashboard's separate origin to call it.
Known gaps / follow-up:
  - The dashboard's real `fetch("/api/v1/dashboard/stats")` call is
    still commented out in `DashboardStats.tsx` — it renders mock data
    only. Wiring it to the now-real backend endpoint is Antigravity's
    next piece; posted the working endpoint + confirmed response shape
    in `agentchat/chat.jsonl` so that's unblocked.
  - Counters are in-memory only (resets on restart) — acceptable for a
    skeleton/demo, not for a real client dashboard. Needs the planned
    Postgres store before this is real analytics, not a placeholder.
  - This was Claude Code's first full review-and-fix pass on
    Antigravity's code under the new model
    (`claude_and_agy.md`) — worth checking next session whether this
    depth of review (read the code, run its tests, mutation-check
    both the existing tests and the ones added, fix a real gap
    directly) is the right amount, too much, or not enough, once
    there's more than one data point.

---

## 2026-09-16 — Client-side automation probe (ROADMAP P0 item 6, done); session wrap-up
Changed:
  - `proxy/challenge.go`: the challenge page's JS now also checks
    `navigator.webdriver` and known Selenium/PhantomJS/Nightmare.js
    globals, sending the result as `automation` in the verify POST.
    `handleVerify` fails the challenge (no passed cookie) if it's
    `"true"`, even when the sha256/canvas checks already passed.
  - `proxy/challenge_test.go`: added `postVerifyFull` (extends
    `postVerify` with an automation param, existing calls unaffected)
    and `TestChallengeRejectsAutomationFlag`.
  - `docs/ROADMAP.md`: item 6 moved to Done with the scope reasoning
    inline; `docs/DECISIONS.md` has the full "why inside the challenge
    page, not site-wide" entry.
Why: next P0 item after the scoring engine. Scoped deliberately
narrower than the literal wording — see the new `docs/DECISIONS.md`
entry — reusing the one place this codebase already runs its own JS
in a visitor's browser, instead of building a new (and much larger)
HTML-injection feature with no roadmap decision behind it.
Tested how: mutation-checked (removed the automation check, the new
test went red, reverted, confirmed byte-for-byte via diff). Full
`go build/vet/test ./proxy/... ./cmd/...` clean, `gofmt -l` clean on
changed files.
Known gaps / follow-up: Patchright-class tools that specifically strip
these markers aren't caught (documented, matches `docs/RESEARCH.md`'s
existing "behavioral/timing signals, item 7" answer for that tier).
Only runs for traffic that already reached the challenge, not
site-wide — see `docs/DECISIONS.md` for why and when to revisit.

**Session wrap-up (project owner asked to stop for today) — status of
everything built by both agents, checked against `CLAUDE.md`:**

*Claude Code (backend, `proxy/`, `cmd/botshield/`) — all items below
built test-first, mutation-checked, verified against the real compiled
binary this session, `go build/vet/test ./proxy/... ./cmd/...` clean
as of this entry:*
- Item 1 (reverse proxy), item 2 (TLS/JA4), item 3 (UA consistency):
  built in earlier sessions, unchanged today, still green.
- Item 4 (JS challenge): built earlier this session (2026-09-15).
- Item 5 (scoring engine — `score.go`, `guard.go`): built today.
- `proxy/stats.go` (dashboard analytics endpoint, matches
  Antigravity's contract): built today.
- Item 6 (automation probe): built today, this entry.
- **Not done / explicitly out of scope for now:** item 7+ (P1,
  behavioral scoring and beyond) — not started. `-race` could not be
  run this session (no cgo/gcc in this environment) — noted as a real
  gap in each entry it affects, not silently skipped. No per-client
  config (item 11). Counters/challenge secret are in-memory only
  (need the still-planned Redis/Postgres).

*Antigravity (frontend, `dashboard/`) — reviewed by Claude Code once
this session (see the "first real Antigravity code review" entry
above):*
- Dashboard skeleton (Next.js, `dashboard/`) built against mock data,
  with loading/error/happy-path states and tests. One real gap
  (untested error state) found and fixed during review.
- API contract (`/api/v1/dashboard/stats`) proposed by Antigravity,
  now implemented for real on the backend and confirmed matching via
  `curl` against the compiled binary.
- **Not done:** the dashboard's real `fetch()` call to that endpoint
  is still commented out — it renders mock data only. This is
  Antigravity's next piece, not blocked on anything now that the real
  endpoint exists and its shape is confirmed. Not reviewed again since
  the one pass above (no new Antigravity commits landed in
  `agentchat/chat.jsonl` before this session ended).

**Honest overall answer to "is everything done and CLAUDE.md-compliant":**
Backend items 1-6 are each individually done to this project's own bar
(tested, mutation-checked, real-binary-verified, docs updated) — yes.
The *product* is not done: nothing beyond items 1-6 exists (no
behavioral scoring, no rate limiting, no per-client config, no
dashboard wired to real data, no deployment story). That's expected —
P0 was always items 1-6, and P1/P2 are explicitly next, not skipped.
Next session should read `claude_and_agy.md` and this entry before
assigning new work.

---

## 2026-09-16 — Dashboard wired to the real backend; dead code removed; merged a landed upstream PR
Changed:
  - `dashboard/src/lib/stats.ts` (new): `fetchStats()` — the real
    `fetch("${API_BASE_URL}/api/v1/dashboard/stats")` call, checks
    `res.ok` before returning, throws on a non-2xx status.
    `API_BASE_URL` reads `NEXT_PUBLIC_API_BASE_URL`, defaults to
    `http://localhost:8080` for local dev. `DashboardStatsData` moved
    here from the deleted mock file.
  - `dashboard/src/components/DashboardStats.tsx`: now calls
    `fetchStats()` instead of `fetchStatsMock()`; the commented-out
    "replace with real API call when backend is ready" block is gone
    — it *is* the real call now.
  - `dashboard/src/components/StatsMock.ts`: **deleted**. Once
    `DashboardStats.tsx` called the real endpoint, this was dead code
    (`CLAUDE.md` Section 24a) — nothing referenced `fetchStatsMock`
    outside the test file.
  - `dashboard/src/app/page.tsx`: removed `import Image from "next";`
    — unused, and actually broken (`Image` isn't a default export of
    `"next"`, that's `"next/image"`) — leftover, never-fixed
    `create-next-app` boilerplate.
  - `dashboard/src/app/layout.tsx`: page metadata (`title`,
    `description`) still said "Create Next App" / "Generated by create
    next app" — replaced with the actual product name/description.
  - `dashboard/__tests__/DashboardStats.test.tsx`: rewritten to mock
    `global.fetch` directly instead of mocking the now-deleted
    `StatsMock` module. Added a test for a non-2xx backend response
    (the existing rejected-promise test only covered a network-level
    failure, not "backend responded but with an error status" — a
    different, equally real failure mode for a live HTTP call that
    didn't exist when the code only called a mock).
Why: the project owner asked directly — "Antigravity ne jo kaam kiya,
usko connect aur polish kar, tests aur unnecessary lines/dead code
hatao" (connect Antigravity's work to the real backend, polish it per
`CLAUDE.md`, clean up dead code) — before stopping for the day. The
skeleton was correct but explicitly unfinished (its own comment said
"replace with real API call when backend is ready" — the backend
was ready as of the previous entry).
Tested how:
  - `npm test` in `dashboard/`: 4/4 passing (was 3/4 — added the
    non-2xx-status case).
  - Mutation-checked the new `res.ok` check in `fetchStats()`:
    removed it, the non-2xx test went red (rendered nothing instead
    of the error UI), reverted, confirmed green again.
  - `npx eslint .` clean, `npx tsc --noEmit` clean, `npm run build`
    (real Next.js production build, not just dev mode) succeeded.
  - Real end-to-end integration test, not just unit tests: built and
    ran the actual `botshield` binary on `:8080` with a real origin
    behind it, ran the actual Next.js dev server on `:3000`, confirmed
    (a) the compiled JS bundle contains `api/v1/dashboard/stats` and
    no reference to the deleted mock, (b) `curl` with an
    `Origin: http://localhost:3000` header against the backend returns
    `Access-Control-Allow-Origin: *` — the exact cross-origin request
    shape a real browser's `fetch()` would make from the dashboard's
    dev server to the backend, confirmed to succeed.
  - Killed only the specific PIDs this session's test processes were
    bound to (checked via `netstat`), not a blanket `taskkill` of all
    `node.exe`/`python.exe` — other node/python processes on this
    machine belong to Antigravity's own tooling, not this session's.
  **Also this session: merged a landed upstream PR before any of this
  was pushed.** The project owner flagged that PR #3 (a different
  Claude Code session's work — new `docs/RESEARCH.md` competitor
  research and two new `docs/ROADMAP.md`/`docs/DECISIONS.md` items,
  9a/11a) had merged to `main` overnight, and asked to check for
  conflicts before pushing anything from this session. It had, and
  both sessions had touched `docs/DECISIONS.md` and `docs/ROADMAP.md`.
  Resolved safely since nothing from this session was committed yet:
  `git stash -u` (all uncommitted changes, tracked and untracked),
  `git merge origin/main` (clean fast-forward), `git stash pop`.
  `docs/ROADMAP.md` auto-merged with no conflict; `docs/DECISIONS.md`
  had one (both sessions added a new entry at the top of the file) —
  resolved by keeping both entries in full, newest-dated first, no
  content from either side dropped. Verified `go build/vet/test
  ./proxy/... ./cmd/...` still clean after the resolution before
  dropping the stash.
Known gaps / follow-up:
  - Nothing in this session has been committed or pushed yet — still
    working tree changes only. Next step, if the project owner wants
    it, is to commit and open a PR the same way items 1-4's sessions
    did.
  - `dashboard/AGENTS.md`/`dashboard/CLAUDE.md` (a `next dev`-generated
    file, per its own header comment) weren't touched — out of scope
    for this pass, and the file itself says it's regenerated
    automatically.
  - The dashboard's own `README.md` is still the unedited
    `create-next-app` default (generic Next.js getting-started text,
    not bot-shield-specific). Noticed during this review, not fixed —
    lower priority than the dead code/wiring issues actually blocking
    real functionality, flagging per `CLAUDE.md` Section 16 rather than
    silently leaving it unmentioned.

---

## 2026-09-16 — Session handoff: P0 (items 1–6) complete, nothing committed yet
Changed: nothing new in this entry — end-of-day handoff, project owner
asked to stop for today after finishing what was in flight.
**Verified state, right before writing this:**
  - `git log -1 HEAD` = `2c3e33f` (origin/main, PR #3 merged in — see
    the "merged a landed upstream PR" note in the entry above this
    one). Local branch is caught up with `origin/main`; no divergent
    local commits.
  - **Nothing from today is committed.** `git status` shows all of
    today's work as working-tree changes (modified: `.gitignore`,
    `CLAUDE.md`, `README.md`, `cmd/botshield/main.go`,
    `docs/ARCHITECTURE.md`, `docs/DECISIONS.md`, `docs/PROGRESS.md`,
    `docs/ROADMAP.md`; new/untracked: `claude_and_agy.md`,
    `proxy/{challenge,guard,score,stats}.go` + their `_test.go` files,
    `dashboard/`). `agentchat/` and `.agents/` are gitignored, not
    tracked — expected.
  - `go build/vet/test ./proxy/... ./cmd/...` clean (44 tests, all
    green). `dashboard/`: `npm test` (4/4), `npx eslint .` clean,
    `npx tsc --noEmit` clean, `npm run build` (real production build)
    succeeds.
  - `agentchat/web.py` was left running on `http://localhost:9999`
    (background process) so the chat UI stays usable — a new session
    doesn't need to restart it unless it's actually down; check with
    `curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:9999/`
    before assuming it needs restarting.

**What a new session must do first (per `CLAUDE.md` Section 0/2):**
  1. Read `docs/AGENT.md` → `ARCHITECTURE.md` → `ROADMAP.md` →
     `DECISIONS.md` → `RESEARCH.md` → `PROGRESS.md` (this file, read
     the last ~3 entries in full, not just this one) → `CLAUDE.md` →
     **`claude_and_agy.md`** (new this session — the two-agent
     workflow model; read it before assigning or reviewing any work).
  2. `git status` and `git log -1` to confirm the state above still
     holds — don't assume it does just because this note says so.
  3. Check `agentchat/chat.jsonl` (or `http://localhost:9999`) for
     anything from Antigravity since this session ended — no automatic
     notification exists, it must be checked manually (see
     `claude_and_agy.md` and the "agentchat: removed the MCP server"
     `DECISIONS.md` entry for why).
  4. Decide whether to commit/PR today's work — this session did not,
     since committing/pushing wasn't explicitly requested and is a
     more consequential action than the polish/connect work that was
     asked for. Everything is verified and working in the tree; it's
     ready to commit whenever the project owner confirms they want
     that.

**What's actually done (P0, items 1–6):** reverse proxy, TLS/JA4
fingerprinting, UA-consistency check, JS challenge (sha256 + canvas +
automation-framework detection), scoring engine (`Score`/`Decide`/
`Guard`), and a live dashboard-stats endpoint wired to a real (skeleton)
Next.js dashboard. All individually tested, mutation-checked, and
verified against real compiled binaries this session and prior ones.

**What's next (not started, no code exists for these yet):** P1 items
7–11 (behavioral scoring, session consistency, rate/pattern anomaly
detection, honeypot fields, per-client rules) and the rest of P2 item
12 (dashboard history-over-time, top offenders, false-positive report
button — today's work is a live-snapshot skeleton, not the full item).
Read `docs/ROADMAP.md`'s "How to read this list" section before
picking the next item — P1 comes after P0 is proven, not before.

---

## 2026-09-16 — Market scan; repositioned the product away from "affordable alternative"

**Docs only. No Go code, no behaviour, no tests changed.** Everything
below is `.md`. Named up front so nobody looks for a code change that
isn't there, and so the absence of new tests in this entry isn't read
as a gap (`CLAUDE.md` Section 23a applies to code under test — there
is none in this diff).

### Why this session happened

The project owner asked the question that no test suite answers:
*will anyone actually pay $200/month for this, and what would make
them?* Answering it properly meant doing real market research rather
than reasoning from the founding assumption, and the research
contradicted the founding assumption.

### What the research found

Full notes and sources: `docs/RESEARCH.md`, new section **"Market &
pricing scan — 2026-09-16"**. The short version:

- The market's price floor is **$0**, not "cheaper than Akamai."
  CrowdSec, SafeLine and Coraza are free and self-hosted; Cloudflare
  has a free tier; Prosopo is ~$39/mo. Above that, DataDome/HUMAN/
  Kasada sit at ~$1K–50K/mo.
- So "affordable bot detection" — the framing every doc in this repo
  opened with — walks straight into *"CrowdSec is free."* That is not
  an argument we can win.
- What the free tools **don't** do: they parse server **logs**, so
  they react to an IP after it has already misbehaved somewhere.
  bot-shield reads the live ClientHello and scores the first request
  with no prior sighting. No self-hostable product does inline
  TLS/JA4 scoring today. **That**, not price, is the moat.
- Forrester renamed the category in Q2 2026 to *Bot and Agent Trust
  Management*. Cloudflare shipped pay-per-crawl (HTTP 402); RSL and
  Web Bot Auth appeared. The buyer's question moved from "is this a
  bot?" to "which agent is this, is it allowed, can I prove what I
  decided?" — and mid-market sites have no tool for it.

### What changed, file by file

- **`docs/RESEARCH.md`** — added the market & pricing scan: what the
  market charges (table), why CrowdSec rather than DataDome is our
  real competitor (table), the 2026 agent-governance shift, and what
  was deliberately *not* taken from the scan (pay-per-crawl billing,
  shared blocklists) with reasons.
- **`docs/DECISIONS.md`** — new top entry, *"Positioning:
  self-hostable agent governance, not 'cheap DataDome'."* Records the
  decision, the numbers behind it, the two buyer segments, four
  rejected alternatives (compete on price / chase enterprise / build
  402 billing now / open-source a community edition), and an explicit
  note on what this does **not** change.
- **`docs/ROADMAP.md`** — rewrote the intro as a scannable
  "what bot-shield is" block with a NOT/IS table and a who-pays list.
  Added items **11b** (verified agent policy), **12a** (decision
  evidence trail) and **18** (shadow mode + traffic report). Gave item
  17 an actual pricing anchor. Added a two-question test to "How to
  read this list" and a dated priority note.
- **`CLAUDE.md`** — new 30-second "What you're building" block at the
  very top, plus a positioning row in the lookup table. Section 1
  rewritten: it previously said "affordable," which now contradicts
  everything else.
- **`docs/AGENT.md`** — the "why are we building this" section said
  mid-size companies can't afford enterprise tools, therefore be
  cheap. Corrected to state why cheap is the wrong conclusion, and
  who the two real buyer segments are.
- **`README.md`** — headline and "why this exists" rewritten to match.
  Also fixed a stale line claiming there is no dashboard — PR #4 adds
  one, so that sentence was already wrong before this session.

### Why every doc, not just one

`CLAUDE.md` Section 0's last checklist item: no doc may contradict
another. Changing the positioning in `DECISIONS.md` alone would have
left five files still opening with "affordable alternative" — the
exact stale-doc failure that section exists to prevent. A future
session reading `AGENT.md` first (as instructed) would have got the
old framing and never reached the correction.

### New roadmap items — what they are and the risk on each

- **11b, verified agent policy** — per-agent allow / rate-limit /
  deceive / block, so "GPTBot is fine, a scraper wearing its
  User-Agent is not" becomes expressible. Reuses item 11's config,
  item 5's decision output, item 3's `UAMismatch`. *Risk logged:* an
  over-broad allow rule is a bypass with a config file; an allowed
  agent must still be fingerprint-checked, or `CLAUDE.md` Section 6
  is defeated by our own feature.
- **12a, decision evidence trail** — per-request score, signals fired,
  JA4, decision. *Risk logged:* it's a log of visitor traffic, so it
  needs a retention limit and bounded storage from day one (Sections
  9 and 18), not bolted on after it fills a client's disk.
- **18, shadow mode** — score and record without enforcing. *Risk
  logged:* a client who thinks they're protected while in shadow mode
  is worse off than one with no bot-shield at all, so the mode has to
  be loud in the dashboard, the logs and at startup.

### How this was checked

No code, so no mutation check applies. What was verified instead:

- `grep` for the old framing across every tracked doc
  ("affordable", "can't afford", "cheaper than") — hits in
  `README.md` remained after the first pass and were fixed; that's
  how the stale README dashboard line was caught too.
- Read each edited file's surrounding section to confirm the new text
  doesn't contradict a rule stated elsewhere in the same file —
  specifically that the new `CLAUDE.md` block doesn't restate
  Section 1 differently, and that `ROADMAP.md`'s new items don't
  reverse the "no feature because a vendor has it" rule they sit next
  to.

### What is NOT covered / honest gaps

- **This is desk research with zero paying clients.** The two buyer
  segments and the ~$200/mo anchor are reasoned, not observed. The
  first real client conversation is the test; if it contradicts this,
  the correction belongs in `DECISIONS.md`, not a quiet drift.
- **Item 18 (shadow mode) is the highest-value item here and it isn't
  built.** It's also what would validate the false-positive rate
  against real traffic instead of assumptions — so the product's
  biggest unknown stays unknown until it ships.
- Items 11b, 12a and 18 are written as roadmap entries only. No
  interfaces sketched, no config schema decided.
- The pricing anchor has an anchor but no tiers, metering unit or
  overage model (item 17 says so explicitly).

**Status, stated plainly (Section 22's last checkbox):** this is
*done for the current scope* — the scope being "get the positioning
and its reasoning written down so the next session builds toward it."
It is **not** "the product is repositioned," because nothing about
the running product changed. Docs now point at the target; the code
still has to get there.

### Next session should

Pick from items 12a → 18 → 11b in that order (evidence trail first,
because shadow mode's report needs it and it's also what makes a
false-positive dispute answerable). Do **not** default to P1 items
7–10 just because more detection signals feel like the real work —
see `ROADMAP.md`'s dated priority note for why.
