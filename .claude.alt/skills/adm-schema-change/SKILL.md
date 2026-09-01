---
name: adm-schema-change
description: >
  AgenticDevelopmentModel database schema change procedures for platform and tenant
  tables. Use when adding columns, creating tables, modifying schema,
  or when the user says "schema", "table", "column", "migration",
  "DDL", "bootstrap", "tenant-schema", "ORM", or "ALTER TABLE".
  Covers platform schema (bootstrap module + schema verifier),
  tenant schema (tenant-DDL builder), idempotent DDL, and the
  invariant tests that pin schema parity.
---

# AgenticDevelopmentModel Schema Change Procedures

## Platform Schema Changes (CRITICAL)

When adding columns or tables to the `<shared schema module>` under the `<platform schema namespace>`:

1. Define in the `<shared schema module>` (ORM source of truth)
2. **MUST** add `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` or `CREATE TABLE IF NOT EXISTS` to the `<platform-schema bootstrap module>`
3. **MUST** add to the `<platform-schema verifier>` expected columns list in the `<platform-schema bootstrap module>`
4. Add corresponding indexes with `CREATE INDEX IF NOT EXISTS`
5. Verify the `<ORM migration config>` restricts the schema filter to the public and platform schemas

**Why:** the ORM's push command silently no-ops on platform schema changes. The bootstrap module's idempotent SQL is the reliable migration path. See the `<debugging FAQ>` Section 1 for full details.

## Tenant Schema Changes (CRITICAL)

When adding tables or columns to the `<shared schema module>` as tenant tables (a plain table without the `<platform schema namespace>`):

1. Define in the `<shared schema module>` using a plain table (NOT a platform-namespaced table)
2. **MUST** add `CREATE TABLE IF NOT EXISTS` to the `<tenant-DDL builder>` in the `<tenant-schema module>`
3. **MUST** add the table to the tenant-tables array at the bottom of the `<shared schema module>`
4. Add corresponding indexes with `CREATE INDEX IF NOT EXISTS`
5. For new columns on existing tenant tables, add `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` in the idempotent section at the bottom of the `<tenant-DDL builder>`

**Why:** Tenant schemas are provisioned by the tenant-provisioning routine, which runs the DDL from the `<tenant-DDL builder>`. The ORM's push command only affects the public schema — it does NOT create tables in existing tenant schemas. If you add a tenant table to the schema module but not to the `<tenant-DDL builder>`, the table will be missing in ALL existing tenants, causing "relation does not exist" errors at runtime.

## Key Modules

| Module | Purpose |
|--------|---------|
| `<shared schema module>` | All ORM table definitions (platform + tenant) |
| `<platform-schema bootstrap module>` | Idempotent SQL for platform schema + the `<platform-schema verifier>` |
| `<tenant-schema module>` | The `<tenant-DDL builder>` — tenant-scoped DDL source of truth |

## Related Invariant Tests

- **INV2** — Tenant-schema parity: `<tenant-schema-parity invariant test>`
- **INV3** — Platform-schema parity: `<platform-schema-parity invariant test>`

If your schema change breaks parity, these invariant tests will catch it during G3.
