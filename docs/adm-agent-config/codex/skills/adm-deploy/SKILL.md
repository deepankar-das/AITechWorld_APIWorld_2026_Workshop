---
name: adm-deploy
description: >
  AgenticDevelopmentModel build, deploy, seeding, migration, and Cloud Run operations.
  Use when building images, deploying to staging or production, running seed or
  migrate flows, checking GCP logs, or debugging build and deploy scripts.
---
> **AgenticDevelopmentModel**
> Copyright © 2026 AgenticDevelopmentModel Inc. Proprietary and Confidential.
>
> Author: Deepankar Das

# AgenticDevelopmentModel Deploy Workflow

## When to use

Use this skill for the `<build script>`, `<deploy script>`, Cloud Run, seed jobs, migration runs, log inspection, or GCP project isolation checks.

## Workflow

1. Confirm the target project and environment explicitly.
2. If schema changed, build first, then deploy with migration.
3. Use repository scripts only; never start deployment flows with bare server commands.
4. For smoke or post-seed failures, inspect readiness and service logs before claiming product failure.

## Core commands

- `<build script> --env staging`
- `<deploy script> --env staging --migrate`
- `gcloud run services logs read <service> --region <region> --project <build-project>`

## References

Read `references/deploy-flags-and-logs.md` for flag meanings, project isolation, and log commands.
