# PROGRESS — what happened, in one line each

An index, not a journal. Git already stores what changed, and the commit
messages in this repository are written long and deliberately: what was
tried, what was rejected, what was verified and how. Repeating that here
made the file 3,700 lines, which every session then read instead of the
code.

So this file holds the **pointer**, and one thing git cannot give you.

## Format

```
### YYYY-MM-DD — <what it was>
`<commit>` — one or two lines on what changed and why.
Gotcha: <only when there is one — something that would bite the next
person and is not visible in the diff>
```

The gotcha line is the part worth keeping. `git log` will tell you that a
bit was masked out; it will not tell you that leaving it in makes the model
learn the label back.

## Where the detail actually lives

| Question | Look here |
|---|---|
| What exactly changed, and how was it verified? | `git show <commit>` |
| Why was it done this way, and what was rejected? | `docs/DECISIONS.md` |
| What is known about this threat or vendor? | `docs/RESEARCH.md` |
| How does this part work? | the topic doc — `SCORING_EXPLAINED`, `LEARNED_SCORING`, `DEPLOYMENT`, `ARCHITECTURE` |
| What is built and what is not? | `docs/WHAT_IS_BUILT.md` |
| Sessions before 2026-09-22 | `docs/PROGRESS_ARCHIVE.md` |

---

### 2026-09-22 — deployment setup, bot ladder, PROGRESS split
`14de934` — `deploy/` (Dockerfile, production compose, systemd unit, setup.sh
for a fresh box), `bot-testing/ladder/` (seven rungs, each adding one
capability), and this file became an index with the old content in
`PROGRESS_ARCHIVE.md`.
Gotcha: the container listens on **8443**, not 443 — the host publishes
443→8443 so the process needs neither root nor a capability. Debugging a
"port 443 refused" by looking at the process first will waste an hour.

### 2026-09-22 — where it deploys and what bandwidth costs
`c002cea` — `docs/DEPLOYMENT.md`: the TLS-termination constraint that rules out
every managed edge, AWS vs Hetzner egress maths, and ROADMAP item 27.
Gotcha: putting Cloudflare, Railway, an ALB or CloudFront in front does not
error. JA4 silently becomes a constant and detection quietly degrades.

### 2026-09-22 — what the product actually is
`ff55455` — `docs/WHAT_IS_BUILT.md`: plain-language inventory of what works,
what does not, and the measured numbers, for explaining it to someone.

### 2026-09-22 — scoring explained from zero
`35c1ef8` — `docs/SCORING_EXPLAINED.md`: the onboarding read. Worked examples
use real weights with outputs verified by running `Predict`.

### 2026-09-22 — label collection (ROADMAP item 26)
`14cc08c` — `pkg/labels` collects human labels from solved challenges and
automated ones from honeypot trips, behind `-collect-labels`. Seven mutation
checks.
Gotcha: `honeypot_trap` must be cleared from the vector of any sample it
labelled, or the model just learns the label back instead of learning from the
other eight checks.

### 2026-09-22 — how the model gets fed
`58f7ec1` — `docs/LEARNED_SCORING.md`: label sources, the traps, selection
bias, and the gate before a model may enforce.
Gotcha: a verified good-bot lookup looks like a free correct label and is not
usable — `guard.go` forwards crawlers before `Evaluate` runs, so no vector
exists, and training on them teaches the model to stop crawler-shaped traffic.

### 2026-09-22 — learned scoring weights (ROADMAP item 25)
`80be315` `27eb71d` `e716796` `2bff8f9` — `pkg/decide`: logistic regression over
the existing checks. 14ns and zero allocations per prediction, shadow only.
Lint went to 0 issues repo-wide, 13 of which pre-dated the branch.
Gotcha: the fired mask is **positional**. Reorder or rename a check and every
saved model and stored sample silently means something else. `FeatureVersion()`
is what guards it — do not work around it.

### 2026-09-23 — hosting decided: Hetzner
`e36d0f4` — DEPLOYMENT.md §3 rewritten from "AWS because free tier" to the
actual decision, with the comparison that drove it (Hetzner 20 TB included and
~$0.001/GB overage vs AWS 100 GB then $0.09/GB).
Gotcha: Hetzner has **no India datacenter**. Singapore is closest, so Indian
visitors pay 60–150 ms that AWS Mumbai would not charge — accepted because
bandwidth decides this product's economics and latency does not. Revisit if
that stops being true.
**Superseded by `46168a7`:** the bandwidth figures in this entry are Hetzner
EU pricing and do not apply to Singapore. Read that entry, not this one.

### 2026-09-23 — external review of PR #13: two fixes, two roadmap items
`3d06c1a` — `Collector.Record` could panic ("send on closed channel") when it
raced `Close`; guarded with an RWMutex and a closed flag. Corrected the comments
that claimed a solved challenge proves a browser ran JavaScript. Opened ROADMAP
28 (decode the canvas PNG) and 29 (label the honeypot trip on the request that
trips it).
Gotcha: a `select` with a `default` case does **not** make a send on a closed
channel safe — it still panics. The non-blocking send read as if it did, which
is exactly why nobody caught it.

### 2026-09-23 — DEPLOYMENT.md §3 corrected: the Hetzner numbers were EU-only
`46168a7` — the "20 TB included, €1/TB" figures that decided the hosting choice
are Hetzner **EU** pricing. Singapore includes 0.5 TB and charges €7.40/TB.
CX22 does not exist in Singapore at all (CX is EU-only; it has CPX/CCX).
Decision still holds — 11× cheaper per GB than AWS Mumbai, not 90×.
Gotcha: hakaishield is a **reverse proxy**, so the origin must live in the same
datacenter as the proxy. Proxy in Singapore with an origin in India makes an
Indian visitor cross that link three times — ~180–240 ms, not ~60 ms.

### 2026-09-23 — hosting settled: DigitalOcean Bangalore
`4bd5221` — DEPLOYMENT.md §3 rewritten again. DO Bangalore ($24/mo, 4 TB
included, $0.01/GiB) costs the same as Hetzner Singapore at 10 TB and sits in
India: 5–40 ms instead of 55–70 ms. `deploy/` retargeted.
Gotcha: the only reason Hetzner ever won was a wrong number — its famous
"20 TB / €1 per TB" is **EU-only**, and Singapore is 0.5 TB / €7.40 per TB.
Once that was corrected the cost gap vanished and latency decided it. Always
check a provider's rate for the region you are actually deploying to.
