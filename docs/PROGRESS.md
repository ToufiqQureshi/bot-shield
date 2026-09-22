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

