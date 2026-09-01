---
name: adm-deploy
description: >
  AgenticDevelopmentModel build and deployment workflow for Cloud Run. Use when
  building images, deploying to staging or production, running
  migrations, seeding data, or when the user says "deploy", "build",
  "staging", "production", "seed", "migrate", "Cloud Run", "gcloud",
  or "deploy script". Covers GCP project isolation, build-then-deploy
  sequence, environment configs, Cloud SQL tier enforcement, and
  secret IAM binding.
---

# AgenticDevelopmentModel Build and Deploy Workflow

## GCP Project Isolation

- **App scripts** (`<build script>`, `<deploy script>`): Default project is `<build-project>`. Must reject `<ai-project>`.
- **AI scripts** (`<ai-deploy script>`, `<ai-train script>`): Default project is `<ai-project>`. Must reject `<build-project>`.
- Every `gcloud` resource command must have explicit `--project=` flag. Never rely on `gcloud config get-value project`.

## Build and Deploy Sequence

When schema changes are made:
1. Commit changes
2. Build new image: `<build script> --env staging`
3. Deploy with `--migrate` flag: `<deploy script> --env staging --migrate`

The Docker image must contain the latest schema for the ORM's push command to detect changes. A stale image means the migration runs against old schema definitions.

## Server Startup Rules

**Never start a server with bare commands.** Always use repository scripts:

- **Dev (hot-reload):** `<dev-server script>` (default port 5000) or `<dev-server script> --port 3000`
- **Deploy:** `<deploy script> --env <target>` with flags as needed
- **Before RADAR:** a server must be running (the `<radar script>` fails fast otherwise). Use `--force` to skip Playwright E2E when no server is available.

Bare `npx tsx <server entrypoint>` / `npm run dev` won't load `.env`, won't hot-reload, won't exclude temp files.

## Deploy Flag Reference

| Flag | Purpose |
|------|---------|
| `--env <target>` | Target environment: `local_macos`, `local_ubuntu`, `staging`, `production` |
| `--migrate` | Run DDL migrations after deploy |
| `--seed-reset` | Drop and re-seed all tenant data |
| `--dual-tenant` | Create the primary demo tenant + a second demo tenant |
| `--multi-tenant` | Create all configured demo tenants |
| `--alpha-ea-tenants` | Create the 6 Alpha EA tenants |
| `--two-tenant-empty` | Create two empty tenants (no seed data) |
| `--force` | Skip Playwright E2E checks |
| `--data-migrate` | Run data migration scripts |
| `--cleanup` | Remove stale Cloud Run jobs |

**Example:** `<deploy script> --env staging --migrate --seed-reset --dual-tenant --alpha-ea-tenants`

## Cross-Platform Shell Rules

All scripts must work on macOS (bash 3.2) and Linux (bash 4+):
- Never use GNU-only tools (`tac`, `readlink -f`, `sed -i` without `''`)
- Never use `declare -A` (associative arrays) — macOS bash 3.2 lacks them
- Never use `rg` (ripgrep) in scripts — not available in bash subshells or CI
- Bash arrays under `set -u` must use `${arr[@]+"${arr[@]}"}` syntax
- For platform-sensitive commands, branch on `uname`: `if [[ "$(uname)" == "Darwin" ]]; then ... fi`

For environment-specific configs (CPU, memory, instances, secrets), see `references/env-configs.md`.
