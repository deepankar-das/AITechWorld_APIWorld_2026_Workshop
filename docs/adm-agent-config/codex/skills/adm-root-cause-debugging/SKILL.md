---
name: adm-root-cause-debugging
description: >
  AgenticDevelopmentModel root-cause-first debugging, failure-bucket classification, and
  evidence-driven remediation planning. Use when a bug, run failure, deploy
  regression, or repeated test failure needs causal analysis rather than
  spec-count triage.
---
> **AgenticDevelopmentModel**
> Copyright © 2026 AgenticDevelopmentModel Inc. Proprietary and Confidential.
>
> Author: Deepankar Das

# AgenticDevelopmentModel Root-Cause Debugging

## When to use

Use this skill for production bugs, test cascades, deploy failures, or repeated regression clusters where the right move is causal-bucket analysis before patching.

## Workflow

1. Aggregate failures across the full artifact, not just the first few lines.
2. Classify by cause: auth/session, render/hydration, pool pressure, schema/data contract, stale test contract, deploy/startup, or environment.
3. Patch the highest-blast-radius cause first.
4. Define one counter-example that would refute closure and run it before closing the bucket.

## References

Read `references/bucket-classification.md` when classifying failures.
