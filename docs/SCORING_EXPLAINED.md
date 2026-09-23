# How scoring works — start here

**Who this is for:** you just joined, you have to touch scoring or the model,
and you want to actually understand it rather than pattern-match around it.
No machine-learning background assumed. Read it top to bottom once; it takes
about twenty minutes and will save you a week.

**What this is not:** `docs/LEARNED_SCORING.md` is the design document — the
decisions, the traps, the open questions. This one explains how the thing
works. Read this first, that one second.

---

## 0. The job, in one paragraph

A request arrives. We have a few milliseconds and no idea who sent it. We have
to answer two questions: *is this automated?* and *what should we do about
it?* — and then be able to tell the customer **why**, in terms they can argue
with. That last part is not a nice-to-have. It is what the product sells.

---

## 1. How a request becomes a decision today

Nine independent checks run against every request. Each one is a yes/no
question, and each one is deliberately a different **kind** of evidence:

| Check | What it notices |
|---|---|
| `fragmented_handshake` | The TLS handshake was split in a way real browsers do not split it |
| `ua_mismatch` | The client claims to be Chrome, but its TLS fingerprint disagrees |
| `header_anomaly` | A browser claim with none of the headers browsers send |
| `ja4_blocklist` | A TLS fingerprint we have verified belongs to a scraping tool |
| `scripting_tool` | The user agent says `python-requests`, `curl`, etc. |
| `velocity_spike` | This IP is requesting far faster than a person reads |
| `ja4_velocity_spike` | This TLS fingerprint is, across IPs |
| `crawl_pattern` | Walking many distinct URLs, fetching no images or CSS |
| `honeypot_trap` | Followed an invisible link no human can see |

They are different on purpose. A scraper can fake a user agent trivially, fake
a TLS fingerprint with effort, and fake *browsing like a person* only by
actually being slow — so making it past one check is easy and making it past
all nine is expensive. That is the whole design: **no single check decides
anything.**

### The arithmetic

Each check that fires adds points. `backend/pkg/signals/score.go`:

```text
fragmented_handshake   50      ja4_blocklist      100
ua_mismatch            50      scripting_tool     100
header_anomaly         25      velocity_spike      50
crawl_pattern          50      ja4_velocity_spike  50
honeypot_trap          50

score >= 100  ->  block
score  >  0   ->  challenge
score ==  0   ->  allow
```

Worked example. A request arrives claiming Chrome, with a TLS fingerprint that
is not Chrome's, and no `Sec-Fetch-*` headers:

```text
ua_mismatch      fires   +50
header_anomaly   fires   +25
                        ----
                          75   ->  75 < 100, so: challenge
```

It gets the JavaScript challenge. A real person passes it in a second and
never notices. A scraper does not.

### The thing to notice

**Where did 50 and 25 come from?** Someone sat down and thought "a user-agent
mismatch feels about twice as damning as missing headers." That is a
reasonable guess by someone who knows the domain. It is still a guess. Nobody
measured it against real traffic, because at the time there was no real
traffic to measure against.

That is the entire reason `pkg/decide` exists.

---

## 2. The realisation that led to the model

Look at the rule again, written differently:

```text
50·(ua_mismatch) + 25·(header_anomaly) + 50·(fragmented) + ... >= 100
```

Each check is 0 or 1. Each has a number in front of it. Add them up, compare
to a threshold.

**That is a linear model.** The structure is already exactly what a trained
classifier produces. The only difference is where the numbers came from.

So "add machine learning to hakaishield" does not mean bolting something new
on the side. It means: **keep the structure, measure the numbers.**

This matters because it tells you what *not* to build. We do not need a neural
network — nine yes/no inputs cannot support that much capacity, and a bigger
model would fit noise and stop being explainable. We do not need a hosted AI
API — that is 70–500ms and a per-request bill on a path that currently costs
36µs. (The full reasoning, including the vendor we evaluated and rejected, is
in `docs/DECISIONS.md` and `docs/RESEARCH.md`.)

We need logistic regression. Which is maths from 1958 and about 200 lines of
Go.

---

## 3. What the model actually is

### Step 1: the same weighted sum

```text
z = bias + (w₁ · x₁) + (w₂ · x₂) + ... + (w₉ · x₉)
```

`xᵢ` is 1 if check *i* fired, 0 otherwise. `wᵢ` is its weight. Same shape as
the rule scorer. The `bias` is new — it is the model's starting opinion before
any evidence, which encodes how common automated traffic is in general. A
large negative bias means "assume human until shown otherwise."

### Step 2: turn the sum into a probability

`z` is an unbounded number — it can be −4 or +8. That is not something you can
show a customer. So we squash it into 0–1 with the **sigmoid**:

```text
p = 1 / (1 + e^(−z))
```

```text
 z:  -6     -4     -2      0      2      4      6
 p: 0.002  0.018  0.119  0.500  0.881  0.982  0.998
```

`z = 0` means exactly 50/50. Positive z leans automated, negative leans human,
and the curve flattens at both ends so extreme evidence stops mattering much —
which is correct behaviour. Once you are 99% sure, more evidence should not
make you meaningfully more sure.

### Why this shape and not something simpler

The weights live in **log-odds** space, and that is what makes them add up
honestly. Two independent pieces of evidence multiply your odds; adding their
log-odds does the same thing. That is why `wᵢ` can simply be summed, and why
each one is individually meaningful — which is what makes `Explain` possible.
Any other squashing function would lose that.

### Step 3: the decision

```text
p >= 0.9  AND  at least 2 checks fired   ->  block
p >= 0.5                                 ->  challenge
otherwise                                ->  allow
```

That second condition on block is not statistics, it is policy, and it is
there deliberately — see §6.

---

## 4. A real prediction, end to end

These are actual weights a training run produced, and actual outputs. You can
reproduce them.

```text
bias                  -3.8335
fragmented_handshake  +2.0437      ja4_blocklist        0.0000
ua_mismatch           +5.7201      scripting_tool      +6.9825
header_anomaly        +1.2228      velocity_spike       0.0000
crawl_pattern         +5.8786      ja4_velocity_spike   0.0000
honeypot_trap         +6.2137
```

Two things to notice before the arithmetic. **`header_anomaly` came out
weakest** (+1.22) while `scripting_tool` came out strongest (+6.98) — which is
the same ranking the hand-tuned weights had (25 vs 100). The model was never
told that; it worked it out. And **three weights are exactly zero**: those
checks never fired in the training data, so the model correctly has no opinion
about them. A zero weight means "no evidence", not "harmless".

### Case A: nothing fires

```text
z = -3.8335
p = 0.0212        2.1% automated
                  -> allow
```

### Case B: only `header_anomaly` fires

```text
z = -3.8335 + 1.2228 = -2.6107
p = 0.0685        6.9% automated
                  -> allow
```

**The rules would have challenged this** (score 25, which is > 0). The model
allows it. That is a real disagreement, and it is the model saying: *missing
fetch-metadata headers on their own is weak — privacy browsers and corporate
proxies strip them, and in the training data most clients that looked like
this were people.*

The model might be right. It might also be wrong because of how the data was
collected (§7). **This is exactly what shadow mode exists to find out**, and
why no model is allowed to enforce on the strength of looking sensible.

### Case C: `ua_mismatch` + `header_anomaly`

```text
z = -3.8335 + 5.7201 + 1.2228 = +3.1094
p = 0.9573        95.7% automated
confidence 0.9146
2 checks fired    -> block
```

And the explanation the customer sees is not "95.7%, trust us":

```text
ua_mismatch      +5.7201   ← claimed a browser its TLS handshake isn't
header_anomaly   +1.2228   ← and sent none of that browser's headers
bias             -3.8335
                 --------
                 +3.1094   ->  95.7%
```

Every number is checkable. The contributions plus the bias sum to `z`, and
sigmoid(z) is the probability shown. There is a test that fails if they ever
stop adding up (`TestExplainSumsToThePredictedLogOdds`).

### Case D: `scripting_tool` alone — the interesting one

```text
z = -3.8335 + 6.9825 = +3.1490
p = 0.9589        95.9% automated  — higher than case C!
1 check fired     -> challenge, NOT block
```

The model is *more* confident here than in case C, and it still does not
block. One signal is never enough to refuse a visitor outright, however
certain a model is about it (§6).

---

## 5. Where the weights come from

Nobody picks them. Training finds them. The method is **gradient descent**,
and it is less mysterious than it sounds:

```text
1. Start with every weight at 0.
2. For each labelled request, predict it.
3. Measure how wrong you were:  error = predicted − actual
     predicted 0.9, actually human (0)  ->  error +0.9   (badly wrong)
     predicted 0.9, actually a bot  (1)  ->  error -0.1   (nearly right)
4. Nudge every weight that fired on that request against its error.
5. Repeat 2000 times over the whole set.
```

That is the entire algorithm. A weight for a check that keeps showing up on
bots gets pushed up. One that keeps showing up on humans gets pushed down. One
that shows up on both drifts toward zero, because it is not evidence.

Two details in the code that matter:

**L2 regularisation.** Without it, a check that happens to be perfectly
separating in your sample drives its weight toward infinity, and the model
becomes certain about a pattern it saw four times. L2 penalises large weights
and keeps that honest.

**It is deterministic.** Same data in, same weights out, every time. No
randomness anywhere. This is deliberate: without it, a drop in detection
quality would be impossible to tell apart from training noise.

It runs offline, never in the request path, and takes about a second.

---

## 6. The safety rails, and why each one exists

Every one of these is a rule somebody could remove to make a number look
better. Each is load-bearing.

**A model never blocks on a lone signal.** Case D above. The rule scorer does
permit single-signal blocks, but only for two checks (`ja4_blocklist`,
`scripting_tool`) that were verified against real captures by a person. No
single model weight has that behind it — it is a number fitted to data — so
the model gets the stricter rule. `CLAUDE.md` §10 and §14.

**A model decides nothing until a person says so.** `-model` loads it in
*shadow*: it scores alongside the rules and records what it would have done,
and the rules decide. Agreements and disagreements are counted. A model earns
enforcement by being compared against the rules on real traffic — not by
passing tests.

**A mismatched model is fatal at startup.** The fired mask is *positional* —
bit 3 means `ja4_blocklist` only because that is where it sits in the list. Add
or reorder a check and an old model applies every weight to the wrong signal,
confidently. So the feature names and their order are stored in the model file
and checked at load, and a mismatch stops the process rather than scoring
wrong quietly.

**Training never runs automatically.** This is the one people want to remove.
Solving a challenge is something an attacker can choose to do. A bot that
deliberately solves challenges — or farms solves — is injecting *"this is a
human"* labels for its own fingerprint. Retrain on that automatically and the
model teaches itself that this exact client is a person. Two weeks later it
walks straight through and nothing in the logs says why.

So: collection is automatic, training is a command someone runs, deployment is
a separate decision.

**One identity can contribute 5 samples an hour.** The other half of that
defence. Contributing more labels requires more distinct (IP, JA4) identities,
which costs an attacker the same as evading the rest of detection.

---

## 7. Where the data comes from

A label is *"this request really was / was not automated."* It is worth having
only if it comes from something that **actually knows**, independently of our
own scoring — label from the rule score and the model just learns to repeat
the guesses it exists to improve on, then scores brilliantly against the very
data that misled it.

Two candidate sources collect themselves once `-collect-labels` is on:

**A solved JS challenge → human candidate.** The proof-of-work can be computed
by an automated client; canvas and automation fields are client supplied and
can be forged. This is an observation, not verified ground truth. The solve
arrives on a *later* request than
the one that was scored, so the fired mask is parked server-side against the
challenge nonce and claimed when the solve lands. It is never put in the token
— the token goes to the client, and handing a bot a list of the checks it
tripped tells it exactly what to fix.

**A honeypot trip → automated candidate.** An invisible, `aria-hidden`,
`nofollow` link is strong evidence, but prefetch and accessibility tools can
also reach it. The first trap hit is captured without a follow-up request.
One rule: `honeypot_trap` is
stripped from the sample it labelled. Leave it in and the model just learns
"honeypot_trap means automated" — which is the label, not a finding. The other
eight checks on that request are the part worth learning from.

**A failed challenge is not a bot label.** Someone on a slow phone, with JS
off, on a locked-down work laptop, or who just closed the tab produces exactly
the same silence as a scraper. Only solves are labels.

**Verified good bots are not a label either**, and this one catches people.
Googlebot *is* automated, so it looks like a free correct label. Two problems:
`guard.go` forwards verified crawlers before scoring even runs, so there is no
mask to pair with — and training on them would teach the model that
crawler-shaped traffic should be stopped, which is a false positive aimed
squarely at legitimate bots.

### The trap you should know about before you touch any of this

**Selection bias.** Under the default policy, only traffic that already scored
above zero gets challenged. So every human candidate we collect comes from
traffic that **already looked suspicious**.

Think of a doctor who only ever examines people in a hospital, then concludes
most people are ill.

This makes the data *excellent* for deciding where the boundary goes — those
borderline cases are precisely what the rules get wrong today — and *wrong*
about how much traffic sits over that boundary. The model's bias term inherits
the skew.

This is an open decision, not a solved problem. `docs/LEARNED_SCORING.md` §3
lays out three ways to handle it. **Nothing should enforce a learned model
until it is decided.**

---

## 8. Running it

```bash
cd backend

# 1. Collect. Records, decides nothing. Needs a database.
./hakaishield -target https://example.com -db-url "$DATABASE_URL" -collect-labels

# 2. Train on independently reviewed labels.
go run ./cmd/hakaishield-train -in verified-labels.jsonl -out model.json

# 3. Load it — SHADOW. It still decides nothing.
./hakaishield -target https://example.com -model model.json
```

The trainer refuses automatically collected database candidates by default.
`-allow-unverified-labels` permits DB training only as a shadow experiment;
its held-out score does not prove production accuracy.

Watch collection with the counters on the observability endpoint:

```text
label_sample_queued_total     accepted
label_sample_capped_total     refused by the per-identity cap
label_sample_dropped_total    queue full — the writer cannot keep up
label_written_total           actually stored
```

Watch the shadow model:

```text
model_shadow_agree_total      model and rules agreed
model_shadow_disagree_total   they did not — these are what you read
```

---

## 9. Reading the training output

```text
training set  samples=3072  logloss=0.0654  accuracy=0.990  false-positives=0  false-negatives=30
held-out set  samples=768   logloss=0.0786  accuracy=0.987  false-positives=0  false-negatives=10
```

**Read the held-out line.** The trainer keeps back a fifth of the data and
never trains on it. A model that scores well on the data it was fitted to and
badly here has memorised your sample rather than learned your traffic. The two
lines being close, as above, is the good sign.

**Read false positives before accuracy.** Each false positive is a real
visitor a customer would have lost. Accuracy averages them away with correct
bot blocks; it will look excellent while the model quietly turns away
customers. They are reported separately for exactly this reason.

**Log loss** is how surprised the model was by the truth. Lower is better.
0.693 is what you score by answering "no idea" to everything, so anything
above that is worse than useless.

**Before a model may ever enforce**, its held-out false positives must be
**better than the rule scorer's on the same held-out data**. Not "good" —
better. The rules are what it replaces. The full gate is
`docs/LEARNED_SCORING.md` §8.

---

## 10. Mistakes that look right

- **Labelling from our own score.** The model learns to copy our guesses, then
  scores 99% against them. This is the single easiest way to build something
  useless that looks excellent.
- **Leaving `honeypot_trap` in the samples it labelled.** The model learns the
  label back. Accuracy looks great. It has learned nothing.
- **Judging a model on its training numbers.** Always the held-out line.
- **Chasing accuracy.** A model that blocks everything is 95% accurate on 95%
  bot traffic, and destroys the customer's business.
- **Reordering or renaming a check without thinking about stored data.** The
  mask is positional. Old models and old samples silently start meaning
  different things. `FeatureVersion()` is what protects you — do not work
  around it.
- **"Let's just retrain nightly."** See §6. This hands an attacker a write
  channel into detection.
- **Trusting a zero weight.** It means the check never fired in training data,
  not that it does not matter.

---

## 11. Where the code is

```text
backend/pkg/signals/score.go      the nine checks, the hand-tuned weights,
                                  Evaluate() -> {Score, Signals, Fired}
backend/pkg/decide/model.go       Predict, Explain, Load, Save
backend/pkg/decide/train.go       Train, Score (the quality metrics)
backend/pkg/labels/               collection: queue, per-identity cap,
                                  parked challenge samples
backend/pkg/core/guard.go         where a request meets all of it
backend/cmd/hakaishield-train/    the offline trainer
```

Every one of those has tests next to it, and the important ones have been
mutation-checked — deliberately broken to confirm the test actually fails.
If you change behaviour there, do the same; `CLAUDE.md` §12 is not optional.

---

## 12. Glossary

| Term | What it means here |
|---|---|
| **Feature** | One check. Nine of them. Each is 0 or 1 per request. |
| **Fired mask** | A 32-bit number, one bit per check. Bit *i* set = check *i* fired. |
| **Weight** | How much one check pushes the decision, in log-odds. |
| **Bias** | The model's opinion before any evidence — the base rate. |
| **Log-odds (z)** | The weighted sum. Unbounded. Adds up honestly. |
| **Sigmoid** | Squashes log-odds into a 0–1 probability. |
| **Label** | Ground truth: was this request really automated? |
| **Held-out set** | Data kept back from training, used to check for memorisation. |
| **False positive** | A real visitor we stopped. The number that matters most. |
| **Shadow mode** | Scoring and recording without acting. |
| **Selection bias** | Your sample is not your traffic. See §7. |

---

## Related reading

- `docs/LEARNED_SCORING.md` — the design: decisions, traps, open questions
- `docs/DECISIONS.md` — why logistic regression and not a hosted AI model
- `docs/RESEARCH.md` — the vendor scan behind that decision
- `docs/ARCHITECTURE.md` — where scoring sits in the request path
- `CLAUDE.md` §10, §12, §14 — the detection, testing and false-positive rules
