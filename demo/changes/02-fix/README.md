# 02-fix

The fix for the `01-audit-mutation` incident, plus the counter-example.

## What it does

**`src/daemon/routes/enrich.ts`** — restored to append-only. The endpoint again
builds a *new* enrichment event with its own `event_id`, links it to the pending
event by `correlation_id`, and stores it. The original event is never touched.

**`tests/integration/audit-enrich-immutability.test.ts`** — adds a counter-example
that catches the *class* of defect, not the one assertion that happened to fail
first:

- The stored original event and its nested `action` / `policy_detail` are
  `Object.freeze`d. Under ES-module strict mode, any in-place assignment now
  throws a `TypeError` — mutation becomes a hard, immediate failure.
- Enrichment is called twice; the store must end at exactly three rows
  (original + two appended enrichments), with two events linked back to the
  original by `correlation_id`.

The mutating version of the endpoint fails this test by *throwing*; the correct
version passes it cleanly.

## Stage flow

```bash
demo/scripts/apply-change.sh 01-audit-mutation   # defect
node demo/radar/run.mjs                          # VERDICT: FAIL

demo/scripts/apply-change.sh 02-fix              # fix stacks on top of the defect
node demo/radar/run.mjs                          # VERDICT: PASS — admissible_for_merge: true

demo/scripts/revert.sh                           # unwind both, back to a clean tree
```
