# Enforcer convergence demo

The live-demo harness for the API World 2026 workshop. It runs the Agentic
Development Model pipeline — fast gates, RADAR risk-aware selection with
mandatory floors, run-folder receipts — against **Enforcer's real Vitest
suite**. The PASS / FAIL results are genuine; the selector and gate scripts
are a teaching implementation sized for a 15-minute stage slot.

No infrastructure required — the TypeScript layer runs fully in-memory
(`npm test` needs no Postgres, no Go, no daemon).

## Layout

```
demo/
  radar/
    floors.json        mandatory floors — anchor tests RADAR may never drop
    map.json           reachability map: source path -> tests + blast dimension
    lib.mjs            shared helpers
    select.mjs         print the risk-weighted corpus for a change
    run.mjs            run the selected corpus -> verdict + first failure + receipt
    gate-fast.mjs      G1 tsc · G2 secret scan · G3 touched-module tests
  changes/
    TEMPLATE/          copy to changes/<name>/ ; overlay/ holds the modified files
  scripts/
    run-demo.sh              presenter driver — run it, press Enter per beat
    apply-change.sh <name>   copy an overlay onto the working tree
    revert.sh                restore it
  run-folder/          produced selections, run.json, receipts   (gitignored)
```

**Presenting?** Run `demo/scripts/run-demo.sh` and press Enter to advance each
beat — no typing. `--auto` runs it straight through as a pre-flight check. The
narration and expected output for every beat are in [`DEMO_SCRIPT.md`](DEMO_SCRIPT.md).

## Stage runbook (what the driver runs)

```bash
# 0. clean tree, floors green
node demo/radar/gate-fast.mjs                            # G1/G2/G3 PASS
node demo/radar/select.mjs                               # 6 mandatory floors, clean baseline
node demo/radar/run.mjs                                  # VERDICT: PASS

# 1. the AI-generated PR lands
demo/scripts/apply-change.sh 01-audit-mutation
node demo/radar/gate-fast.mjs --change 01-audit-mutation # G1/G2 PASS ... G3 FAIL
node demo/radar/select.mjs --change 01-audit-mutation    # floors + reachability, reasons per test
node demo/radar/run.mjs                                  # VERDICT: FAIL — FIRST FAILURE on the audit-immutability floor

# 2. fix stacks on top of the diff, re-run (the Convergence Loop — no revert)
demo/scripts/apply-change.sh 02-fix
node demo/radar/run.mjs                                  # VERDICT: PASS — receipt: admissible_for_merge: true

# 3. clean up
demo/scripts/revert.sh                                   # unwinds 02-fix then 01-audit-mutation
```

## The seeded defect

`01-audit-mutation` changes `src/daemon/routes/enrich.ts` so the enrichment
endpoint updates the original pending audit event in place instead of appending
a linked enrichment row. Every unit test still passes — but the mandatory floor
`tests/integration/audit-enrich-immutability.test.ts` goes red: the audit
trail's tamper-evidence guarantee (`no silent corruption of persisted state`)
is broken. RADAR is forbidden from selecting that floor away, so the fast gate
and the selected run both catch it before the diff reaches a human.

## Status

- **Stage 1 (this)** — harness + the `audit-enrich-immutability` floor + baseline green.
- Stage 2 — `changes/01-audit-mutation` overlay.
- Stage 3 — `changes/02-fix` + counter-example, receipt to `admissible_for_merge`.
- Stage 4 — in-deck scripted terminal (slides 14 / 20 / 25) matched to these runs.
