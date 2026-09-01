# Live demo script — the Enforcer convergence run

The ~7-minute demo segment for the API World 2026 workshop. Runs against
Enforcer's **real** Vitest suite — every PASS / FAIL on screen is genuine.
No infrastructure: no Postgres, no Go, no daemon. Whole cycle ≈ 30 s of
command time.

Run everything from the repo root (`src/`). Font size up, one terminal,
one editor showing `src/daemon/routes/enrich.ts`.

If anything stalls: deck slides **14 / 20 / 25** carry the captured output —
hit **play**. Do not debug live.

---

## Run it — the driver script

You don't type the commands live. Run the driver and press **Enter** to
advance each beat:

```bash
demo/scripts/run-demo.sh
```

It walks the seven beats below, pausing for a keystroke before each command,
printing everything verbose (the diff, the receipt JSON, `git status`).
**Ctrl-C** at any point aborts and restores a clean tree.

Pre-flight self-test — run straight through with no pauses:

```bash
demo/scripts/run-demo.sh --auto
```

The beats below are the reference: what each command shows and what to say
over it.

---

## Pre-flight (before the session)

```bash
cd /path/to/repo
npm ci                        # once
npm test                      # sanity: 152 passing, ~4s
demo/scripts/revert.sh        # ensure a clean tree (no stale overlay)
git status --porcelain        # expect empty
```

- [ ] `node demo/radar/gate-fast.mjs` → FAST GATE VERDICT: PASS
- [ ] Editor open at `src/daemon/routes/enrich.ts` — **read-only; don't save it**
- [ ] Terminal font large; window wide enough for ~80 columns
- [ ] Deck open at slide 34 (demo setup)
- [ ] **No other terminal or editor is going to write to this repo during the
      run.** The driver applies and reverts `enrich.ts` as it goes; if anything
      else edits or reverts that file mid-run, `run-demo.sh` will stop itself
      with a red `UNEXPECTED` message rather than narrate a stale result.

---

## Beat 0 — framing · ~20 s · (slide 34 on screen)

> "The target here is Enforcer itself — the policy-and-audit control plane this
> whole repo is. An agent has opened a pull request against it. One file. It
> compiles, it introduces no secrets, and almost every unit test stays green.
> Watch what stops it."

---

## Beat 1 — clean baseline · ~35 s

**Type:**
```bash
node demo/radar/gate-fast.mjs
```
**Appears:**
```
FAST GATES · baseline
  G1  tsc --noEmit ... PASS
  G2  secret-leak scan ... PASS
  G3  touched-module tests (6) ... PASS 69/69
  FAST GATE VERDICT: PASS   6.3s
```

> "Clean tree. Three fast gates — typecheck, a secret-leak scan, and the
> touched-module tests. All green. This is the state a good PR should preserve."

---

## Beat 2 — the AI-generated PR lands · ~40 s

**Type:**
```bash
demo/scripts/apply-change.sh 01-audit-mutation
git diff --stat
```
**Appears:** one file changed — `src/daemon/routes/enrich.ts`.

**Show the diff in the editor.** The change replaces the append-only enrichment
path with: *find the pending audit event, write the outcome onto it in place,
re-store it.*

> "The commit message is reasonable: 'the pending event already has the full
> action context, so appending a second row is redundant — just fill in what
> happened.' It reads like a tidy refactor — one file. The response body even
> still returns `append_only: true`."

---

## Beat 3 — the fast gate catches it · ~55 s · (advance deck to slide 25)

**Type:**
```bash
node demo/radar/gate-fast.mjs --change 01-audit-mutation
```
**Appears:**
```
  G1  tsc --noEmit ... PASS
  G2  secret-leak scan ... PASS
  G3  touched-module tests (6)  FAIL  1 failing
        FIRST FAILURE: tests/integration/audit-enrich-immutability.test.ts —
        Audit Enrichment Immutability (invariant floor)
        appends an enrichment event and leaves the original pending event byte-identical
  FAST GATE VERDICT: FAIL   6.3s
```

> "It compiles. No secrets. A scanner-only gate would wave this through. What
> fails is G3 — the invariant floor `audit-enrich-immutability`. The audit
> trail's whole tamper-evidence guarantee is: an event, once written, is never
> mutated. This change mutates it."

---

## Beat 3.5 — how this actually gets fixed · ~30 s

> "Before we look at what runs next: in the full pipeline, a failure like this
> doesn't go straight to a human. Several independent AI models each look at
> it on their own — observe, analyze, propose a fix — without seeing each
> other's answer. They then cross-review and converge on one fix. Only then
> does the system pick which tests that fix needs to be checked against, and
> the whole loop repeats until nothing is failing. We're skipping straight to
> the converged fix so we can walk through the mechanics in the time we have."

---

## Beat 4 — RADAR selection · ~50 s · (advance deck to slide 14)

**Type:**
```bash
node demo/radar/select.mjs --change 01-audit-mutation
```
**Appears:** six `[floor]` rows, each with a *reason* and a *dimension*, then
`selected corpus: 6 test files (6 mandatory floors enforced, 0 by
reachability)`, followed by `6 of 10 test files selected (69 of 152
individual tests in the whole suite)`.

> "The selector isn't guessing. Every test on the list carries the reason it
> was pulled in and the blast-radius dimension it covers. This change touches
> one file and it lands entirely on floor-covered surface — so the six
> mandatory floors *are* the corpus. RADAR is forbidden from dropping any of
> them, whatever the blast-radius math says. And look at the bottom line: six
> of the repo's ten test files, about sixty-nine of its hundred fifty-two
> individual tests. Everything else got left out on purpose, not by luck —
> nothing about it could plausibly be touched by this change."

---

## Beat 5 — the run and the verdict · ~45 s · (advance deck to slide 20)

**Type:**
```bash
node demo/radar/run.mjs
```
**Appears:**
```
  PASS  approval-service 20/20 · mcp-gateway 12/12 · policy-engine 16/16
  FAIL  audit-enrich-immutability 1/2
        ↳ leaves the original pending event byte-identical
  PASS  audit-pipeline 14/14 · rbac-regression 5/5
  VERDICT: FAIL   68 passed, 1 failed, 69 total
  FIRST FAILURE: audit-enrich-immutability
  admissible_for_merge: false
```

> "Sixty-eight of sixty-nine green — and it doesn't matter. One floor red is a
> hard stop. The run folder now has a receipt: `admissible_for_merge: false`,
> hashed against this exact change."

---

## Beat 6 — the fix, stacked on the diff, re-run · ~75 s

**Type:**
```bash
demo/scripts/apply-change.sh 02-fix
node demo/radar/run.mjs
```
**Appears:**
```
  PASS  audit-enrich-immutability 3/3      # was 1/2 — the fix ships with a counter-example
  VERDICT: PASS   70 passed, 0 failed
  admissible_for_merge: true
```

> "The fix restores the append-only path — and it ships with a counter-example:
> it freezes the original event so any in-place write is now a thrown error, not
> a silent field change, and it proves repeated enrichment only ever appends.
> Re-run against the same diff: seventy of seventy. The receipt flips to
> `admissible_for_merge: true`. That's the Convergence Loop — the fix lands on
> the diff, the corpus re-runs, the residual goes to zero before a human ever
> looks at it."

---

## Beat 7 — cleanup + the line · ~25 s

**Type:**
```bash
demo/scripts/revert.sh
```

> "Back to a clean tree. The takeaway: a tamper-evidence regression that
> compiled clean and passed the secret scan was stopped by an invariant floor
> the selector isn't allowed to skip — before it reached review."

---

## Timing budget

| Beat | Target | Running |
|---|---:|---:|
| 0 framing | 0:20 | 0:20 |
| 1 baseline | 0:35 | 0:55 |
| 2 PR lands + diff | 0:40 | 1:35 |
| 3 gate catches it | 0:55 | 2:30 |
| 3.5 how the fix converges | 0:30 | 3:00 |
| 4 RADAR selection | 0:50 | 3:50 |
| 5 run + verdict | 0:45 | 4:35 |
| 6 fix + re-run | 1:15 | 5:50 |
| 7 cleanup + line | 0:25 | 6:15 |
| slack / questions | 0:45 | 7:00 |

## If you're tight on time — cut in this order

1. Beat 1 (the clean baseline) — mention it, don't run it.
2. Beat 3.5 (how the fix converges) — one sentence, or skip: "this gets fixed
   by a converged, cross-reviewed AI proposal — we're jumping to that fix."
3. Beat 4 (`select`) — the six floors are visible on slide 14; describe, don't run.
4. Beat 2's `git diff --stat` — just narrate the one-file change.

**Never cut:** the gate FAIL with its FIRST FAILURE line (Beat 3), and the
`admissible_for_merge: false → true` flip across Beats 5–6.

## Recovery

- Command hangs or errors → switch to the deck, hit **play** on the matching
  scripted terminal (14 = select, 20 = run defect→fix, 25 = gate-fast), narrate
  over it.
- `apply-change.sh` refuses ("working tree is dirty") → `demo/scripts/revert.sh`,
  then `git stash` any unrelated edits, retry.
- Wrong overlay stuck → `demo/scripts/revert.sh` unwinds the whole stack.
- `run-demo.sh` stops with a red **UNEXPECTED** message → it detected the demo
  state was wrong (baseline gate failing, or the seeded change not triggering a
  test failure) and refused to narrate something untrue. It has already reverted
  to a clean tree; run `git status` (expect only unrelated edits), then re-run.
