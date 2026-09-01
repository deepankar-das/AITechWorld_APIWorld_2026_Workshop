---
name: adm-schema-change
description: >
  AgenticDevelopmentModel platform and tenant schema-change rules. Use when adding or changing
  tables, columns, indexes, DDL, bootstrap parity, or tenant schema
  provisioning logic.
---
> **AgenticDevelopmentModel**
> Copyright © 2026 AgenticDevelopmentModel Inc. Proprietary and Confidential.
>
> Author: Deepankar Das

# AgenticDevelopmentModel Schema Change Procedures

## When to use

Use this skill for any `<shared schema module>`, `<platform-schema bootstrap module>`, `<tenant-schema module>`, or migration-related change.

## Workflow

1. Classify the change as platform schema or tenant schema.
2. Update the ORM source of truth.
3. Add matching idempotent DDL to the correct bootstrap path.
4. Add parity verification updates and indexes.
5. Update seed or simulation assets if the contract changed.

## References

Read `references/schema-touchpoints.md` before editing schema-related files.
