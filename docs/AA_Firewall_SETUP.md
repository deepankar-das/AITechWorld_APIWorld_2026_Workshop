> Author: Deepankar Das

# AA Firewall — Development Setup Guide

> How to set up a new development machine to prepare, build, deploy, and test AA Firewall.

---

## Prerequisites

| Software | Version | Purpose | Install |
|---|---|---|---|
| **macOS** or **Linux** | macOS 14+ or Ubuntu 22+ | Host OS | — |
| **Homebrew** | Latest | Package manager (macOS) | [brew.sh](https://brew.sh) |
| **Node.js** | 22+ | TypeScript daemon, console, tests | `brew install node@22` |
| **Go** | 1.26+ | Go daemon binaries | `brew install go` |
| **PostgreSQL** | 16+ | Audit event persistence | `brew install postgresql@16` |
| **Git** | 2.x+ | Version control, branch detection | `brew install git` |
| **Docker** | Latest (optional) | Containerized PostgreSQL | `brew install --cask docker` |
| **OpenSSL** | 3.x | Certificate generation (mTLS) | Pre-installed on macOS |

---

## Quick Start — Single Machine (Development/Demo)

For development and evaluation, you can run everything on one machine:

> **Note:** Do **not** use `./scripts/deploy.sh --migrate` for full Hub+Sentinel mode; that starts Sentinel-side services only.

```bash
# 1. Get the source code (either clone or untar)
git clone <repo-url> AAFirewall
# OR: tar xzf aafirewall-<version>.tar.gz && mv aafirewall-<version> AAFirewall
cd AAFirewall

# 2. Install prerequisites (includes PostgreSQL — starts it automatically)
./scripts/prepare.sh

# 3. Build everything (TypeScript + Go — does NOT need PostgreSQL)
./scripts/build.sh

# 4. Deploy Hub + Sentinel (runs post-deploy validation automatically)
#    Replace YOUR_ADMIN_PASSWORD and YOUR_DEV_PASSWORD with your own values.
sudo ./scripts/deploy_single_machine_hub_sentinel.sh \
  --seed-auth \
  --seed-hub-admin-user admin \
  --seed-hub-admin-password "YOUR_ADMIN_PASSWORD" \
  --seed-dev-user "$(whoami)" \
  --seed-dev-password "YOUR_DEV_PASSWORD"

# 5. Open both consoles
open http://localhost:9201         # Hub Console (admin/security view)
open http://localhost:9100/login/  # Sentinel Console (developer view)
```

Login values (single machine):
- Hub Console: http://localhost:9201 — username `admin`, your admin password
- Sentinel Console: http://localhost:9100/login/ — your OS username, your dev password

---

## Quick Start — Two Machines (Enterprise/Production)

In production, the Management Hub and Sentinel run on separate machines.

> If the same person is acting as both developer and admin for local evaluation, you can run both on one machine with `deploy_single_machine_hub_sentinel.sh`.

### Machine 1: Management Hub (security team server)

```bash
# 1. Install prerequisites
./scripts/prepare.sh

# 2. Build Go binaries
cd go && make build && cd ..

# 3. Deploy the Hub (sets up PostgreSQL, generates certs, starts Hub)
sudo ./scripts/deploy_hub.sh \
  --seed-auth \
  --seed-admin-user admin \
  --seed-admin-password "adm1"
```

This single command handles: PostgreSQL setup (restricted `aafirewall` user), mTLS certificate generation, policy bundle deployment, Hub startup, and seeded Hub login values for demo environments.

```bash
sudo ./scripts/deploy_hub.sh --status    # Check Hub status
sudo ./scripts/deploy_hub.sh --stop      # Stop the Hub
```

The Hub Console is available at `http://<hub-server>:9201`.
Sentinel API listens on port 9200 (mTLS).

### Machine 2: Developer Machine (Sentinel)

```bash
# Install as privileged service (run by IT admin, NOT by the developer)
sudo AA_CENTRAL_URL=https://<hub-server>:9200 ./scripts/deploy_sentinel.sh \
  --seed-auth \
  --seed-dev-user dev \
  --seed-dev-password "dev1"

# Check status
sudo ./scripts/deploy_sentinel.sh --status
```

This single command (idempotent — safe to re-run) handles: binary installation (root-owned), PostgreSQL setup (developer locked out), managed Claude Code hooks, LaunchDaemon (auto-start, KeepAlive), Hub registration, and health check.

The developer opens VS Code — Claude Code hooks are already active via managed settings. The Sentinel Console at `http://localhost:9100/login/` shows the developer's personal activity.

Demo login values:
- Hub Console: username `admin`, access token `adm1`
- Sentinel Console: username `dev`, access token `dev1`

---

## Detailed Setup

### Step 1: Install Prerequisites

The prepare script validates and installs everything:

```bash
./scripts/prepare.sh
```

This checks for and installs (via Homebrew on macOS):
- Node.js 22+, npm
- Go 1.26+
- PostgreSQL 16
- npm dependencies (`npm install`)
- Console dependencies (`cd console && npm install`)
- Playwright Chromium browser
- TypeScript compilation check

To check without installing:

```bash
./scripts/prepare.sh --check-only
```

### Step 2: PostgreSQL Setup

> **`prepare.sh` already installs and starts PostgreSQL.** For single-machine setup, `deploy_single_machine_hub_sentinel.sh` handles Hub + Sentinel bootstrap automatically.

**If you need to set up PostgreSQL manually:**

```bash
# Option A: Standalone (prepare.sh handles this)
brew services start postgresql@16
createdb aa_firewall
psql -d aa_firewall -f docker/init.sql

# Option B: Docker Compose
docker compose -f docker/docker-compose.yaml up -d
```

**Verify connection:**

```bash
psql -d aa_firewall -c "SELECT COUNT(*) FROM audit_events;"
```

### Step 3: Build

**TypeScript (daemon, enforcement, console):**

```bash
./scripts/build.sh
```

This runs:
1. `npx tsc --noEmit` — type check
2. `npx vitest run` — unit tests (145 tests)
3. `npx tsc -p tsconfig.build.json` — compile to `dist/`
4. `cd console && npm run build` — build Next.js console

**Go (compiled binaries):**

```bash
cd go
make build
```

This produces 5 statically compiled binaries in `go/bin/`:

| Binary | Size | Purpose |
|---|---|---|
| `aafirewall-daemon` | ~9 MB | Daemon + HTTP server + embedded console |
| `aafirewall-hook` | ~8 MB | Claude Code hook handler |
| `aafirewall-central` | ~9 MB | Central mTLS server |
| `aafirewall-client` | ~9 MB | Sentinel agent |
| `aafirewall-authseed` | ~8 MB | Encrypted auth token seeding into PostgreSQL |

**Run Go tests:**

```bash
make test        # 252 tests
make test-v      # Verbose output
make test-cover  # Coverage report → coverage.html
```

### Step 4: Deploy

**Start single-machine Hub + Sentinel deployment (runs validation automatically):**

```bash
sudo ./scripts/deploy_single_machine_hub_sentinel.sh \
  --seed-auth \
  --seed-hub-admin-user admin \
  --seed-hub-admin-password "adm1" \
  --seed-dev-user dev \
  --seed-dev-password "dev1"
```

Equivalent two-command form (if you want explicit split steps):

```bash
sudo ./scripts/deploy_hub.sh --seed-auth --seed-admin-user admin --seed-admin-password "adm1"
sudo AA_CENTRAL_URL=https://localhost:9200 ./scripts/deploy_sentinel.sh --seed-auth --seed-dev-user dev --seed-dev-password "dev1"
./scripts/validate.sh
```

This starts both sides on one machine:
- Management Hub (Sentinel API) on `https://localhost:9200`
- Hub Console on `http://localhost:9201`
- Sentinel Server API on `http://localhost:9100`
- Sentinel Console on `http://localhost:9100/login/`
- Sentinel Network Proxy on `http://localhost:9101`

**Optional: Start Sentinel-only services (no Hub):**

```bash
./scripts/deploy.sh --migrate --seed-auth \
  --seed-role admin \
  --seed-username "admin" \
  --seed-token "adm1"
```

This starts Sentinel-side services only:
- Sentinel Server API on `http://localhost:9100`
- Sentinel Network Proxy on `http://localhost:9101`
- Sentinel Console on `http://localhost:9100/login/`

Logs:
- `/tmp/aa-firewall-daemon.log`
- `/tmp/aa-firewall-console.log`

**Or start the Go daemon:**

```bash
cd go
./bin/aafirewall-daemon
```

The Go daemon serves both the API (port 9100) and the embedded console.

**Sentinel-only deploy options:**

```bash
./scripts/deploy.sh --migrate        # Apply DB migration first
./scripts/deploy.sh --daemon-only    # No console
./scripts/deploy.sh --seed-auth \
  --seed-role admin \
  --seed-username "admin" \
  --seed-token "adm1"                 # Seed short demo credentials
```

### Step 5: Verify

> Validation runs automatically as part of `deploy_single_machine_hub_sentinel.sh`. Use this section if you want to re-run validation independently.

**Run enforcement validation (full mode: Hub + Sentinel required):**

```bash
./scripts/validate.sh
```

`./scripts/validate.sh` performs preflight checks for:
- Sentinel Server health (`/v1/health`)
- Hub Console health (`/api/v1/health`)
- Hub registered Sentinel clients (`/api/v1/clients`) — fails fast if zero clients
- Hub audit ingestion (`/api/v1/audit`) — verifies new events arrive from Sentinel

**Optional manual visibility check (not required):**

```bash
curl -s http://localhost:9201/api/v1/clients \
  -H "X-Admin-Token: adm1" | python3 -m json.tool
```

**Sentinel-only mode (no Hub check):**

```bash
./scripts/validate.sh --sentinel-only
```

> Note: Sentinel policy mutation endpoints are disabled by default in enterprise mode (`local_policy_edit_disabled`). Policy authoring/changes must be done in the Management Hub and then synced to Sentinels. Standalone local policy authoring for `--sentinel-only` workflows is planned for Phase 2.

9 scenarios test all enforcement surfaces:

| # | Scenario | Expected |
|---|---|---|
| 1 | File write inside project | ALLOW |
| 2 | File write outside project | DENY |
| 3 | Safe shell command (`npm test`) | ALLOW |
| 4 | Destructive command (`rm -rf`) | REQUIRE_APPROVAL |
| 5 | Network to unknown host | DENY |
| 6 | Sensitive path read (`~/.ssh/id_rsa`) | DENY |
| 7 | Package install (`npm install`) | REQUIRE_APPROVAL |
| 8 | Credential access (`cat .env`) | DENY |
| 9 | Unknown MCP server | REQUIRE_APPROVAL |

**Run the readiness gate report:**

```bash
./scripts/readiness-report.sh
```

Reports 6 gates: policy mediation rate, enforcement fidelity, audit completeness, schema validation, approval latency, false positive rate.

---

## Services and Ports

> **Two-machine deployment:** Hub services (9200, 9201) run on the security team server. Sentinel services (9100, 9101) run on the developer machine. The Sentinel Console is embedded in the daemon at port 9100. PostgreSQL runs on BOTH machines — Hub PG aggregates all developers, Sentinel PG stores local events only.

| Service | Port | Protocol | Auth | Purpose |
|---|---|---|---|---|
| Daemon API | 9100 | HTTP | Admin token (write), open (read) | Policy evaluation, audit, approvals, metrics |
| Network Proxy | 9101 | HTTP CONNECT | None | Egress traffic interception |
| Sentinel Console | 9100 | HTTP | Developer login | Embedded in daemon — developer-personal view (no admin access) |
| Management Hub (Sentinel API) | 9200 | HTTPS (mTLS) | Client certificate | Policy distribution, audit aggregation |
| Hub Console | 9201 | HTTP | RBAC token (see below) | Admin dashboard, policy management, approvals |
| MCP Gateway | 9102 | HTTP | None | MCP tool governance |
| PostgreSQL | 5432 | PostgreSQL | Local trust | Audit event storage |
| OS Kernel Enforcement | — | Syscall | Internal | KernelEnforcer (eBPF/ESF) — no listening port; intercepts file.open, execve, connect at kernel level. Audit events routed to daemon on port 9100. |

### Hub RBAC Tokens

The Hub requires RBAC authentication for all API and Console access. Tokens are seeded during deployment:

| Role | Token (demo) | Permissions |
|---|---|---|
| **admin** | `adm1` | Full access: policy management, approvals, audit, client management, enforcement toggle |
| **reviewer** | (assigned per org) | Read access + approve/deny pending approvals |
| **operator** | `dev1` | Read-only: view audit events, sessions, and own activity |

All Hub API calls require the `X-Admin-Token` header:

```bash
curl -s http://localhost:9201/api/v1/policy \
  -H "X-Admin-Token: adm1" | python3 -m json.tool
```

---

## Environment Variables

### Required for production

| Variable | Default | Purpose |
|---|---|---|
| `DATABASE_URL` | `postgresql://$USER@localhost:5432/aa_firewall` | PostgreSQL connection (with `sslmode=require` for production) |
| `AA_ADMIN_TOKEN` | Auto-generated dev token | Admin authentication for management endpoints |

### Optional configuration

| Variable | Default | Purpose |
|---|---|---|
| `DAEMON_PORT` | `9100` | Daemon API port |
| `AA_WORKSPACE` | `process.cwd()` | Project root for path-based policy evaluation |
| `AA_STRICT_MODE` | `true` | Strict mode: deny on all error paths, fail-closed startup |
| `AA_DAEMON_URL` | `http://127.0.0.1:9100` | Daemon URL for hook handler |
| `AA_SESSION_ID` | Auto-generated | Session tracking for audit correlation |
| `CERT_DIR` | `/etc/aafirewall/certs` | TLS certificate directory |
| `AA_CENTRAL_URL` | `https://localhost:9200` | Management Hub URL (for Sentinel agent) |
| `AA_CLIENT_ID` | `client_<hostname>_<timestamp>` | Sentinel agent identifier |
| `AA_SYNC_INTERVAL` | `5s` | Sentinel agent heartbeat/sync interval |
| `AA_CONFIG_DIR` | `/etc/aafirewall` | Configuration directory |
| `AA_OSGUARD_MODE` | `off` | OS-level enforcement mode: `enforce` (block), `audit` (log only), `off` (disabled) |
| `NEXT_PUBLIC_DAEMON_URL` | `http://127.0.0.1:9100` | Daemon URL for the console frontend |

---

## Claude Code Integration

AA Firewall governs Claude Code through **PreToolUse** and **PostToolUse** hooks. Every tool call (Read, Write, Edit, Bash, Glob, Grep, WebFetch, WebSearch) is routed through the hook handler, which calls the Sentinel daemon for policy evaluation before execution.

There are two hook configuration locations. Both can be active simultaneously — managed settings take priority.

### 1. Project-Level Hooks (Development)

**Location:** `.claude/settings.json` in the project root

**What it does:** Adds hooks that fire for this project only. The developer can modify or remove these.

**Install:**
```bash
cat > .claude/settings.json << 'HOOKS'
{
  "hooks": {
    "PreToolUse": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/aafirewall-hook pre_tool_call",
            "timeout": 300,
            "statusMessage": "AA Firewall: Evaluating policy..."
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/aafirewall-hook post_tool_call"
          }
        ]
      }
    ]
  }
}
HOOKS
```

**Disable:**
```bash
echo '{}' > .claude/settings.json
```

**Verify:**
```bash
cat .claude/settings.json
```

> **Note:** This file is checked into git. The deploy script (`deploy_single_machine_hub_sentinel.sh`) restores it on redeploy.

### 2. Managed Hooks (Enterprise / MDM)

**Location:** `/Library/Application Support/ClaudeCode/managed-settings.json` (macOS), root-owned

**What it does:** Enforces hooks system-wide. When `allowManagedHooksOnly` is `true`, the developer **cannot remove or modify** hooks — only root can. This is the enterprise deployment model.

**Install (requires sudo):**
```bash
sudo mkdir -p "/Library/Application Support/ClaudeCode"
sudo tee "/Library/Application Support/ClaudeCode/managed-settings.json" > /dev/null << 'HOOKS'
{
  "hooks": {
    "PreToolUse": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/aafirewall-hook pre_tool_call",
            "timeout": 300,
            "statusMessage": "AA Firewall: Evaluating policy..."
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/aafirewall-hook post_tool_call"
          }
        ]
      }
    ]
  },
  "allowManagedHooksOnly": true
}
HOOKS
sudo chmod 644 "/Library/Application Support/ClaudeCode/managed-settings.json"
sudo chown root:wheel "/Library/Application Support/ClaudeCode/managed-settings.json"
```

**Disable (requires sudo):**
```bash
sudo rm "/Library/Application Support/ClaudeCode/managed-settings.json"
```

**Verify:**
```bash
cat "/Library/Application Support/ClaudeCode/managed-settings.json" 2>/dev/null || echo "Not installed"
```

> **Note:** The deploy script (`deploy_sentinel.sh`) installs managed hooks automatically (Step 6). Redeploying restores them.

### Completely Disable AA Firewall Governance

To fully disable all governance hooks and let Claude Code run ungoverned:

```bash
# 1. Remove managed hooks (requires sudo)
sudo rm -f "/Library/Application Support/ClaudeCode/managed-settings.json"

# 2. Remove project hooks
echo '{}' > .claude/settings.json

# 3. Restart VS Code (hooks are read on startup)
```

Claude Code will now execute all tool calls without policy evaluation.

### Re-enable AA Firewall Governance

To restore governance after disabling:

```bash
# Full redeploy (restores both managed and project hooks, restarts services)
sudo ./scripts/deploy_single_machine_hub_sentinel.sh
```

Or restore hooks manually without redeploying:

```bash
# Restore managed hooks only
sudo ./scripts/deploy_sentinel.sh --restart

# Restore project hooks only (from git)
git checkout .claude/settings.json
```

### Hook Handler Details

- **Binary:** `/usr/local/bin/aafirewall-hook` (Go compiled, ~6 MB, starts in ~5ms)
- **Log file:** `~/.aafirewall/hook.log` (developer-readable, logs every invocation)
- **Workspace detection:** Walks up from current directory to find `.git/` or `.claude/` marker as the project root
- **Auth:** Reads operator token from `/etc/aafirewall/.operator_token` (must be `chmod 644`)
- **Internal tools:** Agent, TodoWrite, Skill, etc. are governed by the `org.allow_internal_tools` policy rule (not hardcoded bypasses)
- **Exit codes:** `0` = allow, `2` = block (deny or approval required)

---

## TLS Certificates (for Management Hub)

Generate mTLS certificates for secure central-client communication:

```bash
./scripts/generate-certs.sh
```

This creates in `certs/` (or `CERT_DIR`):

| File | Purpose |
|---|---|
| `ca.pem` | Certificate Authority (distribute to clients) |
| `ca-key.pem` | CA private key (keep secret) |
| `server.pem` + `server-key.pem` | Management Hub identity |
| `client.pem` + `client-key.pem` | Sentinel agent identity |

---

## System Service Installation (macOS)

For production deployment where the daemon must survive reboots and resist developer tampering:

```bash
sudo ./scripts/install-service.sh
```

This installs:
- LaunchDaemon at `/Library/LaunchDaemons/com.aafirewall.daemon.plist`
- Auto-restarts on crash (`KeepAlive`)
- Runs as root (developer cannot kill it)
- Managed hooks at `/Library/Application Support/ClaudeCode/managed-settings.json`
- Admin token at `/etc/aafirewall/.admin_token` (permissions 600)

```bash
sudo ./scripts/install-service.sh --status     # Check status
sudo ./scripts/install-service.sh --uninstall  # Remove
```

> **Governance is always on.** After system service installation, the developer cannot: kill the daemon (it auto-restarts), remove hooks (managed settings prevent it), toggle enforcement (requires admin token), or modify policies (signed bundles from Management Hub). Only an administrator with root access and the admin token can change the configuration.

---

## Management Hub + Sentinel Deployment

For multi-machine deployments with centralized policy distribution:

```bash
# On the Management Hub server
sudo ./scripts/deploy_hub.sh \
  --seed-auth \
  --seed-admin-user admin \
  --seed-admin-password "adm1"

# On each developer machine
sudo AA_CENTRAL_URL=https://<hub-server>:9200 ./scripts/deploy_sentinel.sh \
  --seed-auth \
  --seed-dev-user dev \
  --seed-dev-password "dev1"

# Optional wrapper mode (when explicit seed values are not required):
sudo ./scripts/aafirewall_deploy.sh central

# Or both components on one machine
sudo ./scripts/aafirewall_deploy.sh full

# Check status
sudo ./scripts/aafirewall_deploy.sh status

# Remove everything
sudo ./scripts/aafirewall_deploy.sh uninstall
```

---

## Enterprise MDM Deployment (Zero-Touch)

For enterprise environments where IT provisions machines via MDM, the developer never runs any installation commands. The security team pushes the AA Firewall package to developer machines, and governance activates automatically when they open VS Code.

### What Gets Pushed

| Component | Target Path | Purpose |
|---|---|---|
| `aafirewall-daemon` | `/usr/local/bin/` | Daemon binary |
| `aafirewall-hook` | `/usr/local/bin/` | Hook handler binary |
| `aafirewall-client` | `/usr/local/bin/` | Sentinel agent binary |
| `com.aafirewall.daemon.plist` | `/Library/LaunchDaemons/` | Auto-start at boot |
| `managed-settings.json` | `/Library/Application Support/ClaudeCode/` | Pre-configures Claude Code hooks |
| `.admin_token` | `/etc/aafirewall/` | Admin token (600 perms) |
| `client.crt`, `client.key`, `ca.crt` | `/etc/aafirewall/certs/` | mTLS certificates |
| `default.yaml` | `/etc/aafirewall/` | Initial policy bundle |
| `.db_credentials` | `/etc/aafirewall/` | PostgreSQL DATABASE_URL (root:600 — developer cannot read) |

### How It Works

1. **IT provisions machine via MDM** (Jamf, Intune, Ansible, etc.) — pushes AA Firewall package
2. **Installer runs `setup-database.sh` under sudo** — creates dedicated `aafirewall` PostgreSQL user with INSERT+SELECT only, revokes developer's OS user access, stores credentials in `/etc/aafirewall/.db_credentials` (root:600)
3. **LaunchDaemon starts daemon at boot** — before the developer logs in
4. **Sentinel agent registers with Management Hub** — receives signed policy bundle via mTLS
5. **Developer opens VS Code** — Claude Code reads `managed-settings.json`, hooks are active
6. **Every tool call is governed** — the developer sees blocks and approval prompts but cannot disable them
6. **Management Hub sees the developer** — workspace, repo, branch, agent type, action history

### Managed Settings — Why the Developer Cannot Disable Governance

The file `/Library/Application Support/ClaudeCode/managed-settings.json` contains:

```json
{
  "hooks": {
    "PreToolUse": [
      {"type": "command", "command": "/usr/local/bin/aafirewall-hook pre_tool_call"}
    ],
    "PostToolUse": [
      {"type": "command", "command": "/usr/local/bin/aafirewall-hook post_tool_call"}
    ]
  },
  "allowManagedHooksOnly": true
}
```

- `allowManagedHooksOnly=true` means Claude Code ignores the developer's personal `~/.claude/settings.json` hooks
- The managed file is owned by `root:wheel` (644) — the developer cannot edit it
- The hook handler binary is owned by root — the developer cannot replace it
- The daemon runs as root via LaunchDaemon with `KeepAlive` — the developer cannot kill it

### MDM Tools by Platform

| Platform | Tool | Package Format |
|---|---|---|
| macOS | Jamf Pro, Mosyle, Kandji, Fleet, Munki | .pkg installer |
| Linux | Ansible, Chef, Puppet, Salt | .deb / .rpm |
| Windows | Microsoft Intune, SCCM | .msi |

### Verification

From the Management Hub, verify all machines are governed:

```bash
curl -s http://localhost:9201/api/v1/clients \
  -H "X-Admin-Token: adm1" | python3 -m json.tool
```

Any machine with `"status": "stale"` or `"status": "offline"` needs investigation.

---

## Audit Trail Security

The audit database is tamper-proof. The developer cannot query, read, modify, or delete audit events.

### Database Setup (automatic during installation)

```bash
# Run by the installer under sudo — creates restricted PostgreSQL user
sudo ./scripts/setup-database.sh
```

This script:
1. Creates a dedicated `aafirewall` PostgreSQL user with a random 32-char password
2. Creates the `aa_firewall` database owned by `aafirewall`
3. Applies the schema with **INSERT + SELECT only** grants (no UPDATE, DELETE, or TRUNCATE)
4. **Revokes all access from the developer's OS user** — they cannot even connect to the database
5. Stores `DATABASE_URL` in `/etc/aafirewall/.db_credentials` (root:600 — developer cannot read)
6. Verifies the developer cannot connect

### What the Developer Cannot Do

| Attempt | Result |
|---|---|
| `psql -d aa_firewall` | Connection refused — developer's OS user has no CONNECT grant |
| `cat /etc/aafirewall/.db_credentials` | Permission denied — file is root:600 |
| Read DATABASE_URL from daemon process | Not visible — set in LaunchDaemon plist environment |
| Stop PostgreSQL | PostgreSQL runs as a system service — developer lacks privilege |
| Delete the database files | Files owned by postgres user in PostgreSQL data directory |

### What the Daemon Can Do

| Operation | Allowed? | Purpose |
|---|---|---|
| INSERT into audit_events | Yes | Write new audit events |
| SELECT from audit_events | Yes | Query/export for console and analytics |
| UPDATE audit_events | **No** | Events are immutable after insertion |
| DELETE from audit_events | **No** | Events cannot be removed |
| CREATE/DROP tables | **No** | Schema cannot be modified |

### Additional Protections

- **SHA-256 hash chain** — each event's hash includes the previous event's hash. If any event is modified, deleted, or reordered, the chain verification fails.
- **Management Hub aggregation** — events are forwarded to the Management Hub via mTLS. Even if local PostgreSQL is wiped, the Management Hub has the copy.
- **Ed25519 signed exports** — exported evidence packages are cryptographically signed. Cannot be forged or tampered with after export.

---

## Running Tests

### Unit tests (TypeScript)

```bash
npx vitest run                 # 150 tests
npx vitest run --watch         # Watch mode
```

### Unit tests (Go)

```bash
cd go
make test                      # 252 tests
make test-v                    # Verbose
make test-cover                # Coverage report
```

### E2E tests (Playwright)

```bash
# Requires daemon running on port 9100 (console is embedded in daemon)
npx playwright test            # 25 tests (headless Chromium)
npx playwright test --ui       # Interactive UI mode
```

### Enforcement validation

```bash
# Full validation requires Hub Console (9201) + registered Sentinel + daemon (9100)
./scripts/validate.sh              # 9 scenarios (full mode)
./scripts/validate.sh --verbose    # Full API responses
./scripts/validate.sh --sentinel-only  # Skip Hub registration check
```

---

## Building Distribution Packages

### Go binaries (production)

```bash
cd go
make all                       # Build all 4 binaries
make package                   # Create tarball with binaries + policies
make build-linux               # Cross-compile for Linux amd64
```

### Release with integrity artifacts

```bash
make release-integrity         # Build + SBOM (syft) + signature (cosign) + provenance
```

Output in `go/dist/`:
- `aafirewall-<version>-<os>-<arch>.tar.gz` — binaries + policies
- `aafirewall-<version>-sbom.spdx.json` — Software Bill of Materials
- `aafirewall-<version>.sig.bundle` — Code signature
- `aafirewall-<version>-provenance.json` — Build provenance

---

## Uninstall

Remove all AA Firewall components (services, binaries, hooks, configs, logs):

```bash
sudo ./scripts/uninstall.sh
```

This removes:
- LaunchDaemon services (stops all running daemons)
- Binaries from `/usr/local/bin/`
- Managed hooks (`/Library/Application Support/ClaudeCode/managed-settings.json`)
- User-level Claude Code hooks (`~/.claude/settings.json`)
- Configuration (`/etc/aafirewall/`, `/opt/aafirewall/`)
- Logs (`/var/log/aafirewall/`, `/var/lib/aafirewall/`)

The PostgreSQL audit database (`aa_firewall`) is **preserved by default** because it contains audit evidence. To also drop the database:

```bash
sudo ./scripts/uninstall.sh --drop-database
```

After uninstall, restart VS Code for hook changes to take effect.

---

## Troubleshooting

| Problem | Check | Fix |
|---|---|---|
| PostgreSQL not running | `pg_isready` | `brew services start postgresql@16` |
| Database doesn't exist | `psql -l \| grep aa_firewall` | `createdb aa_firewall && psql -d aa_firewall -f docker/init.sql` |
| Port 9100 in use | `lsof -i :9100` | `kill <PID>` or change `DAEMON_PORT` |
| Sentinel Console not loading | Check daemon logs | Verify daemon is running on port 9100 (console is embedded) |
| TypeScript errors | `npx tsc --noEmit` | Fix reported errors |
| Go build fails | `go vet ./...` | Fix reported errors, run `go mod tidy` |
| Hooks not firing | `./scripts/install-hooks.sh --status` | Reinstall hooks, restart Claude Code |
| Console shows no data | Check daemon logs | Verify daemon is running and `NEXT_PUBLIC_DAEMON_URL` is correct |
| Admin endpoints return 401 | Check `AA_ADMIN_TOKEN` | Set token in env or login via console |
| Strict mode blocks startup | `AA_STRICT_MODE=true` | Ensure PostgreSQL is running and policy YAML exists |

---

## File Structure Reference

```
AAFirewall/
├── go/                        # Go port (compiled binaries — production deployment)
│   ├── cmd/                   # 4 entry points (daemon, hook, central, client)
│   ├── internal/
│   │   ├── types/             # Action, Policy, Audit, Approval, MCP structs
│   │   ├── policy/            # Engine, loader, hierarchy, packs, signing (Ed25519)
│   │   ├── audit/             # Validate, buffer, store (PostgreSQL), flush, chain (SHA-256)
│   │   ├── approval/          # Service, scope, break-glass
│   │   ├── enforcement/       # Classifier, package guard, secret detector, redaction, MCP gateway
│   │   │   └── osguard/       # KernelEnforcer interface + StubEnforcer (eBPF/ESF, 17 tests)
│   │   ├── intelligence/      # Anomaly detection (8 patterns)
│   │   ├── daemon/            # HTTP server, RBAC auth, strict mode, routes
│   │   ├── central/           # Central mTLS server
│   │   ├── client/            # Sentinel agent (sync, heartbeat)
│   │   └── console/           # Embedded static assets (go:embed)
│   ├── Makefile               # build, test, package, release-integrity
│   └── go.mod                 # 3 dependencies (uuid, pgx, yaml)
├── src/                       # TypeScript daemon (original prototype)
├── console/                   # Next.js 15 + shadcn/ui Hub Console + Sentinel Console
├── types/                     # Shared TypeScript types (Zod schemas)
├── tests/                     # Vitest unit tests (150)
├── e2e/                       # Playwright E2E tests (25)
├── policies/                  # YAML policy bundles
│   ├── default.yaml           # 13 rules
│   └── network-allowlist.yaml # Allowed hosts + warning list
├── docker/                    # PostgreSQL compose + schema
├── scripts/                   # 11 shell scripts (prepare, build, deploy, ...)
├── docs/                      # PRD, TDD, Implementation Plan, Demo, Setup, peer reviews
├── CLAUDE.md                  # Project instructions for Claude Code
└── package.json               # Node.js dependencies
```
