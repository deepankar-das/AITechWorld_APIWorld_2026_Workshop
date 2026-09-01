# 01-audit-mutation

The AI-generated pull request for the live demo.

## What the change claims

"Consolidate the audit enrichment path. Today every executed action produces
two rows — a `pending` event and a linked enrichment event. The pending event
already has the full action context, so the second row is redundant. This
updates `observed_effect` on the pending event directly: one row per action,
half the storage, simpler replay queries."

It compiles cleanly (`tsc --noEmit` passes), the secret scan is clean, and every
unit test that doesn't touch `handleEnrich` stays green. The diff is small and
reads as a reasonable refactor.

## What it actually breaks

The audit trail's tamper-evidence rests on one invariant: **an event, once
written, is never mutated.** "What was attempted" and "what actually happened"
are separate rows so that neither can be silently rewritten after the fact.

This change mutates the original `pending` event in place. The invariant floor
`tests/integration/audit-enrich-immutability.test.ts` catches it:

```
FIRST FAILURE: tests/integration/audit-enrich-immutability.test.ts
  Audit Enrichment Immutability (invariant floor)
  › appends an enrichment event and leaves the original pending event byte-identical
```

RADAR is forbidden from selecting that floor away, so both `gate-fast` (G3) and
the selected `run` surface it before the diff reaches a human.

The response body still returns `append_only: true` — the code's own claim about
itself is now false.

## Fix

`demo/changes/02-fix/` restores the append-only enrichment event and adds a
sharper counter-example. See `demo/README.md` for the stage runbook.
