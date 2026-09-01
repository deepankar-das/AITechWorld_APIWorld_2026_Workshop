---
name: adm-radar
description: >
  AgenticDevelopmentModel RADAR execution, failure analysis, and restart-gate procedures. Use
  when running RADAR, classifying failures, deciding whether a new full run is
  justified, or extracting actionable failure buckets from test logs.
---
> **AgenticDevelopmentModel**
> Copyright © 2026 AgenticDevelopmentModel Inc. Proprietary and Confidential.
>
> Author: Deepankar Das

# AgenticDevelopmentModel RADAR Operations

## When to use

Use this skill for the `<radar script>`, run-log triage, failure-bucket classification, or decisions about smoke vs matched-load validation.

## Workflow

1. Prefer RADAR entrypoints over raw `vitest` or `playwright` commands.
2. Classify failures by causal bucket, not failing-spec count.
3. Patch the highest-blast-radius bucket first.
4. Do not request a new full run until blocker metrics materially improve.

## Core commands

- `<radar script> --quick`
- `<radar script>`
- `<radar script> --all`
- `<radar script> --exit-on-fail`

## References

Read `references/restart-gates.md` before requesting another full RADAR run.
