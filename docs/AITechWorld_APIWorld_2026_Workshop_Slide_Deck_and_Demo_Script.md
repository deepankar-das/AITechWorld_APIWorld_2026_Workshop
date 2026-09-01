# Securing AI-Generated APIs at the Speed of AI

## Risk-Aware Test Selection, the Agentic Development Model, and Closure Discipline

**Event:** API World + CloudX + AI TechWorld 2026
**Session:** Tuesday, September 1, 2026 · 12:00–12:50 PM PDT
**Location:** API World — Workshop Stage A
**Speaker:** Deepankar Das — CEO & Founder, Cyberhead AI
**Slide deck (live artifact):** `docs/AITechWorld_APIWorld_2026_Workshop_Deck.html` (39 slides, fixed 1280×720 canvas)
**This document:** v1.1 — full word-for-word speaking script, matching the current 39-slide deck (includes the slide 36 "switch to the terminal" placeholder) and the real, runnable Enforcer demo. Supersedes the v0.1 draft (which described an earlier, unbuilt cache-key/tenant-isolation demo). Use this instead of the deck's own in-app notes panel when you want the full script, not just cues.

---

# Run of show

The deck carries no built-in per-slide timestamps — the times below are an estimated pacing budget, scaled from the deck's own stated "~38 min + demo clips + Q&A" inside a 50-minute slot. Treat them as a rehearsal aid, not a contract; adjust after your first run-through. The live-demo timings *are* authoritative — they come straight from `demo/DEMO_SCRIPT.md`, which is written against the real driver script.

| Act | Slides | Content | Est. time |
| :---- | :---- | :---- | ----: |
| Act I | 1–8 | Cold open, the Faros AI whiplash data, why shift-left never worked, why coverage is dead at agent velocity, Gartner's 40% cancellation stat, naming the Agentic Development Model | ~8 min |
| Act II·a — RADAR | 9–15 | What RADAR is, the seven blast-radius dimensions, false-negative-rate targets, mandatory floors, a live-captured selection run, the run folder as audit trail | ~8 min |
| Act II·b — Convergence Loop | 16–20 | The eight-step loop, why it's only economical now, the convergence curve, a live-captured run showing the loop close | ~6 min |
| Act II·c — CI vs. nightly | 21–26 | Architecture-first security, the whitebox/blackbox split and why, a live-captured fast-gate catch, the SQLi catch story | ~6 min |
| Act III — Closure discipline | 27–33 | Why a green rerun isn't closure, the Cross-Model Convergence Review jury, the research behind it, the commit-attested gate, "works" is not a test, the human checkpoint (Ford) | ~8 min |
| Act IV — Demo | 34–36 + live demo | Demo framing, then a "switch to the terminal" placeholder slide, then the real live demo against Enforcer's own repo (see the Live Demo Script below) | ~2 min + 7 min |
| Act IV — Close | 37–39 | Take-home, three things, resources, open Q&A | ~5 min |

That sums to ~50 minutes including buffer. If you're short on time, the live-demo section already has its own tested cut order — see **Live Demo Script → If you're tight on time** below. Upstream of that, the highest-value cuts are slide 30 (research citations — mention the headline number, skip the per-paper detail) and slide 24 (why whitebox/blackbox split — the split itself on slide 23 usually speaks for itself).

---

# Presentation deck

## SLIDE 1 — Title · ~30s

### On slide

> **Securing AI-Generated APIs**
> at the speed of AI
>
> Deepankar Das — CEO & Founder, Cyberhead AI
>
> Risk-aware test selection, the Agentic Development Model, and closure discipline — a verification system that keeps pace with code generation.
>
> Sep 1 2026 · 12:00–12:50 PDT / ~38 min · demo clips · Q&A

### Speaker script

> Good afternoon. This session is about a practical problem: AI coding agents can now generate API code faster than conventional testing and review can establish whether that code is safe to ship.
>
> This is not a session about whether agents can write an endpoint. They can, routinely. It's about what happens next — how a team verifies contract correctness, runtime behavior, authorization, tenant isolation, and security at the same pace the agents create changes, without slowing them back down to human speed and without pretending the diff is safe because it compiled.

### Delivery notes

- Open flat — this is an engineering and verification session, not a product pitch. Don't lead with the Agentic Development Model name yet; that gets introduced deliberately on slide 8.
- Target ~30 seconds. Let the room settle.

---

## SLIDE 2 — Cold open · ~35s

### On slide

> Your agents landed **47 commits** today.
> Your last pen test **was in** March.

> How many of those commits introduced a contract break, an authorization gap, or a tenant-isolation defect that nobody finds until the next audit?

### Speaker script

> Your agents landed forty-seven commits today. Your last penetration test was in March.
>
> How many of those forty-seven commits introduced a contract break, an authorization gap, or a tenant-isolation defect that nobody finds until the next audit?

### Delivery notes

- Rhetorical question — the rest of the talk answers it. Let the numbers land before you speak; don't rush the pause after "was in March."
- ~35–40 seconds, then move immediately.

---

## SLIDE 3 — Act I divider · ~15s

### On slide

> **I**
> The problem is not that AI writes insecure code.
> It is that the loop never converges.

### Speaker script

> The problem is not that AI writes insecure code. It is that the loop never converges.

### Delivery notes

- Act I. The divergence thesis: the naive loop lets new bugs land faster than old ones close. AI didn't cause this — it accelerated a methodology failure that already existed in human-only teams. One beat, move on.

---

## SLIDE 4 — The acceleration whiplash · ~90s

### On slide

> Throughput went up. So did everything you don't want.

| Stat | Label |
| :---- | :---- |
| +33.7% | task throughput |
| +66.2% | epics completed |
| +54% | bugs per developer |
| +242.7% | incidents per merged PR |
| +861% | code churn |
| +441.5% | median PR review time |
| 31.3% | of code reached production with no review at all |

> "Strong pre-AI engineering maturity provides no protection." — Faros AI 2026, 22,000 developers across 4,000 teams

### Speaker script

> This is Faros AI's 2026 telemetry — two years of data, twenty-two thousand developers, four thousand teams. And critically, it's a within-team comparison: the same organizations' own low-AI-adoption quarters against their own high-AI-adoption quarters. Not company against company.
>
> Look at the two green numbers first — task throughput per developer up thirty-three point seven percent, epics completed up sixty-six point two percent. That's the story leadership wants to hear.
>
> Now the wall of red. Bugs per developer up fifty-four percent. Incidents per merged pull request up two hundred forty-two percent. Code churn — up eight hundred sixty-one percent. Median PR review time up four hundred forty-one percent. And thirty-one percent of code reaching production with no review at all. Faros's own line for this is: throughput measures what was shipped, not what survived.
>
> Here's the part that should change how you think about it. Faros split these same teams by how strong their engineering discipline already was before AI adoption — test coverage, review culture, CI rigor. The expectation is that disciplined teams absorb the shock better. They didn't. Teams doing everything right by 2024 standards hit the same wall as teams that weren't. That's not a hiring problem and it's not a process-tightening problem. It's structural — which is exactly what the rest of this talk exists to fix.

### Delivery notes

- Point at the two green stats, then the wall of red, in that order — the contrast is the point.
- Land "throughput measures what was shipped, not what survived," then the maturity finding, then pause before moving to slide 5.
- ~90 seconds — this is the evidence slide the whole talk leans on; don't rush it.

---

## SLIDE 5 — Why shift-left never worked · ~75s

### On slide

> The loop always terminated on a schedule, not a quality gate.

> Shift-left has been the stated goal of every quality and security movement for two decades. It never worked at scale because there was no mechanism strong enough to force the residual defect count to zero before review. **Sprints ended. Deadlines hit. Code shipped with known defects.**

annual pen test → release regression sweep → manual review → production

> AI-generated changes arrive continuously. This cadence does not.

### Speaker script

> Shift-left has been the stated goal of every quality and security movement for twenty years — catch problems earlier, closer to the point they're introduced. It never worked at scale, and the reason is structural, not motivational: there was never a mechanism strong enough to force the residual defect count to zero before a release went out the door. Sprints ended. Deadlines hit. Code shipped with known defects, and the exit condition was the calendar, not the quality bar.
>
> The old rhythm was built for human-paced change: an annual penetration test, a release regression sweep, manual review in between, then production. That rhythm was never perfect, but it was survivable when a handful of engineers produced a handful of pull requests a day.
>
> AI-generated changes arrive continuously. That cadence does not. And that gap is exactly what the Convergence Loop, later in this talk, is built to close — it's the first mechanism that actually changes the exit condition from "the sprint ended" to "residual defects reached zero."

### Delivery notes

- This sets up the Convergence Loop as the payoff — plant the phrase "exit condition" here; you'll return to it on slide 17.
- ~75 seconds.

---

## SLIDE 6 — Coverage is dead at agent velocity · ~75s

### On slide

> Coverage is dead at agent velocity.

**Run everything** — The full suite becomes the critical path. Slow feedback, queueing, weak attribution — and the cloud bill climbs on a duty cycle nobody budgeted for. You lose the speed you adopted agents to gain.

**Run too little** — Trust the unit tests, the agent summary, a plausible diff. Authorization gaps, cache-key leaks, unsafe CORS, malformed error paths escape into production.

> The answer is neither. It is **selective rigor**: the smallest credible corpus for this change, plus non-negotiable invariant floors.

### Speaker script

> At agent speed, teams make one of two mistakes.
>
> The first is to run everything, on every change. That gives you coverage, but the full suite becomes the critical path — slow feedback, queueing behind every other agent's diff, weak attribution when five things are running at once, and a cloud bill that climbs on a duty cycle nobody budgeted for. You lose the exact speed you adopted agents to gain.
>
> The second is to run too little — trust the unit tests, the agent's own summary of what it did, a diff that reads plausibly. That's where the real failures escape: an authorization gap, a cache-key leak, an unsafe CORS setting, a malformed error path. None of those show up in a green unit-test run.
>
> The answer is neither extreme. It's selective rigor — the smallest credible corpus for this specific change, plus a set of invariant floors that are never subject to selection. That phrase, smallest credible corpus, not smallest possible set, is the whole idea RADAR is built around, which is where we're headed next.

### Delivery notes

- Repeat "smallest credible corpus" verbatim — it's the bridge phrase into RADAR.
- ~75 seconds.

---

## SLIDE 7 — Gartner · ~45s

### On slide

> **40%** of agentic AI projects will be cancelled by 2027.

> Not from a lack of model capability. From methodological breakdown — review collapse, demo theater, inadequate specifications, ambition freeze. **Convergence is a system property, not a per-commit property.**
>
> Gartner, June 2026

### Speaker script

> Gartner, June 2026: forty percent of agentic AI projects will be cancelled by 2027.
>
> Not from a lack of model capability — the models keep getting better. From methodological breakdown: review collapse, demo theater, inadequate specifications, ambition freeze. Convergence is a system property, not something a better prompt buys you on a per-commit basis.
>
> Say that stat once, let it sit. This is a methodology problem before it's a capability problem — which is exactly the frame for what comes next.

### Delivery notes

- Say the stat once; don't over-explain the four failure modes listed — they're context, not the point.
- ~45 seconds.

---

## SLIDE 8 — The Agentic Development Model · ~90s

### On slide

> **The Agentic Development Model**
>
> An operating model, not a testing stage — one load-bearing mechanism at the center, the others arranged around it in the order a diff flows through them.

1. **Cross-Model Convergence Review** — a three-member cross-family LLM jury; findings converge into one owned action list.
2. **RADAR** — the risk-aware test selector; every test bound to a false-negative-rate target.
3. **The Convergence Loop** — eight steps that drive residual defects to zero before human review.

> Above the pipeline: a **commit-attested gate** that admits only diffs whose receipts prove the loop terminated cleanly.

### Speaker script

> This is the Agentic Development Model — ADM, if you hear me abbreviate it later. It's an operating model, not a testing stage bolted onto the old pipeline. There's one load-bearing mechanism at the center, and the other two are arranged around it in the order a single diff actually flows through them.
>
> First, Cross-Model Convergence Review — a three-member jury, one juror per model family, reviewing the diff. Their findings don't get majority-voted; they converge into one owned action list. Second, RADAR — the risk-aware test selector. Every test it runs is bound to a false-negative-rate target, not just a pass/fail. Third, the Convergence Loop — eight defined steps that drive residual defects to zero before a human ever looks at the diff.
>
> And above all three sits a commit-attested gate: it admits a commit only if its receipt proves the loop actually terminated cleanly on that exact diff — not on some earlier version of it.
>
> I'm naming all three once, right now, as a unit, because you're going to need this vocabulary for the rest of the talk. We'll go deep on RADAR first, then the Convergence Loop, then the CI-versus-nightly security split, then closure discipline — in that order, because that's the order a diff actually moves through the system.

### Delivery notes

- This is the vocabulary slide — say all three primitive names clearly, once, as a set. The audience needs to leave this slide able to recognize "RADAR" and "the Convergence Loop" by name for the rest of the talk.
- ~90 seconds — don't rush this one.

---

## SLIDE 9 — Act II·a divider (RADAR) · ~15s

### On slide

> Act II · a
>
> **RADAR**
> Risk-Aware Dependency Analysis for Rapid Verification

### Speaker script

> Act two, part one: RADAR — Risk-Aware Dependency Analysis for Rapid Verification.

### Delivery notes

- Let the sweep animation run for a beat before speaking. This is the deepest technical section of the talk — slow your pace down starting here.

---

## SLIDE 10 — What RADAR is · ~60s

### On slide

> RADAR
>
> Run exactly the right subset — with a defensible argument for why it is sufficient.

> Full-suite regressions on every agent-authored diff are neither fast enough to feed the loop nor cheap enough for any sane cloud budget. Skipping is what produces the bad-quality exposure that surfaces in production. RADAR is the selector that resolves both, *with declared floors no selector can undercut*.

> Not a random subset. Not just the dependency graph. A risk-weighted list of tests, each carrying the reason it was selected.

### Speaker script

> RADAR's core claim: run exactly the right subset of tests, with a defensible argument for why that subset is sufficient — not a guess, an argument you could show an auditor.
>
> Full-suite regression on every agent-authored diff is neither fast enough to feed a loop that needs to run repeatedly, nor cheap enough for any sane cloud budget. But skipping tests is exactly what lets the bad-quality exposure we saw in the Faros data reach production. RADAR is built to resolve both problems at once — with declared floors that no selector, however smart, is allowed to undercut. We'll get to those floors in a few slides.
>
> And to be clear about what RADAR is not: it's not a random subset, and it's not just a file-level dependency graph. A dependency graph misses indirect risk — a cache key can turn a perfectly correct query into a cross-tenant data leak, and no file-diff view will show you that. RADAR produces a risk-weighted list of tests, and every single one carries the reason it was selected.

### Delivery notes

- Contrast explicitly with a naive file-based selector — the cache-key example is the concrete failure mode that makes "reachability, not just files changed" land.
- ~60 seconds.

---

## SLIDE 11 — Blast radius across seven dimensions · ~90s

### On slide

> Blast radius across seven dimensions

1. file touched
2. module & package boundaries crossed
3. data structures read or written
4. API contracts exposed or consumed
5. runtime paths altered
6. historical failure correlations of adjacent code
7. system-level always-on invariants

> Output: a risk-weighted list — reason and dimension per test — kept legible in the run folder.

### Speaker script

> Here are the seven dimensions RADAR scores blast radius across. One: the file touched — the obvious starting point, but only the starting point. Two: module and package boundaries crossed. Three: data structures read or written. Four: API contracts exposed or consumed. Five: runtime paths altered. Six: historical failure correlations of adjacent code — has this neighborhood of the codebase been fragile before? And seven: system-level always-on invariants — properties that hold regardless of which file changed.
>
> The output isn't a single dependency-graph slice. It's a risk-weighted list of tests, and each one carries both the reason it was pulled in and which of these seven dimensions triggered it. That pairing — reason plus dimension — is what makes the selection legible enough to put in an audit trail, which is exactly where it ends up, in the run folder we'll look at in a couple of slides.

### Delivery notes

- Point at each ring on the diagram as you name the dimension — this is the technical meat of the RADAR section, don't compress it.
- ~90 seconds.

---

## SLIDE 12 — Every test carries a false-negative-rate target · ~75s

### On slide

> Every test carries a false-negative-rate target

> A false negative is a run that passes when the code is actually broken. Coverage never tracks this — it measures what a test *touches*, not what it *catches*. RADAR binds each test to an FNR target by priority tier, measured empirically over the test's history.

| tier | FNR target |
| :---- | :---- |
| critical invariant | ≈ 0 |
| high-blast-radius code | tighter |
| low-priority code | loosest |

> If a test drifts above its target, RADAR flags **the test** for repair — not the code it covers.

### Speaker script

> Here's the concept no other test-selection framework names explicitly: a false negative is a test run that passes while the code underneath it is actually broken. Traditional coverage metrics never track this — coverage tells you what a test *touches*, not what it *catches*. A test can execute every line of a function and still never assert the one thing that would have caught the bug.
>
> RADAR binds every test to a false-negative-rate target, set by priority tier and measured empirically against that test's own history. Critical-invariant tests get a target of approximately zero. High-blast-radius code gets a tighter target. Low-priority code gets the loosest target — RADAR isn't pretending everything deserves the same rigor.
>
> And here's the mechanism that makes this self-correcting: if a test's observed false-negative rate drifts above its target, RADAR doesn't quietly trust it anyway. It flags the test itself for repair — not the code it's supposed to be covering. The suite gets stronger over time instead of just accumulating more tests.

### Delivery notes

- This is the slide the technical/DevSecOps audience is paying closest attention to — it's a genuinely uncommon idea. Don't compress it.
- ~75 seconds.

---

## SLIDE 13 — Floors the selector may never undercut · ~60s

### On slide

> Floors the selector may never undercut

- no data loss on the primary write path
- no authorization bypass on any privileged operation
- no unbounded resource consumption in any request handler
- no silent corruption of persisted state

> Each critical invariant has anchor tests. RADAR is **forbidden** from selecting them away, whatever the blast-radius analysis says.

> Intelligence in test selection is welcome. Intelligence that lets a critical-invariant regression through is not.

### Speaker script

> Four floors, read slowly, because this is the slide the security side of the room is waiting for. No data loss on the primary write path. No authorization bypass on any privileged operation. No unbounded resource consumption in any request handler. No silent corruption of persisted state.
>
> Each of those has anchor tests behind it, and RADAR is forbidden — not discouraged, forbidden — from selecting those tests away, no matter what its blast-radius math concludes about a given diff. Intelligence in test selection is welcome. Intelligence that lets a critical-invariant regression through because the selector decided it was low-risk is not.

### Delivery notes

- Direct and concrete — this is the reassurance slide. Read each floor deliberately; don't rush through the list.
- ~60 seconds.

---

## SLIDE 14 — CLIP 1 · RADAR selection (scripted terminal) · ~60s

### On slide

Terminal (`node demo/radar/select.mjs --change 01-audit-mutation`), captured live from the real repo:

```
RADAR · Risk-Aware Dependency Analysis for Rapid Verification
change:  01-audit-mutation — consolidate audit enrichment to one row/action
touched: src/daemon/routes/enrich.ts

  test                          dimension                why
  ----------------------------------------------------------------------------
  [floor] audit-enrich-immutability   system-level invariant   append, never mutate the original
  [floor] audit-pipeline              system-level invariant   store is append-only
  [floor] policy-engine               system-level invariant   default-deny on unmatched actions
  [floor] mcp-gateway                 system-level invariant   denies unregistered servers
  [floor] approval-service            system-level invariant   break-glass needs a rationale
  [floor] rbac-regression             system-level invariant   role boundaries, daemon + hub

  selected corpus: 6 tests  (6 mandatory floors enforced, 0 by reachability)
  selection written to demo/run-folder/…__01-audit-mutation/selection.json
```

> Every test carries the reason it was selected and the dimension it covers. The floors are enforced whatever the blast-radius analysis says.

### Speaker script

> This is real output — hit play, and what you're watching is a captured recording of the actual selector, `node demo/radar/select.mjs`, run against Enforcer's own suite for the change we'll walk through live later.
>
> Look at the `[floor]` tag on every row, and the "why" column next to it. The selector isn't guessing — every test on this list carries the reason it was pulled in and the blast-radius dimension that triggered it. This particular change touches one file, the audit-enrichment route, and it lands entirely on floor-covered surface. So the six mandatory floors alone *are* the corpus here — RADAR isn't allowed to drop any of them, whatever the blast-radius analysis concludes.
>
> This is genuinely fast, by the way — about one second, no infrastructure required. Later in the talk we'll run this exact command live.

### Delivery notes

- Hit play, let it run, then narrate over or after — don't talk through the animation itself.
- ~60 seconds including the clip.

---

## SLIDE 15 — The run folder is the audit trail · ~60s

### On slide

> The run folder is the audit trail

```
# run-folder/<diff-hash>/
selection.json        — tests RADAR chose for this diff
reasons.json          — why each test, which dimension
fnr.json               — target vs observed rate, per test
anchors.json           — critical-invariant tests enforced
executor.log           — run traces
receipt                — hashed against the diff
```

> Six months later, an auditor reads not just **which tests ran** — but why each one was chosen.

### Speaker script

> Everything from the last three slides lands in one place: the run folder, keyed by diff hash. `selection.json` — the tests RADAR chose. `reasons.json` — why each one, and which dimension. `fnr.json` — the target versus the observed false-negative rate, per test. `anchors.json` — which critical-invariant tests were enforced as floors. An executor log with the run traces. And a receipt, hashed against the exact diff it covers.
>
> Six months from now, an auditor — or you, trying to remember why you shipped something — doesn't just see which tests ran. They see why each one was chosen. That's the difference between a CI log and an audit trail.

### Delivery notes

- This is the compliance/audit payoff slide for the RADAR section — keep it tight, it should land quickly after the density of slides 11–13.
- ~60 seconds.

---

## SLIDE 16 — Act II·b divider (The Convergence Loop) · ~15s

### On slide

> Act II · b
>
> **The Convergence Loop**
> The primitive that finally makes left-shift real rather than aspirational.

### Speaker script

> Act two, part two: the Convergence Loop — the primitive that finally makes left-shift real, rather than aspirational.

### Delivery notes

- The Convergence Loop is the primitive the other two orbit — it's what actually changes the exit condition we set up back on slide 5. Brief divider, move on.

---

## SLIDE 17 — Eight steps, in order · ~100s

### On slide

> Eight steps, in order — product code and test code both first-class

1. Agent hands off the diff **+ full context** — prompts, codebase, product and test diffs
2. Cross-family jury reviews in **two lanes** — product code, test code
3. Approved test-code changes built into the RADAR suite; **RADAR runs** → failures + gaps
4. Findings + failures → work-item list, **three lanes, one owner each**
5. Owners fix in parallel — no-rerun logs carry enough detail for **one-shot fixes**
6. Each owner runs the tests covering its items; confirms green
7. Jury **re-reviews the fixes** across all three lanes
8. Follow-up RADAR run + coverage check → clean = **terminate**; new issues = re-enter at step 2

### Speaker script

> Eight steps, walked through as a procedure. One: the agent hands off the diff with full context — not just the code, but the prompts that produced it, the surrounding codebase, and both the product diff and the test diff. Two: the cross-family jury reviews in two separate lanes, product code and test code, both first-class — a test-code regression is treated as seriously as a product-code one. Three: approved test-code changes get built into the RADAR suite, and RADAR runs, producing failures and coverage gaps. Four: findings plus failures become a work-item list split into three lanes, each with one named owner. Five: owners fix in parallel — and because the no-rerun logs from the failing run carry enough detail, most of these are one-shot fixes, not guess-and-check. Six: each owner runs the tests covering their own items and confirms green. Seven: the jury re-reviews the fixes across all three lanes — not just the original finder, everyone. And eight: a follow-up RADAR run plus coverage check. Clean means terminate. New issues mean you re-enter at step two, not step one — you don't restart the whole process, you loop back to jury review with the new information.
>
> That loop-back at step eight, the dashed arc on the diagram, is what makes this a convergence loop and not just a checklist — it's designed to run more than once on the same diff until nothing is left failing.

### Delivery notes

- Point at the ring diagram as you walk each numbered step. Emphasize step 2 (two lanes, both first-class), step 5 (one-shot fixes from no-rerun logs), and step 8 (the loop-back).
- ~100 seconds — this is a photographable slide; give it room.

---

## SLIDE 18 — The loop the old economics could not afford · ~60s

### On slide

> The Convergence Loop
>
> The loop the old economics could not afford

> In the human-paced world, iterating until residual defects reached zero was theoretically correct and **economically impossible** — each iteration cost days. Teams shipped with residual defects because the alternative was to never ship.

cost per iteration: human-days → agent-minutes + tokens

> It **has to** run in the agent-paced world — because leaving residual defects in a diff wastes the one scarce resource left: **human reviewer attention**.

### Speaker script

> Here's the economic argument for why this loop exists now and didn't exist before. In the human-paced world, iterating until residual defects reached zero was theoretically the right thing to do — and economically impossible, because each iteration cost days of someone's time. Teams shipped with known residual defects because the only alternative was to never ship at all.
>
> What changed is the cost per iteration: it moved from human-days to agent-minutes and tokens. That's not a process improvement, it's an order-of-magnitude change in unit economics — and it's why this loop has to run in the agent-paced world. Leaving residual defects in a diff now wastes the one resource that's actually still scarce: human reviewer attention. The loop exists to protect that attention, not to replace it.

### Delivery notes

- This is a genuine invention-of-the-agentic-era argument, not a rebrand of an old idea — say that explicitly if there's time.
- ~60 seconds.

---

## SLIDE 19 — The curve replaces coverage · ~60s

### On slide

> The Convergence Loop
>
> The curve replaces coverage

Convergence curve across four iterations: **2,094 → 445 → 10 → 0**

> What replaces coverage is the *shape* of this curve — how many iterations to zero, and what defect population resolved at each step.
>
> residual ratios 0.21 → 0.022 → 0.0 · one production convergence campaign · a multi-tenant B2B SaaS platform

### Speaker script

> Let the curve draw. This is real data from one production convergence campaign, on a multi-tenant B2B SaaS platform — two thousand ninety-four failures at iteration zero, down to four hundred forty-five after the first pass, ten after the second, zero after the third. Residual ratios: point two one, point zero two two, zero.
>
> Coverage is dead as a metric, as we said earlier — this is what replaces it. Not "what percentage of lines did we execute," but the shape of this curve: how many iterations it takes to reach zero, and what fraction of the defect population each iteration actually resolved.

### Delivery notes

- Let the SVG curve-draw animation finish before you speak the numbers — the visual and the narration should land together, not race each other.
- ~60 seconds.

---

## SLIDE 20 — CLIP 2 · the loop closing (scripted terminal) · ~60s

### On slide

Terminal (`node demo/radar/run.mjs`), captured live, showing the defect-to-fix cycle on Enforcer's real suite:

```
$ node demo/radar/run.mjs
  PASS  approval-service         20/20
  PASS  mcp-gateway              12/12
  PASS  policy-engine            16/16
  FAIL  audit-enrich-immutability 1/2
        ↳ leaves the original pending event byte-identical
  PASS  audit-pipeline           14/14
  PASS  rbac-regression           5/5
  VERDICT: FAIL   68 passed, 1 failed
  FIRST FAILURE: audit-enrich-immutability
  admissible_for_merge: false

$ demo/scripts/apply-change.sh 02-fix   # fix stacks on the diff
$ node demo/radar/run.mjs
  PASS  audit-enrich-immutability 3/3   # + the counter-example
  VERDICT: PASS   70 passed, 0 failed
  admissible_for_merge: true
```

> The receipt is bound to the change hash. A diff that no longer matches its receipt cannot pass the gate.

### Speaker script

> Another real recording — hit play. Top half: `node demo/radar/run.mjs` against the seeded defect we'll introduce live in the demo. Sixty-eight of sixty-nine green, one floor red — `audit-enrich-immutability` — verdict fail, and the receipt records `admissible_for_merge: false`.
>
> Bottom half: `demo/scripts/apply-change.sh 02-fix` stacks the fix on top of the same diff, no revert — that's the Convergence Loop in miniature. Re-run: the floor is now three-for-three, because the fix ships with a counter-example, not just a patch. Seventy of seventy passing. The receipt flips to `admissible_for_merge: true`.
>
> That receipt is bound to the change hash. A diff that no longer matches its receipt cannot pass the gate — you can't quietly re-edit the code after the fact and keep the old green stamp.

### Delivery notes

- This is the same defect (`audit-enrich-immutability`) you'll reproduce live in the real demo later — the audience should recognize it when it comes back.
- ~60 seconds including the clip.

---

## SLIDE 21 — Act II·c divider (CI vs. nightly) · ~15s

### On slide

> Act II · c
>
> **The security gate:**
> CI vs. nightly

### Speaker script

> Act two, part three: the security gate — CI versus nightly.

### Delivery notes

- Take the security-expert stance hardest starting here. Architecture first, testing second — that's the pivot the next slide makes explicit.

---

## SLIDE 22 — Remove the attack surface structurally · ~90s

### On slide

> Architecture first
>
> Remove the attack surface structurally — then automate proof that it stays removed.

- the developer cannot disable enforcement — managed hooks can't be removed, the daemon runs as root with keep-alive, toggling it needs an admin token
- the audit trail cannot be rewritten — no UPDATE/DELETE on stored events, enrichment appends a linked event, SHA-256 hash chain, signed exports
- a raw terminal doesn't bypass it — the kernel enforcer gates file-open / exec / connect syscalls from any process
- local policy can only tighten, never weaken — org → team → repo → local; Sentinel policy-mutation endpoints off by default
- every Hub mutation is RBAC-gated — reads operator+, approve reviewer+, policy writes admin only
- an unsigned policy bundle won't load — Ed25519 signatures verified before a policy takes effect

> Security is not purely a testing exercise. It is removing the class of failure, then verifying it stays gone.

### Speaker script

> These six items are Enforcer's own design guarantees, straight from its architecture docs — not test results, structural facts about the system. Walk through them with me, because each one is a class of attack removed by design, not caught by a test.
>
> The developer cannot disable enforcement — managed hooks can't be removed, the daemon runs as root with keep-alive, and toggling it off needs an admin token. The audit trail cannot be rewritten — no UPDATE or DELETE on stored events, ever; enrichment appends a linked event, chained with SHA-256 hashes, with signed exports. This one matters specifically because it's exactly what the live demo's seeded defect attacks — hold that thought. A raw terminal doesn't bypass any of this — the kernel enforcer gates file-open, exec, and connect syscalls from any process, not just ones that go through the app. Local policy can only tighten, never weaken, as it flows from org to team to repo to local machine. Every Hub mutation is RBAC-gated. And an unsigned policy bundle simply won't load — Ed25519 signatures are verified before any policy takes effect.
>
> The pivot: given all of that is true by architecture, what's left for testing to do is prove these guarantees actually hold on every single change. Security isn't purely a testing exercise — it's removing the class of failure first, then automating proof that it stays removed. That's the frame for the next few slides.

### Delivery notes

- Walk the list at a steady pace — this is six structural facts, not six test results, and the distinction matters to a security audience.
- Flag the audit-trail item explicitly as foreshadowing the live demo.
- ~90 seconds.

---

## SLIDE 23 — Whitebox on every commit. Blackbox nightly. · ~60s

### On slide

> Whitebox on every commit. Blackbox nightly.

**Whitebox — every commit** — source · config · build artifacts · container defs · dependency graph
`SQL-injection patterns` `XSS exposure` `secret leakage` `auth-coverage gaps` `unsafe shell execution` `container posture` `dependency risk`

**Blackbox — nightly** — the deployed API as a client or attacker sees it
`security headers` `method allowlists` `oversized payloads` `auth enforcement` `brute-force handling` `path traversal` `CORS behavior` `session-cookie attributes`

> Whitebox and blackbox are complements. A pass in one is not a waiver for the other.

### Speaker script

> The penetration suite splits into two halves. Whitebox, on every commit: it inspects source, config, build artifacts, container definitions, and the dependency graph. It looks for SQL-injection patterns, XSS exposure, secret leakage, gaps in auth coverage, unsafe shell execution, weak container posture, and dependency risk.
>
> Blackbox, nightly: it tests the deployed API the way a client or an attacker actually sees it — security headers, method allowlists, oversized payloads, auth enforcement, brute-force handling, path traversal, CORS behavior, session-cookie attributes.
>
> I'll say "about seventy-five checks total" once, right now, and not read exact counts off any slide after this. The point isn't the number — it's that whitebox and blackbox are complements. A pass in one is not a waiver for the other, and the next slide is about why they run on different schedules.

### Delivery notes

- Say the approximate total check count once here; don't repeat it elsewhere in the talk.
- ~60 seconds.

---

## SLIDE 24 — Why whitebox on commit, blackbox nightly · ~60s

### On slide

> Why whitebox on commit, blackbox nightly

**Whitebox = pattern-matching** — Deterministic, seconds, no infrastructure. A developer gets the verdict before the diff leaves their machine.

**Blackbox = live system** — Needs a running server, a real database, an HTTP listener. Per commit that is minutes of spin-up and contention across concurrent developers.

> Continuous — without pretending the same depth is economically sensible for every commit.

### Speaker script

> Why the split is by cadence, not by importance. Whitebox is pattern-matching against source — deterministic, takes seconds, needs no infrastructure. A developer gets the verdict before the diff even leaves their own machine.
>
> Blackbox needs a live system — a running server, a real database, an actual HTTP listener. Per commit, that's minutes of spin-up and teardown, and contention when multiple developers are shipping at once. Running it nightly gives you full coverage with zero developer friction.
>
> The system is continuous — security isn't deferred to an annual event — without pretending the same depth is economically sensible on every single commit.

### Delivery notes

- Cadence, not importance — that's the one distinction this slide needs to land.
- ~60 seconds; this can compress if you're behind schedule.

---

## SLIDE 25 — LIVE · the fast gate catches it (scripted terminal) · ~60s

### On slide

Terminal (`node demo/radar/gate-fast.mjs --change 01-audit-mutation`), captured live:

```
FAST GATES · 01-audit-mutation

  G1  tsc --noEmit ............ PASS
  G2  secret-leak scan ........ PASS
  G3  touched-module tests (6)  FAIL  1 failing
        FIRST FAILURE: audit-enrich-immutability.test.ts
        "leaves the original pending event byte-identical"

  ✓  G1  TypeScript
  ✓  G2  Secret scan
  ✗  G3  Touched-module tests

  FAST GATE VERDICT: FAIL   6.3s
```

> tsc and the secret scan are the whitebox-shaped checks — both green. The invariant floor is the layer that stops the diff.

### Speaker script

> The point of this recording: the security-shaped checks are green, and the diff still gets stopped. `node demo/radar/gate-fast.mjs --change 01-audit-mutation` — G1, the TypeScript check, passes. G2, the secret-leak scan, passes. G3, the touched-module tests, fails — first failure on `audit-enrich-immutability.test.ts`.
>
> The change compiles cleanly and introduces no secrets. A scanner-only gate — the kind most teams actually run — would wave this straight through. What catches it is the invariant floor: the audit trail's tamper-evidence guarantee, gated in the touched-module tests. This runs in about six seconds, with no infrastructure — we'll run this exact command live in a few minutes.

### Delivery notes

- This is the moment that proves architecture-plus-testing beats testing alone — pair it explicitly with slide 22's "audit trail cannot be rewritten" guarantee.
- ~60 seconds including the clip.

---

## SLIDE 26 — The catch that proves it works · ~45s

### On slide

> The catch that proves it works
>
> One query used quote-escaping instead of parameterization — with real user input.

> Not exploitable in the tested configuration. But it was the one place a future developer copying that pattern could introduce a real injection. We fixed it — and the *finding class* is now a permanent regression test.

> That specific mistake **can never ship again.**

### Speaker script

> One honest, concrete story. An analytics query in this codebase used quote-escaping instead of parameterization, on a path that took real user input. Not exploitable in the configuration it actually ran in — but it was the one place in the codebase where a future developer, copying that pattern into a new query, could have introduced a real SQL injection.
>
> We fixed it. More importantly, the finding class — not just this one query, the pattern itself — is now a permanent regression test. That specific mistake can never ship again, in this codebase, from any agent or any developer.

### Delivery notes

- Concrete, honest, low-drama delivery — this is the credibility beat of Act II·c, not a big reveal.
- ~45 seconds.

---

## SLIDE 27 — Act III divider (Closure discipline) · ~15s

### On slide

> Act III
>
> **Closure discipline**
> A dashboard full of green checks does not mean the test system is honest.

### Speaker script

> Act three: closure discipline. A dashboard full of green checks does not mean the test system is honest.

### Delivery notes

- Conceptual transition — slow down slightly here, the room needs a beat before the closure-discipline argument.

---

## SLIDE 28 — A failure is not resolved because a later run turns green · ~75s

### On slide

> A failure is not resolved because a later run turns green.

1. **Preserve the run artifact** — the evidence a reviewer relies on
2. **Investigate the first failure** in stable order — before changing ten things and re-running
3. **Add a counter-example** that would catch recurrence — the class, not the exact request
4. **Re-verify cross-agent** when the risk warrants it — a different family challenges the fix

### Speaker script

> Four requirements, and they're a real sequence — that's why they're numbered, not bulleted. One: preserve the run artifact. That's the evidence a reviewer actually relies on; if you don't keep it, you're asking for trust instead of offering evidence. Two: investigate the first failure, in stable order — before you change ten things at once and rerun the whole suite hoping something sticks. Three: add a counter-example that would catch recurrence — for the *class* of failure, not just the exact request that happened to trigger it today. Four: re-verify cross-agent when the risk warrants it — a different model family challenges the fix before you call it done.
>
> Rerunning until green is not closure. If a failure occurs and the team just reruns until it passes, the cause may still be unknown, and the same blind spot can reappear next week under a slightly different input.

### Delivery notes

- These four are a real sequence, not a list of options — say "one," "two," "three," "four" out loud as you go.
- ~75 seconds.

---

## SLIDE 29 — Three model families. No majority vote. · ~75s

### On slide

> Cross-Model Convergence Review
>
> Three model families. No majority vote.

> A single agent reviewing another from the same model family inherits the **same blind spots** as the code. A jury drawn one juror per family is the shape most likely to surface what any one would miss.
>
> Findings are not majority-voted — they converge into one merged action list where *every divergent finding carries an explicit owner*. The human adjudicates what the jurors cannot resolve.

> Jurors also route work to each other — observed handoffs, not a fixed taxonomy.

### Speaker script

> This is the jury from slide 8, in more depth. A single agent reviewing another from the same model family inherits the same blind spots as the code it's reviewing — same training biases, same failure modes, same things it's confidently wrong about. A jury with one juror drawn from each of three separate model families is the shape most likely to actually surface what any single one of them would miss.
>
> Critically, findings are not majority-voted. If you let the majority win, you lose the one juror who happened to be right and the other two happened to miss. Instead, findings converge into one merged action list, and every divergent finding carries an explicit named owner. The human adjudicates only what the jurors themselves can't resolve — not every finding, just the genuine disagreements.
>
> In practice the rotation looks something like: one family reviews another's work, that family's work gets reviewed by the third, and so on — jurors also route work to each other based on what they actually find, not a fixed assignment. That's observed behavior we've watched happen, not a taxonomy we designed in advance.

### Delivery notes

- Emphasize "no majority vote" as the counter-intuitive claim — most people's instinct is that voting is the safe choice; explain why it isn't here.
- ~75 seconds.

---

## SLIDE 30 — The evidence has firmed up quickly · ~60s

### On slide

> Why cross-family, not a bigger single judge
>
> The evidence has firmed up quickly

- Ensemble ceiling **~83%** above the best single model; naive consensus voting captures almost none of it; diversity-preserving aggregation realizes **~95%**. — arXiv 2510.21513
- A three-model panel across Anthropic, OpenAI, Google beat a single larger judge on six datasets at **~1/7 the cost**; same-family juries share correlated errors. — Verga et al., PoLL
- Cross-LLM verification on secure-code generation: **up to 47%** improvement over single-model baselines when the reviewer is from a different family. — arXiv 2603.22717

### Speaker script

> Three citations, because "trust me, diverse juries work better" isn't good enough for this room. First: the theoretical ensemble ceiling for LLM juries sits about eighty-three percent above the best single model — but naive majority voting captures almost none of that gain, while diversity-preserving aggregation, the approach the jury actually uses, realizes about ninety-five percent of it. That's arXiv 2510.21513.
>
> Second: a three-model panel spanning Anthropic, OpenAI, and Google beat a single larger judge model across six datasets, at roughly one-seventh the cost — because same-family juries share correlated errors, so adding a second instance of the same model buys you almost nothing. That's the PoLL study, Verga and colleagues.
>
> Third, and most directly relevant here: cross-LLM verification specifically on secure-code generation showed up to a forty-seven percent improvement over single-model baselines, when the reviewer came from a different model family than the author. That's arXiv 2603.22717.
>
> I'm citing these precisely because this isn't a marketing claim — it's a design decision backed by published research.

### Delivery notes

- Cite the arXiv numbers precisely; this is a credibility slide for a technically literate audience.
- ~60 seconds — can compress to the headline numbers only if short on time.

---

## SLIDE 31 — A diff that no longer matches its receipt cannot pass · ~60s

### On slide

> The gate
>
> A diff that no longer matches its receipt cannot pass.

> Most teams treat the CI run and the commit as two loosely coupled events — nothing binds the passing run to the change that shipped. At agent velocity, that is a load-bearing weakness.

```
commit trailer
Attested-Run: run-folder/a1b2c3d4
Diff-Hash:    a1b2c3d4   — matches staged diff
Verdict:      PASS

local hook rejects a commit with no receipt
server-side hook re-verifies against the pushed diff
```

### Speaker script

> Most teams treat the CI run and the commit as two loosely coupled events — a run passed, sometime, on something, and the commit merged, sometime, and nobody has formally bound the two together. At agent velocity, with an order of magnitude more commits landing per day, that loose coupling is a load-bearing weakness, not a theoretical one.
>
> The gate closes it: a commit trailer carries an attested run reference and a diff hash. If the diff hash in the trailer doesn't match the actual staged diff, or the verdict wasn't a pass, the commit is rejected — by a local hook first, and independently re-verified by a server-side hook on push, so a developer can't just skip the local check. A diff that no longer matches its receipt — because someone edited it after the fact — simply cannot pass.

### Delivery notes

- The two-hook detail (local rejects, server-side independently re-verifies) is the part that makes this credible to a security audience — don't drop it.
- ~60 seconds.

---

## SLIDE 32 — "Works" is not a test · ~60s

### On slide

> "Works" is not a test.

| Assertion | Counter-example |
| :---- | :---- |
| "Authorization works" | missing, expired, wrong-role, wrong-tenant credentials |
| "Cache is safe" | identical request across two tenants, workspaces, date ranges |
| "Contract is compatible" | existing client fixture against the changed deployment |
| "Input validation works" | malformed, oversized, encoded, boundary inputs |

> The point of a test is not to confirm the story you already believe. It is to create evidence that could disprove it.

### Speaker script

> Walk the table with me. Each left-hand cell is a story an agent — or a developer — believes about their own code. Each right-hand cell is the evidence that could actually disprove it.
>
> "Authorization works" is not a test; missing, expired, wrong-role, and wrong-tenant credentials, each independently tried, are tests. "The cache is safe" is not a test; an identical request replayed across two different tenants, workspaces, and date ranges is a test. "The contract is compatible" is not a test; running the existing client fixture against the changed deployment is a test. "Input validation works" is not a test; malformed, oversized, encoded, and boundary inputs are tests.
>
> The point of a test is not to confirm the story you already believe about your own code. It's to create evidence that could disprove it. And in a few minutes, in the live demo, you'll watch this exact pattern play out for real: the audit-immutability invariant fails, and the fix that closes it ships with a genuine counter-example, not just a patch.

### Delivery notes

- The forward reference is to the *real* demo defect — audit-enrich-immutability — not a cache-key bug. (An earlier draft of this talk used a cache-key/tenant-isolation demo; that was replaced when the demo was rebuilt against Enforcer's own repo. If you're working from an older printout of the deck's notes panel, disregard any reference there to a cache-key demo.)
- ~60 seconds.

---

## SLIDE 33 — Automate the work. Keep the sign-off human. · ~90s

### On slide

> The human checkpoint
>
> Automate the work. Keep the sign-off human.

> The pipeline does the repeatable coverage — selection, gates, the convergence loop, the security suite — and produces the evidence. What it does not do is *decide*: a human owns risk acceptance, business authorization, customer-impact trade-offs, and final approval of any high-blast-radius or irreversible production action. Not a refusal to automate — a mandatory review-and-approve step on the calls that carry liability.

> Some testing still needs human creativity — reconnaissance, timing attacks, workflow-bypass reasoning, manual business-logic probing. The automated suite reports what changed since the last human engagement; the engagement finds what a script would not think to try. Both run; neither replaces the other.

> Ford automated quality inspection and cut headcount — then rehired ~350 veteran engineers to rebuild the data pipelines feeding its AI, and topped J.D. Power's 2026 Initial Quality Study, its first since 2010. **The judgment left before it was encoded; the tools amplified weak inputs instead of catching flaws.**

### Speaker script

> Frame this precisely: automate the work, keep the sign-off human. Not "we refuse to automate" — the opposite. The pipeline does all the repeatable coverage: selection, gates, the Convergence Loop, the security suite, and it produces the evidence. What it deliberately does not do is decide. A human still owns risk acceptance, business authorization, customer-impact trade-offs, and final approval on anything high-blast-radius or irreversible in production. That's a mandatory review-and-approve step on the calls that carry real liability, not a gap in the automation.
>
> Separately — some testing still genuinely needs human creativity: reconnaissance, timing attacks, workflow-bypass reasoning, manual business-logic probing. The automated suite reports what's changed since the last human engagement; the human engagement finds what a script would never think to try. Both run. Neither replaces the other.
>
> I'll close this slide with Ford. Ford automated quality inspection and cut headcount to save money. It didn't work well enough — the results cost them billions — so they rehired roughly three hundred fifty veteran engineers, internally called the "gray beards," whose job now is mentoring juniors and rebuilding the training-data pipelines that feed their AI systems. Ford then topped J.D. Power's 2026 U.S. Initial Quality Study — their first number-one finish since 2010.
>
> The lesson isn't "AI failed." It's that the experienced people left before their judgment was actually encoded into the system, so the tools ended up amplifying weak inputs instead of catching design flaws. That's the whole argument for this slide, in one story.

### Delivery notes

- Source: Assembly Magazine, "Ford Rehires Veteran Engineers to Improve AI, Vehicle Quality" (June 2026); also covered by Fox Business, TNW, Forbes.
- Land the Ford story as the closing parable for Act III — don't rush the "judgment left before it was encoded" line, it's the thesis sentence.
- ~90 seconds.

---

## SLIDE 34 — Live demo setup · ~40s

### On slide

> Live demo
>
> One AI-generated pull request against Enforcer itself.

```
target      Enforcer — the security & policy control plane (this repo)
change      "consolidate audit enrichment to one row per action"
touched     src/daemon/routes/enrich.ts   — 1 file, small diff
defect      the pending audit event is updated in place
            instead of a new linked enrichment row being appended
invariant   no silent corruption of persisted state
            — "what was attempted" and "what happened" are separate rows
```

> tsc clean · no secrets · every unit test green · one invariant floor red · runs in ~7s, no infrastructure.

### Speaker script

> The target for the live demo is Enforcer itself — the security and policy control plane this entire repository is. An agent has opened a pull request against it: one file, `src/daemon/routes/enrich.ts`, a small diff, with a commit message that reads like a perfectly reasonable refactor — "the pending event already has the full action context, so appending a second row is redundant; just fill in what happened." It compiles clean. It introduces no secrets. Almost every unit test stays green.
>
> Watch what stops it anyway.

### Delivery notes

- Move to the real demo environment right after this slide — DEMO_SCRIPT.md's Beat 0 framing (below) picks up from here almost verbatim.
- ~30–40 seconds; keep this slide up as a visual fallback while you switch inputs to the terminal.

---

## SLIDE 35 — baseline → PR lands → gates → floor fails → fix → receipt · ~30s

### On slide

> baseline → PR lands → gates → floor fails → fix → receipt

```
$ node demo/radar/gate-fast.mjs                     G1 G2 G3 PASS   clean baseline
$ demo/scripts/apply-change.sh 01-audit-mutation     the AI-generated PR lands
$ node demo/radar/gate-fast.mjs --change 01-…        G1 PASS  G2 PASS  G3 FAIL
$ node demo/radar/select.mjs   --change 01-…         6 floors, reason + dimension each
$ node demo/radar/run.mjs                            VERDICT FAIL · FIRST FAILURE
                                                      admissible_for_merge: false
$ demo/scripts/apply-change.sh 02-fix                fix + counter-example, stacked
$ node demo/radar/run.mjs                            VERDICT PASS 70/70
                                                      receipt → admissible_for_merge: true
$ demo/scripts/revert.sh                             back to a clean tree
```

### Speaker script

> This is the runbook — the exact sequence we're about to run for real. Clean baseline, the PR lands, the fast gate catches it, the selector shows its reasoning, the full run fails on the invariant floor, the fix stacks on top of the same diff, it re-runs clean, and the receipt flips from inadmissible to admissible for merge. If anything about the live terminal goes sideways, this slide — and the captured clips on slides 14, 20, and 25 — are the fallback.

### Delivery notes

- This slide is the fallback reference during the live demo, not something you narrate line-by-line before switching over — get to the terminal quickly.
- ~30 seconds, then switch inputs.

---

## SLIDE 36 — Live demo placeholder · (stays on screen for the whole live demo)

### On slide

> Live demo
>
> **Demo.**
>
> demo/scripts/run-demo.sh — press Enter to advance each beat · fallback clips: slides 14 / 20 / 25

### Speaker script

> Nothing to say over this slide — it's not narrated. Advance to it right after slide 35, switch your display input (or alt-tab) to the terminal, and run the live demo below. When the demo closes, switch back — this slide will still be on screen — and advance into Act IV.

### Delivery notes

- This slide exists purely so nothing changes on the projected screen while you're heads-down in the terminal — the audience isn't staring at your desktop or a slide transition mid-demo.
- Leave it up for the full ~7 minutes of the live demo below, then advance straight to slide 37 (Act IV) once you're done.

---

# Live demo script

*This section is reproduced from `demo/DEMO_SCRIPT.md`, the repo's own authoritative, tested demo script — kept in sync with it rather than duplicated with drift. If the two ever disagree, `demo/DEMO_SCRIPT.md` is the source of truth; update this section to match it, not the other way around.*

## Demo goal

Run the real Enforcer Vitest suite live, against a genuinely seeded defect and a genuine fix, so every PASS/FAIL the audience sees on screen is real output, not a recording — except where a captured fallback (slides 14, 20, 25) is used to de-risk timing. No infrastructure required: no Postgres, no Go, no daemon. Whole cycle ≈ 30 seconds of command time; ~7 minutes narrated.

Run everything from the repo root (`src/`). Font size up, one terminal, one editor showing `src/daemon/routes/enrich.ts`.

You don't type the commands live — run the driver and press **Enter** to advance each beat:

```bash
demo/scripts/run-demo.sh
```

It walks the seven beats below, pausing for a keystroke before each command, printing everything verbose (the diff, the receipt JSON, `git status`). **Ctrl-C** at any point aborts and restores a clean tree.

Pre-flight self-test — run straight through with no pauses:

```bash
demo/scripts/run-demo.sh --auto
```

If anything stalls: deck slides **14 / 20 / 25** carry the captured output — hit **play**. Do not debug live.

## Pre-flight (before the session)

```bash
cd /path/to/repo
npm ci                        # once
npm test                      # sanity: 152 passing, ~4s
demo/scripts/revert.sh        # ensure a clean tree (no stale overlay)
git status --porcelain        # expect empty
```

- [ ] `node demo/radar/gate-fast.mjs` → FAST GATE VERDICT: PASS
- [ ] Editor open at `src/daemon/routes/enrich.ts` — read-only; don't save it
- [ ] Terminal font large; window wide enough for ~80 columns
- [ ] Deck open at slide 34 (demo setup)
- [ ] No other terminal or editor is going to write to this repo during the run. The driver applies and reverts `enrich.ts` as it goes; if anything else edits or reverts that file mid-run, `run-demo.sh` will stop itself with a red `UNEXPECTED` message rather than narrate a stale result.

## Beat 0 — framing · ~20s · (slide 34 on screen)

> "The target here is Enforcer itself — the policy-and-audit control plane this whole repo is. An agent has opened a pull request against it. One file. It compiles, it introduces no secrets, and almost every unit test stays green. Watch what stops it."

## Beat 1 — clean baseline · ~35s

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

> "Clean tree. Three fast gates — typecheck, a secret-leak scan, and the touched-module tests. All green. This is the state a good PR should preserve."

## Beat 2 — the AI-generated PR lands · ~40s

**Type:**
```bash
demo/scripts/apply-change.sh 01-audit-mutation
git diff --stat
```
**Appears:** one file changed — `src/daemon/routes/enrich.ts`.

Show the diff in the editor. The change replaces the append-only enrichment path with: *find the pending audit event, write the outcome onto it in place, re-store it.*

> "The commit message is reasonable: 'the pending event already has the full action context, so appending a second row is redundant — just fill in what happened.' It reads like a tidy refactor — one file. The response body even still returns `append_only: true`."

## Beat 3 — the fast gate catches it · ~55s · (advance deck to slide 25)

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

> "It compiles. No secrets. A scanner-only gate would wave this through. What fails is G3 — the invariant floor `audit-enrich-immutability`. The audit trail's whole tamper-evidence guarantee is: an event, once written, is never mutated. This change mutates it."

## Beat 3.5 — how this actually gets fixed · ~30s

> "Before we look at what runs next: in the full pipeline, a failure like this doesn't go straight to a human. Several independent AI models each look at it on their own — observe, analyze, propose a fix — without seeing each other's answer. They then cross-review and converge on one fix. Only then does the system pick which tests that fix needs to be checked against, and the whole loop repeats until nothing is failing. We're skipping straight to the converged fix so we can walk through the mechanics in the time we have."

## Beat 4 — RADAR selection · ~50s · (advance deck to slide 14)

**Type:**
```bash
node demo/radar/select.mjs --change 01-audit-mutation
```
**Appears:** six `[floor]` rows, each with a *reason* and a *dimension*, then `selected corpus: 6 test files (6 mandatory floors enforced, 0 by reachability)`, followed by `6 of 10 test files selected (69 of 152 individual tests in the whole suite)`.

> "The selector isn't guessing. Every test on the list carries the reason it was pulled in and the blast-radius dimension it covers. This change touches one file and it lands entirely on floor-covered surface — so the six mandatory floors *are* the corpus. RADAR is forbidden from dropping any of them, whatever the blast-radius math says. And look at the bottom line: six of the repo's ten test files, about sixty-nine of its hundred fifty-two individual tests. Everything else got left out on purpose, not by luck — nothing about it could plausibly be touched by this change."

## Beat 5 — the run and the verdict · ~45s · (advance deck to slide 20)

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

> "Sixty-eight of sixty-nine green — and it doesn't matter. One floor red is a hard stop. The run folder now has a receipt: `admissible_for_merge: false`, hashed against this exact change."

## Beat 6 — the fix, stacked on the diff, re-run · ~75s

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

> "The fix restores the append-only path — and it ships with a counter-example: it freezes the original event so any in-place write is now a thrown error, not a silent field change, and it proves repeated enrichment only ever appends. Re-run against the same diff: seventy of seventy. The receipt flips to `admissible_for_merge: true`. That's the Convergence Loop — the fix lands on the diff, the corpus re-runs, the residual goes to zero before a human ever looks at it."

## Beat 7 — cleanup + the line · ~25s

**Type:**
```bash
demo/scripts/revert.sh
```

> "Back to a clean tree. The takeaway: a tamper-evidence regression that compiled clean and passed the secret scan was stopped by an invariant floor the selector isn't allowed to skip — before it reached review."

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
2. Beat 3.5 (how the fix converges) — one sentence, or skip: "this gets fixed by a converged, cross-reviewed AI proposal — we're jumping to that fix."
3. Beat 4 (`select`) — the six floors are visible on slide 14; describe, don't run.
4. Beat 2's `git diff --stat` — just narrate the one-file change.

**Never cut:** the gate FAIL with its FIRST FAILURE line (Beat 3), and the `admissible_for_merge: false → true` flip across Beats 5–6.

## Recovery

- Command hangs or errors → switch to the deck, hit **play** on the matching scripted terminal (14 = select, 20 = run defect→fix, 25 = gate-fast), narrate over it.
- `apply-change.sh` refuses ("working tree is dirty") → `demo/scripts/revert.sh`, then `git stash` any unrelated edits, retry.
- Wrong overlay stuck → `demo/scripts/revert.sh` unwinds the whole stack.
- `run-demo.sh` stops with a red **UNEXPECTED** message → it detected the demo state was wrong (baseline gate failing, or the seeded change not triggering a test failure) and refused to narrate something untrue. It has already reverted to a clean tree; run `git status` (expect only unrelated edits), then re-run.

---

## SLIDE 37 — Act IV divider (Take home) · ~10s

### On slide

> Act IV
>
> **Take home**

### Speaker script

> Act four: take home.

### Delivery notes

- Short. One beat, move.

---

## SLIDE 38 — If you can't name your guardrails · ~60s

### On slide

> Three things
>
> If you can't name your guardrails, you're not doing AI governance.

1. **The layered model** — fast gates (~3 min) catch most regressions; RADAR impact-selects the rest; the Convergence Loop drives residuals to zero before review.
2. **The split** — CI = whitebox: deterministic, fast, no infrastructure. Nightly = blackbox: live server, full attack simulation. Know which is which.
3. **Closure discipline** — every test an FNR target, every critical invariant a mandatory floor, every commit a receipt bound to its diff, every fix a statement of what would prove it wrong.

### Speaker script

> Three photographable points. One: the layered model — fast gates, under three minutes, catch most regressions; RADAR impact-selects the risk-weighted rest; the Convergence Loop drives what's left to zero before a human ever reviews it. Two: the split — CI is whitebox, deterministic, fast, no infrastructure; nightly is blackbox, a live server, full attack simulation. Know which is which and why, because they answer different questions. Three: closure discipline — every test carries a false-negative-rate target, every critical invariant has a mandatory floor, every commit carries a receipt bound to its exact diff, and every fix states what would prove it wrong.
>
> If you can't name your guardrails, you're not doing AI governance — you're funding next year's cancelled project.

### Delivery notes

- Land each numbered item distinctly, then stop on the closing line — don't soften it or explain it further.
- ~60 seconds.

---

## SLIDE 39 — Questions · ~20s + open floor

### On slide

> Start small
>
> one repository · one invariant suite · one run artifact · one class of AI-generated change
>
> agenticfactory.ai — the framework / deepankardas.substack.com — the three-part series / OWASP WSTG coverage matrix / "A CISO Operating Model for Human and Agent Coworking" — the governance layer
>
> **Questions.**
>
> What sensitive API surface would you pilot first?

### Speaker script

> Start small: one repository, one invariant suite, one run artifact, one class of AI-generated change. The framework is at agenticfactory.ai; the three-part series that walks through every primitive in full detail — the diagnosis, the manual, and multi-agent coordination at scale — is at deepankardas.substack.com; there's an OWASP WSTG coverage matrix if you want to map this against your own existing testing standard; and if you're the one who has to answer for all of this organizationally, not just technically, there's a companion piece — "A CISO Operating Model for Human and Agent Coworking" — that covers the governance layer this talk didn't have time for.
>
> Questions. And to open it up: what sensitive API surface would you pilot this on first?

### Q&A prep

| Likely question | Suggested answer |
| :---- | :---- |
| Why not run the full suite every change? | Too slow, and too weakly tied to the precise change to give clean attribution. Reserve full-suite runs for milestones; per-change verification needs fast, risk-aware, attributable feedback instead. |
| How do you know the selected corpus is sufficient? | No single mechanism claims that alone. It's reachability, plus mandatory floors, plus risk-mandated tests, plus milestone validation, plus a learning loop that turns escaped failures into new tests or new floors. |
| Does this replace penetration testing? | No — it makes security validation continuous and change-aware. Broader human-led penetration testing and periodic third-party engagement remain important milestone-level controls. |
| Does this work without multiple AI agents? | Yes. The core is layered verification, impact selection, run artifacts, and closure discipline. Cross-model review is an additional independent-challenge path available specifically to agentic workflows — not a prerequisite for the rest. |
| What stays human? | Not a list of things you refuse to automate. Automate the work, but keep a mandatory human review-and-approve step on risk acceptance, business authorization, customer-impact trade-offs, ambiguous incident calls, and final approval of any high-blast-radius or irreversible production action. |
| What's the smallest way to start? | Pick one sensitive API module, name its invariants explicitly, start preserving run artifacts, require a counter-example for every fix, and record a simple change-to-test-selection rationale in pull requests. |

---

# Presenter checklist

## Before the session

- [ ] Confirm exact room AV and display inputs.
- [ ] Increase terminal and editor font size.
- [ ] Disable notifications, VPN popups, auto-updates, and sleep mode.
- [ ] `npm ci` once; `npm test` — confirm 152 passing, ~4s.
- [ ] `demo/scripts/revert.sh` — confirm a clean tree (`git status --porcelain` empty).
- [ ] `node demo/radar/gate-fast.mjs` — confirm FAST GATE VERDICT: PASS on the clean baseline.
- [ ] Run `demo/scripts/run-demo.sh --auto` once as a full pre-flight self-test.
- [ ] Editor open, read-only, at `src/daemon/routes/enrich.ts`.
- [ ] Deck open at slide 1, browser/viewer fullscreened, speaker-notes panel tested (press `S`).
- [ ] Copy every demo command to a local scratch file — nothing typed from memory live.
- [ ] Have local screenshots or recordings of every critical terminal output as a last-resort fallback beyond slides 14/20/25.

## During the session

- [ ] Reach slide 34 (demo setup) on schedule; advance through 35 to 36 (the "switch to the terminal" placeholder), then move to the terminal — leave 36 on screen for the whole live demo.
- [ ] Preserve the failure (Beat 3/5) before showing the fix (Beat 6) — never skip straight to green.
- [ ] If any live command stalls or errors, switch to the deck and hit play on the matching captured clip (14/20/25) rather than debugging on stage.
- [ ] Leave real time for Q&A — don't let the live demo run long at its expense.

---

*Securing AI-Generated APIs at the Speed of AI — slide deck and demo script, v1.1. Rewritten to match `docs/AITechWorld_APIWorld_2026_Workshop_Deck.html` (39 slides) and `demo/DEMO_SCRIPT.md`. Supersedes v0.1.*
