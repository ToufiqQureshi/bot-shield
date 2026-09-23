# Learned scoring — how the model gets fed

> This document exists because the code is finished and the data is not.
>
> `pkg/decide` can train and score today. It has nothing trustworthy to
> train on. Everything below is about closing that gap: where labels come
> from, which ones are traps, how they reach the trainer, and what has to
> be true before a learned model is allowed to decide anything.
>
> Read this before building ROADMAP item 26.
>
> **New to the project?** Read `docs/SCORING_EXPLAINED.md` first. It explains
> how scoring and the model actually work, with worked numbers, and assumes no
> machine-learning background. This document assumes you have read it.

---

## 1. What already exists

| Piece | State |
|---|---|
| `pkg/decide` — train, score, explain, load, save | **built**, tested |
| `cmd/hakaishield-train` — offline trainer | **built**, tested |
| `-model` flag — load a model, shadow only | **built** |
| `signals.Evaluation.Fired` — the feature bitmask | **built** |
| `Evidence.Model` — the recorded opinion | **built**, nothing reads it yet |
| `pkg/labels` — collection, caps, parked samples | **built**, tested |
| `training_samples` table + `-collect-labels` | **built** |
| `cmd/hakaishield-train -db-url` — train off it | **built** |
| **Selection-bias correction (§3)** | **not built — an unmade decision** |
| Retention on a schedule | `db.DeleteSamplesBefore` exists, nothing calls it |
| Dashboard surface for any of it | not built, deliberately |

The trainer eats one JSON object per line:

```text
{"signals":["ua_mismatch","header_anomaly"],"automated":true}
{"signals":[],"automated":false}
```

So the whole problem is: **produce that file honestly.**

---

## 2. Where labels can come from

A label is only worth having if it comes from something that **actually
knows**, independently of our own scoring. Labelling from the rule score
would teach the model to repeat the guesses it exists to improve on, and
it would then score beautifully against the very data that misled it.

Three candidate sources were examined against the code. They are not
equally good, and the obvious one is the worst.

### 2.1 Solved JS challenge → human ⚠️ the best one we have, and it is forgeable

`pkg/challenge` issues a proof-of-work plus a canvas render plus
automation-global checks. A solve is **independent evidence**: it is not
our score played back, it is a capability test the client either passes
or does not. That independence is the whole reason it qualifies as a
label.

**But it does not prove a browser ran.** `verifyHandler` checks three
things, and a plain HTTP script can satisfy all three:

| Check | What it really proves |
|---|---|
| `validPoW` — sha256(nonce+answer) starts `00` | The client can compute sha256. Any language can. |
| `validCanvasProof` — prefix `data:image/png;base64,` and length > 100 | The client can concatenate a string. The pixels are never decoded. |
| `automation` / `headless` form fields are not `"true"` | Nothing. The client reports these about itself. |

So the cost of injecting one forged `human` label is one challenge token
and one nonce. `validCanvasProof` is documented in `DECISIONS.md` as an
accepted limitation *for challenge bypass*, where the trade-off is
reasonable. As a **training label source it is a poisoning vector**, and
that is a different argument that was not made when it was accepted.

**What bounds it today**

- the nonce is single-use (`NonceStore.Consume`), so one solve is one
  sample
- `pkg/labels` caps one `(tenant, IP, JA4)` identity at 5 samples/hour
  (`cap.go`)
- nothing trained on this data is allowed to decide anything (section 3)

An attacker rotating IPs defeats the cap. The real fix is to make the
canvas proof mean something — decode the base64 PNG server-side and
check dimensions, header sanity and pixel entropy, so the string has to
come from an actual render. Until that exists, treat every
`challenge_solved` sample as attacker-influencable and weigh it
accordingly.

**What has to be built.** The solve happens on a *later* request than the
one that was scored, so the fired vector has to survive the round trip.
The challenge token carries a nonce, and `pkg/challenge` already has a
`NonceStore` backed by Redis. Store `nonce → fired bitmask` there
alongside the nonce, and on a successful verify, emit
`(fired, automated=false)`.

**Do not put the bitmask in the token.** The token goes to the client.
Handing a bot a signed list of which of our checks it tripped tells it
exactly what to fix — the same reason the evidence endpoint is
token-gated and never gets wildcard CORS. Keep it server-side.

**A failed or abandoned challenge is not a bot label.** A real person on
a slow phone, with JS disabled, on a locked-down corporate browser, or
who simply closed the tab, produces exactly the same non-solve as a
scraper. Only solves are labels. Non-solves are unlabelled, and that is
fine — an unlabelled sample costs nothing.

### 2.2 Honeypot trip → automated ⚠️ usable, with one rule

`pkg/deception` injects a `display:none`, `aria-hidden`, `rel=nofollow`
link. Fetching it means something walked the DOM and followed a link no
person sees. That is independent of our scoring — it is behaviour.

**The rule: `honeypot_trap` must be dropped from the feature vector of
any sample it labelled.** It is one of the nine checks. Leave it in and
the model simply learns "honeypot_trap means automated", which is the
label, not a finding. The other eight features are what we want it to
learn from on those samples.

Not a perfect label either — `pkg/signals/honeypot.go` already says why:
a screen reader or an over-eager prefetch can reach a hidden link, and
those are real people. Weight it as strong evidence, not as truth.

### 2.3 Verified good bot → automated ❌ do not use

This looked like the easiest clean label and it is a trap, for two
separate reasons.

First, **the vector does not exist.** `guard.go` checks
`signals.IsVerifiedGoodBot` and forwards to the origin *before*
`signals.Evaluate` ever runs. Verified crawlers never get scored, so
there is no fired vector to pair a label with.

Second, and worse, **it would poison the model even if it did.**
Googlebot is automated, so `automated=true` is literally correct — and
training on it teaches the model that Googlebot-shaped traffic should be
stopped. Verified crawlers are fine in production because they bypass
scoring entirely, but the model would carry that lesson over to every
*unverified* client that happens to look similar. That is a false
positive aimed squarely at legitimate crawlers.

Leave this source alone.

### 2.4 Customer report → either label, rare

A customer saying "this was my user, you blocked them" is the highest
quality label there is and there will never be many. Build a path for it
when there are customers; it does not gate item 26.

---

## 3. The problem nobody notices until the model is wrong

**Selection bias.** Under `PolicyBalanced` only traffic that scored above
zero is challenged. So every human label we collect comes from a human
who *already looked suspicious*. We will have no labels at all for the
clean majority.

Two consequences:

1. **The base rate is wrong.** The model's bias term is learned from the
   sample it saw. A sample drawn only from suspicious traffic makes
   automated traffic look far more common than it is, and the model
   inherits that.
2. **It is not wrong about the hard cases.** Those suspicious-but-human
   requests are exactly the ones the rules get wrong today. As training
   data for the boundary, this sample is excellent.

So the data is good for *where to put the line* and bad for *how much
traffic is over it*.

Options, none free, pick before collecting rather than after:

- **Sample clean traffic deliberately.** Challenge a small fixed fraction
  of score-zero requests purely to label them. Honest, adds friction to
  real users, and the fraction is a product decision, not an engineering
  one.
- **Correct the bias at training time.** If we know the sampling rate we
  can reweight. Requires counting what we *didn't* label.
- **Only use the model in the suspicious band.** Let the rules keep
  deciding score-zero traffic and let the model refine everything above
  it. Smallest change, smallest claim, probably the right first move.

Whichever is chosen goes in `docs/DECISIONS.md`.

---

## 4. Why training is never automatic

An anti-bot product that retrains itself on live traffic hands the
attacker a write channel into its own detection.

Solving a challenge is something an attacker can choose to do. A bot that
deliberately solves challenges — or farms solves — is injecting
`automated=false` labels for its own fingerprint. Train on that
automatically and the model learns that this exact client is human. Two
weeks later it walks straight through and nothing in the logs says why,
because the system taught itself.

So:

- **Collection is automatic.** Labels accumulate without anyone doing
  anything.
- **Training is a command.** Someone runs it and reads the numbers.
- **Deployment is a separate decision.** A trained model goes into shadow
  first, never straight into enforcement.

Additional defences worth building with the collector, not after:

- **Cap the samples any one identity contributes.** One (IP, JA4) pair
  that produces thousands of "human" labels is an attack, not a customer.
- **Keep labels tenant-scoped.** One customer's traffic must not shape
  another's model (`CLAUDE.md` §16).
- **Record when and from where.** A poisoning attempt is only findable
  afterwards if the samples carry enough to find it.

---

## 5. Storage

Nothing durable exists yet. `pkg/evidence.Trail` is a 1000-entry
in-memory ring buffer that resets on restart — it is for answering "why
was this blocked" today, not for accumulating a training set.

Postgres is already there (Supabase, `pkg/db`). A table is the obvious
home.

**Store the bitmask, not the request.** A sample needs the fired checks
and the label. It does not need the IP, the user agent, the path or the
body. Storing less is cheaper, safer, and makes the retention
conversation short.

Sketch, not a migration:

```text
training_sample
  tenant_id      -- scoped, always (CLAUDE.md §16)
  fired          -- the bitmask (int)
  feature_names  -- or a build/version tag: the vector is POSITIONAL,
                 -- so a sample is meaningless without knowing which
                 -- check list produced it
  automated      -- the label
  source         -- 'challenge_solved' | 'honeypot' | 'customer_report'
  created_at     -- for retention and for finding a poisoning window
```

`feature_names` or an equivalent version tag is not optional. The same
positional-vector reasoning that makes `decide.Load` refuse a mismatched
model applies to stored samples: reorder the `checks` list and every old
row silently starts meaning something else.

Retention has to be bounded like every other traffic-derived store
(`CLAUDE.md` §15).

---

## 6. The pipeline, end to end

```text
request
  ↓
signals.Evaluate → Evaluation{Score, Signals, Fired}
  ↓
  ├─ challenged → nonce stored with Fired (Redis, already there)
  │                 ↓
  │              solved? → (Fired, automated=false)   ← independent
  │
  └─ honeypot trip → (Fired minus honeypot_trap, automated=true)
                          ↓
                    training_sample table (tenant-scoped, bounded)
                          ↓
                    export to JSONL  ─── a human runs this
                          ↓
                    cmd/hakaishield-train
                          ↓
                    model.json + held-out numbers on stderr
                          ↓
                    a human reads false-positives  ← the gate
                          ↓
                    -model model.json   (SHADOW — decides nothing)
                          ↓
                    compare against the rules on real traffic
                          ↓
                    a human decides whether it may ever enforce
```

Two of those steps are a person on purpose. See §4.

---

## 7. Running it

Turn collection on (it needs a database; there is nowhere else to put
samples, and the evidence trail is a small in-memory ring buffer):

```bash
./hakaishield -target https://example.com -db-url "$DATABASE_URL" -collect-labels
```

It records and decides nothing. Watch it work:

```text
label_sample_queued_total    samples accepted
label_sample_capped_total    refused by the per-identity cap
label_sample_dropped_total   queue full - the writer cannot keep up
label_written_total          actually stored
```

Once there is traffic, train straight off it:

```bash
go run ./cmd/hakaishield-train -db-url "$DATABASE_URL" -out model.json
./hakaishield -target https://example.com -model model.json   # shadow
```

`-tenant <id>` trains one customer's model instead of a shared one. The
trainer filters on this build's feature version, so rows captured before
a check was added or reordered are left out rather than silently
misread. A file still works too, for a set produced by hand:

```bash
go run ./cmd/hakaishield-train -in labelled.jsonl -out model.json
```

The trainer holds back a fifth of the data and prints both scores:

```text
training set  samples=3072  logloss=0.0654  accuracy=0.990  false-positives=0  false-negatives=30
held-out set  samples=768   logloss=0.0786  accuracy=0.987  false-positives=0  false-negatives=10
```

**Read the held-out line, not the training line.** A model that does well
on the data it was fitted to and poorly on the rest has memorised the
sample. And read **false-positives** before accuracy: each one is a real
visitor a customer would have lost.

---

## 8. Before a model may enforce anything

All of these, not some:

- [x] Labels come from independent sources only (§2), with `honeypot_trap`
      excluded from the samples it labelled. (Built; `pkg/core/guard.go`
      clears the bit, and a mutation check fails if it stops.)
- [ ] The selection-bias question (§3) has an answer written in
      `DECISIONS.md`.
- [x] Per-identity sample caps exist, so one client cannot flood the set.
      (`pkg/labels/cap.go`: 5 per (tenant, IP, JA4) per hour.)
- [ ] Held-out false positives are **better than the rule scorer's** on
      the same held-out data. Not "good" — better. The rules are the
      thing being replaced; beating them is the bar.
- [ ] It has run in shadow on real traffic long enough for
      `model_shadow_disagree_total` to mean something, and the
      disagreements have been looked at one by one.
- [ ] A person decided. Not a threshold.

Until then `-model` stays shadow, and the rule scorer keeps deciding.

---

## 9. Open questions

- **Which bias correction** (§3)? Leaning toward "model refines the
  suspicious band only", as the smallest honest first claim.
- **Per-tenant models or one shared model?** Per-tenant fits traffic
  better and needs far more data per customer; shared is trainable
  sooner. Probably shared first, per-tenant as an enterprise feature.
- **How much data is enough?** Nine binary features is a small model, so
  thousands rather than millions — but the answer is "when the held-out
  numbers stop moving", measured, not guessed.
- **Does `Evidence.Model` get a dashboard surface before or after a real
  model exists?** After. Rendering an absent field is the kind of
  believable empty state `CLAUDE.md` §27 warns about.

---

## Related

- `docs/DECISIONS.md` — "Learned decision weights are a linear model over
  existing signals, not a new model" (why logistic regression, why not a
  hosted System One model, why the stricter block bar)
- `docs/RESEARCH.md` — the TypeSafe Jev / MLX / Core ML scan
- `docs/ROADMAP.md` — items 25 (this) and 26 (the label pipeline)
- `docs/ARCHITECTURE.md` — "Learned scoring, shadow only"
- `backend/pkg/decide/` — the model, the training, and their tests
