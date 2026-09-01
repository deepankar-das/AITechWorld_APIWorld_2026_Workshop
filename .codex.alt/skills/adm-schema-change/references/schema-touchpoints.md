> **AgenticDevelopmentModel**
> Copyright © 2026 AgenticDevelopmentModel Inc. Proprietary and Confidential.
>
> Author: Deepankar Das
# Schema Touchpoints

## Platform schema

- Define source of truth in the `<shared schema module>` under the `<platform schema namespace>`.
- Add idempotent DDL to the `<platform-schema bootstrap module>`.
- Update the `<platform-schema verifier>`.
- Add indexes with `CREATE INDEX IF NOT EXISTS`.
- Verify the `<ORM migration config>` restricts the schema filter to the public and platform schemas.

## Tenant schema

- Define tenant tables in the `<shared schema module>` as plain (non-platform-namespaced) tables.
- Add DDL to the `<tenant-DDL builder>` in the `<tenant-schema module>`.
- Add the table to the tenant-tables array.
- Add indexes and new-column `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` statements.
