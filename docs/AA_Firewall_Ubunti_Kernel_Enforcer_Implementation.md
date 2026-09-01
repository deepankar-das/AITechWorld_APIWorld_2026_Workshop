> Author: Deepankar Das

# AA Firewall — Ubuntu Port + eBPF Kernel Enforcer Implementation Plan

> Engineering workplan for (a) porting AA Firewall from macOS to Ubuntu and (b) building the eBPF `KernelEnforcer` against the existing Go interface. Companion to [AA_Firewall_Ubuntu_Kernel_Enforcer.md](AA_Firewall_Ubuntu_Kernel_Enforcer.md), which is the design overview; this document is the step-by-step implementation guide with concrete file paths, line references, and acceptance criteria.

---

## 1. Scope and Non-Goals

**In scope**
- Make the daemon, client, hub, and hook handler runnable on Ubuntu 22.04+ with systemd.
- Port `deploy_sentinel.sh` and `uninstall.sh` away from `launchctl` / plist / `/Library/...` paths.
- Build an `EbpfEnforcer` implementation of [KernelEnforcer](../go/internal/enforcement/osguard/kernel.go#L65-L82) using `cilium/ebpf`.
- Wire the new enforcer into the daemon via `AA_OSGUARD_MODE` and expose `/v1/osguard/metrics`.

**Out of scope**
- macOS Endpoint Security Framework implementation (separate workstream).
- Hardening of the eBPF programs against adversarial bypass beyond LSM hook coverage.
- Distribution packaging (`.deb`, snap) — covered by a follow-up doc.

---

## 2. Current State (Verified Against Code)

| Concern | Evidence | Status |
|---|---|---|
| Go binaries | [go/cmd/daemon](../go/cmd/daemon), [go/cmd/client](../go/cmd/client), [go/cmd/central](../go/cmd/central), [go/cmd/hookhandler](../go/cmd/hookhandler), [go/cmd/authseed](../go/cmd/authseed) | Cross-compile clean; no CGO |
| `KernelEnforcer` interface | [kernel.go:65-82](../go/internal/enforcement/osguard/kernel.go#L65-L82) | Defined |
| `StubEnforcer` (proof-of-integration) | [stub.go](../go/internal/enforcement/osguard/stub.go) | Implemented + tested ([osguard_test.go](../go/internal/enforcement/osguard/osguard_test.go)) |
| `EbpfEnforcer` | — | Does not exist |
| Daemon enforcer wiring | [server.go](../go/internal/daemon/server.go) (`StartDaemon`) | Not initialized; no `AA_OSGUARD_MODE` read |
| `/v1/osguard/*` HTTP routes | [server.go](../go/internal/daemon/server.go) router switch | Missing |
| Policy engine helpers (`GetDeniedPaths`, `GetDeniedExecs`, `GetAllowedHosts`) | [engine.go](../go/internal/policy/engine.go), [allowlist.go](../go/internal/policy/allowlist.go) | Missing — must be derived from `PolicyBundle.Rules` |
| `setup-database.sh` | [setup-database.sh:92-100](../scripts/setup-database.sh#L92) | Already has `apt-get` + `chown root:root` fallback |
| `deploy_hub.sh` | `chown root:wheel` calls at lines 342, 347, 374 | Has `root:root` fallback — works on Ubuntu |
| `deploy_sentinel.sh` | LaunchDaemon paths at [deploy_sentinel.sh:58-61](../scripts/deploy_sentinel.sh#L58-L61); `launchctl` at lines 222-224, 631-651 | macOS-only — must be ported |
| `uninstall.sh` | LaunchDaemon array at lines 70-84; `/Library/Application Support/ClaudeCode` at line 129 | macOS-only — must be ported |
| `validate.sh`, `prepare.sh`, `build.sh` | Pure curl/npm/Go | Already portable |
| systemd unit files | none | Must be added |
| `cilium/ebpf` dependency | not in [go.mod](../go/go.mod) | Must be added |

---

## 3. Part A — Ubuntu Port

### 3.1 Goals

1. A developer running Ubuntu 22.04+ can run `sudo ./scripts/deploy_sentinel.sh` and the daemon + client come up under systemd, survive reboot, and survive being killed.
2. `sudo ./scripts/uninstall.sh` cleans up systemd units, hooks, certs, and binaries — leaving no orphan processes.
3. Managed hooks for Claude Code land at the right Linux path so the user cannot disable them.

### 3.2 Platform Detection Strategy

Add a single helper sourced by every deploy/uninstall script. Prefer one script with branches over two parallel scripts — the macOS and Linux flows are 80% identical (paths, ownership, certs, hook installation) and only diverge on the service-supervisor section.

Create [scripts/lib/platform.sh](../scripts/lib/platform.sh) (new):

```bash
#!/bin/bash
# Sourced by deploy_*.sh and uninstall.sh
detect_platform() {
  case "$(uname -s)" in
    Darwin) echo "macos" ;;
    Linux)  echo "linux" ;;
    *) echo "unsupported" ;;
  esac
}

# Group used for root-owned files. macOS = wheel, Linux = root.
root_group() {
  case "$(detect_platform)" in
    macos) echo "wheel" ;;
    *)     echo "root" ;;
  esac
}

# Path for Claude Code managed (developer-locked) settings.
managed_hooks_path() {
  case "$(detect_platform)" in
    macos) echo "/Library/Application Support/ClaudeCode/managed-settings.json" ;;
    *)     echo "/etc/claude-code/managed-settings.json" ;;
  esac
}
```

Then everywhere the existing scripts call `chown root:wheel ... || chown root:root ...`, replace with `chown "root:$(root_group)" ...`. This is purely cosmetic (the fallback already works) but keeps intent clear.

### 3.3 Convert `deploy_sentinel.sh` Service Section

**File:** [scripts/deploy_sentinel.sh](../scripts/deploy_sentinel.sh)

| Lines | Today (macOS) | Replacement (Linux) |
|---|---|---|
| 58 | `MANAGED_HOOKS_DIR="/Library/Application Support/ClaudeCode"` | `MANAGED_HOOKS_DIR="$(managed_hooks_path | xargs dirname)"` |
| 60-61 | `PLIST_FILE=...sentinel.plist`, `CLIENT_PLIST_FILE=...sentinel-client.plist` | `UNIT_FILE="/etc/systemd/system/aafirewall-sentinel.service"`, `CLIENT_UNIT_FILE="/etc/systemd/system/aafirewall-sentinel-client.service"` |
| 222-224 | `launchctl unload "$PLIST_FILE"` | `systemctl stop aafirewall-sentinel aafirewall-sentinel-client \|\| true` |
| 263-264 | `rm -f "$PLIST_FILE"` etc. | `systemctl disable --now aafirewall-sentinel aafirewall-sentinel-client; rm -f "$UNIT_FILE" "$CLIENT_UNIT_FILE"; systemctl daemon-reload` |
| 542-584 | Heredoc emitting `.plist` for sentinel daemon | Heredoc emitting `.service` (template below) |
| 588-622 | Heredoc emitting `.plist` for client | Same |
| 631-651 | `launchctl load` + `kickstart` fallback | `systemctl daemon-reload && systemctl enable --now aafirewall-sentinel aafirewall-sentinel-client` |
| 688-689 | `LaunchDaemon: $PLIST_FILE` summary lines | `Service unit:   $UNIT_FILE` |

Wrap the entire service-management block in `if [[ "$(detect_platform)" == "macos" ]]; then ... else ... fi`. This preserves the macOS path exactly so developers on Mac don't regress.

**Sentinel daemon unit file** to emit at `$UNIT_FILE`:

```ini
[Unit]
Description=AA Firewall Sentinel Daemon
After=network-online.target postgresql.service
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/aafirewall-daemon
Restart=always
RestartSec=3
User=root
EnvironmentFile=/etc/aafirewall/sentinel.env
# Hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=/etc/aafirewall /var/log/aafirewall /var/run
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

Note `EnvironmentFile=/etc/aafirewall/sentinel.env` — write `DATABASE_URL`, `AA_STRICT_MODE`, `AA_CENTRAL_URL`, `DAEMON_PORT`, `CONSOLE_PORT`, and (later) `AA_OSGUARD_MODE` there with mode `0600 root:root`. Equivalent to how the plist embeds env vars today, but keeps secrets out of the unit file (which is world-readable).

**Sentinel client unit file** at `$CLIENT_UNIT_FILE`:

```ini
[Unit]
Description=AA Firewall Sentinel Client (registration + policy sync)
After=network-online.target aafirewall-sentinel.service
Wants=network-online.target
Requires=aafirewall-sentinel.service

[Service]
Type=simple
ExecStart=/usr/local/bin/aafirewall-client
Restart=always
RestartSec=5
User=root
EnvironmentFile=/etc/aafirewall/sentinel.env

[Install]
WantedBy=multi-user.target
```

`Requires=aafirewall-sentinel.service` ensures the client is stopped if the daemon stops, matching today's KeepAlive semantics.

### 3.4 Convert `uninstall.sh`

**File:** [scripts/uninstall.sh](../scripts/uninstall.sh)

- Lines 70-84: replace the `PLISTS=(...)` array + `launchctl bootout/unload` loop with:
  ```bash
  if [[ "$(detect_platform)" == "linux" ]]; then
    for unit in aafirewall-sentinel aafirewall-sentinel-client aafirewall-hub; do
      systemctl disable --now "$unit" 2>/dev/null || true
      rm -f "/etc/systemd/system/${unit}.service"
    done
    systemctl daemon-reload
  else
    # existing macOS launchctl block
  fi
  ```
- Line 129: `MANAGED_HOOKS="$(managed_hooks_path)"`.

### 3.5 Hub Deploy

[scripts/deploy_hub.sh](../scripts/deploy_hub.sh) is mostly portable today. The `chown root:wheel || chown root:root` at lines 342, 347, 374 already works on Ubuntu. To bring it under systemd as well, add an optional `--systemd` flag that emits:

```ini
# /etc/systemd/system/aafirewall-hub.service
[Unit]
Description=AA Firewall Management Hub
After=network-online.target postgresql.service
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/aafirewall-central
Restart=always
RestartSec=3
EnvironmentFile=/etc/aafirewall/hub.env
User=root
# Hardening (same as sentinel)
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/etc/aafirewall /var/log/aafirewall /var/run

[Install]
WantedBy=multi-user.target
```

The `nohup` path at [deploy_hub.sh](../scripts/deploy_hub.sh) line ~ (existing manual-launch block) stays as the fallback for non-systemd hosts (containers, dev loops).

### 3.6 Managed Hooks on Linux

Claude Code reads managed settings from a platform-specific path. On Ubuntu the chosen location is `/etc/claude-code/managed-settings.json` (root-owned, `0644`). The hook installer at [scripts/install-hooks.sh](../scripts/install-hooks.sh) and the hooks-emission block inside `deploy_sentinel.sh` (around lines 520-530) must:

1. Create `/etc/claude-code/` (mode `0755 root:root`) if missing.
2. Write `managed-settings.json` from the existing template at [scripts/claude-hooks-settings.json](../scripts/claude-hooks-settings.json).
3. `chmod 0644` and `chown root:root`.

Per-user fallback (developer-installable) remains `~/.claude/settings.json` — same on both platforms.

### 3.7 Validation

Acceptance test on a fresh Ubuntu 22.04 VM:

```bash
sudo ./scripts/prepare.sh
./scripts/build.sh
sudo ./scripts/setup-database.sh
./scripts/generate-certs.sh
sudo ./scripts/deploy_hub.sh --systemd
sudo AA_CENTRAL_URL=https://localhost:9200 ./scripts/deploy_sentinel.sh

systemctl status aafirewall-hub aafirewall-sentinel aafirewall-sentinel-client
sudo systemctl restart aafirewall-sentinel    # confirm Restart=always works
sudo kill -9 $(pgrep aafirewall-daemon)       # confirm systemd respawns it
./scripts/validate.sh --hub-token <token>     # all green

sudo ./scripts/uninstall.sh
systemctl list-units 'aafirewall-*'           # expect: 0 loaded units
ls /etc/aafirewall /etc/claude-code 2>/dev/null   # expect: gone
```

---

## 4. Part B — eBPF Kernel Enforcer

### 4.1 Goals

1. Implement a second `KernelEnforcer` ([kernel.go:65-82](../go/internal/enforcement/osguard/kernel.go#L65-L82)) backed by eBPF LSM programs.
2. Daemon picks `EbpfEnforcer` vs `StubEnforcer` based on `AA_OSGUARD_MODE` and platform/capability detection. Falls back to `StubEnforcer` on any load failure (with audit-loud logging).
3. Closes the raw-terminal-bypass gap: even if the developer runs `cat ~/.ssh/id_rsa` directly outside Claude Code, the kernel hook denies it.

### 4.2 Files to Add

```
go/internal/enforcement/osguard/
├── kernel.go              (exists — no changes)
├── stub.go                (exists — no changes)
├── osguard_test.go        (exists — no changes)
├── ebpf_enforcer.go       NEW — Go loader, implements KernelEnforcer
├── ebpf_enforcer_linux.go NEW — //go:build linux: real implementation
├── ebpf_enforcer_other.go NEW — //go:build !linux: returns ErrUnsupported
├── ebpf_enforcer_test.go  NEW — integration tests, behind //go:build ebpf
├── factory.go             NEW — NewKernelEnforcer(mode) -> KernelEnforcer
└── bpf/
    ├── common.h           NEW — shared C structs and map limits
    ├── file_guard.c       NEW — bpf_lsm/file_open
    ├── exec_guard.c       NEW — bpf_lsm/bprm_check_security
    ├── connect_guard.c    NEW — bpf_lsm/socket_connect
    └── Makefile           NEW — vmlinux.h + clang -target bpf
```

The existing `KernelEnforcer` interface, types, and `StubEnforcer` are untouched. New code is additive.

### 4.3 Build-Tag Strategy

Two `//go:build` constraints serve two purposes:

- `//go:build linux` vs `//go:build !linux`: keeps macOS dev builds compiling. The non-Linux file returns `ErrUnsupported` from `Init()`, so the daemon falls back to `StubEnforcer`.
- `//go:build ebpf`: gates integration tests that require root + BPF LSM (so `go test ./...` on a developer laptop does not try to mmap kernel memory).

### 4.4 The Factory

`factory.go`:

```go
package osguard

import (
    "errors"
    "log/slog"
    "os"
)

func NewKernelEnforcer() KernelEnforcer {
    mode := os.Getenv("AA_OSGUARD_MODE")
    if mode == "" || mode == "off" {
        return NewStubEnforcer()
    }
    e := newEbpfEnforcer() // platform-conditional; nil on unsupported
    if e == nil {
        slog.Warn("ebpf enforcer unavailable on this platform; falling back to stub",
            "mode", mode)
        return NewStubEnforcer()
    }
    return e
}

var ErrUnsupported = errors.New("ebpf enforcer not supported on this platform")
```

Daemon code only ever calls `osguard.NewKernelEnforcer()` — keeps platform branching out of `server.go`.

### 4.5 Wiring into the Daemon

**File:** [go/internal/daemon/server.go](../go/internal/daemon/server.go) `StartDaemon` (around line 92-170).

After the policy bundle loads but before the HTTP server starts, add:

```go
enforcer := osguard.NewKernelEnforcer()
mode := os.Getenv("AA_OSGUARD_MODE")
if mode == "enforce" || mode == "audit" {
    cfg := osguard.EnforcerConfig{
        Mode:          mode,
        WorkspaceRoot: workspaceRoot, // already known to the daemon
        GovernedUIDs:  governedUIDsFromConfig(),
        DeniedPaths:   policyBundle.DeniedPaths(),   // new helper
        DeniedExecs:   policyBundle.DeniedExecs(),   // new helper
        AllowedHosts:  allowlist.AllHosts(),         // new helper
    }
    if err := enforcer.Init(cfg); err != nil {
        if strict {
            return fmt.Errorf("kernel enforcer init failed in strict mode: %w", err)
        }
        slog.Error("kernel enforcer init failed; continuing without OS guard", "err", err)
    }
    if err := enforcer.RegisterPolicy(buildKernelRules(policyBundle)); err != nil {
        slog.Error("kernel enforcer rule registration failed", "err", err)
    }
    defer enforcer.Shutdown()
}
```

The strict-mode branch matters: when `AA_STRICT_MODE=true` (the default in [deploy_sentinel.sh:49](../scripts/deploy_sentinel.sh#L49)), failure to load the kernel enforcer is fatal — operators want to know rather than silently downgrade.

When the policy bundle hot-reloads (existing flow in the daemon), call `enforcer.RegisterPolicy(...)` again to push updated maps into the kernel.

### 4.6 Policy Engine Helpers

**File:** [go/internal/policy/engine.go](../go/internal/policy/engine.go) (and/or `bundle.go` if rule storage lives there).

Add three pure functions on `*PolicyBundle`:

```go
// DeniedPaths returns absolute path globs the kernel should always deny.
// Sourced from rules with type=file.* and decision=deny.
func (b *PolicyBundle) DeniedPaths() []string

// DeniedExecs returns executable basenames that require approval/denial.
// Sourced from rules with type=process.execve and decision=deny.
func (b *PolicyBundle) DeniedExecs() []string
```

**File:** [go/internal/policy/allowlist.go](../go/internal/policy/allowlist.go).

Add:

```go
// AllHosts returns every allowlisted host, suitable for kernel-side connect()
// gating. The kernel enforcer denies anything not in this set when in enforce mode.
func (a *NetworkAllowlist) AllHosts() []string
```

Each helper must be deterministic (sorted output) so map updates in BPF are diff-stable.

### 4.7 BPF Programs

The C programs follow the templates in [AA_Firewall_Ubuntu_Kernel_Enforcer.md](AA_Firewall_Ubuntu_Kernel_Enforcer.md) Section "Implementation Steps" (steps 1-4). Implementation notes:

- **`common.h`**: define `MAX_PATH_LEN=256`, `MAX_HOST_LEN=128`, `MAX_ENTRIES=1024`. Keep `struct af_event` ≤ 512 bytes so the ring buffer at 256 KiB holds ~500 events between drains.
- **`file_guard.c`** (`SEC("lsm/file_open")`): uses `bpf_d_path()` to materialize the absolute path. `bpf_d_path` is gated to a fixed allowlist of LSM hooks in the kernel — `file_open` is on it. Workspace prefix matching requires unrolled loops; cap the prefix at 64 bytes for a single-pass check, fall back to user-space evaluation for longer prefixes.
- **`exec_guard.c`** (`SEC("lsm/bprm_check_security")`): pull `bprm->filename` and lookup basename in `denied_execs`. Argv inspection is too expensive in BPF — leave that to the user-space daemon's existing `evaluate` endpoint.
- **`connect_guard.c`** (`SEC("lsm/socket_connect")`): inspect `sockaddr` family. Handle `AF_INET` (parse `sin_addr.s_addr` to dotted-quad string for map lookup) and `AF_INET6`. DNS-name allowlists are resolved user-side and pushed in as IPs.

All three programs gate on `governed_uids` first — if the calling UID is not governed, return `0` immediately. This keeps overhead near-zero for non-agent processes.

### 4.8 Go Loader (`ebpf_enforcer_linux.go`)

Use `bpf2go` (cilium/ebpf code generation):

```go
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64 -target arm64 fileGuard ./bpf/file_guard.c -- -I./bpf
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64 -target arm64 execGuard ./bpf/exec_guard.c -- -I./bpf
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64 -target arm64 connectGuard ./bpf/connect_guard.c -- -I./bpf
```

`Init()` performs:

1. `rlimit.RemoveMemlock()` — required for older kernels.
2. Load three program collections (`loadFileGuardObjects` etc.).
3. Attach with `link.AttachLSM`.
4. Populate `config_map` (one entry, key=0) and `governed_uids` map.
5. Push initial denied paths/execs/hosts via `RegisterPolicy`.
6. Spawn a goroutine reading the ring buffer; on each event, write a `KernelInvocation` record to the existing audit pipeline (parallel to `StubEnforcer`'s `build/osguard-invocations.jsonl`).

`RegisterPolicy()` performs an atomic-ish swap: walk the new rule set, compute the diff against the current map keys, batch `Update`/`Delete` calls. cilium/ebpf provides batched ops; use them to keep the policy reload tight.

`EvaluateSyscall()` is mostly a no-op for the eBPF path (the kernel already decided). Return `Allow=true, ReasonCode="EBPF_IN_KERNEL"`. The interface contract stays satisfied for the user-space `/v1/evaluate` endpoint.

`Shutdown()` detaches every link, closes the ring buffer reader, closes the collections.

### 4.9 HTTP API Surface

**File:** [go/internal/daemon/server.go](../go/internal/daemon/server.go) router switch.

Add one route:

```
GET /v1/osguard/metrics    -> calls enforcer.GetMetrics(), returns JSON
```

Response shape (matches [KernelMetrics](../go/internal/enforcement/osguard/kernel.go#L120-L128)):

```json
{
  "mode": "enforce",
  "total_evaluated": 1847,
  "total_denied": 23,
  "total_allowed": 1824,
  "total_logged": 0,
  "by_syscall_type": {"file.open": 1200, "process.execve": 347, "network.connect": 300},
  "by_decision": {"allow": 1824, "deny": 23},
  "uptime_seconds": 3600
}
```

Authentication: same operator-token check used by `/v1/metrics` today.

A second optional route, `GET /v1/osguard/recent?limit=N`, drains the most recent kernel events from an in-memory ring (separate from the BPF ring buffer — that gets fully consumed by the audit pipeline). Useful for the Hub Console "what's the kernel seeing right now?" panel.

### 4.10 Tests

**Unit tests** — `ebpf_enforcer_test.go` without the `ebpf` tag:
- Map-key encoding (path → fixed-width buffer) round-trip
- Diff computation for `RegisterPolicy` (add / remove / unchanged)
- Event decoding from a hand-crafted byte buffer

**Integration tests** — same file, behind `//go:build ebpf`:
- Run as `sudo go test -tags=ebpf -v ./go/internal/enforcement/osguard/ -run TestEbpf`
- Cover: BPF load success; file open allowed inside workspace; file open denied on `~/.ssh/id_rsa`; execve of `/bin/curl` denied; `connect()` to allowlisted host allowed; `connect()` to non-allowlisted denied; non-governed UID skips all checks; policy update propagates within 100 ms; ring-buffer events match expected schema; counters increment.

CI: gate the `ebpf` tag tests behind a self-hosted runner with BPF LSM enabled. Default GitHub-hosted runners do not have LSM=bpf in their grub config.

### 4.11 Rollout Phases

1. **Stub continues to ship** — `AA_OSGUARD_MODE` unset → `StubEnforcer` (today's behavior). No regression for existing deployments.
2. **Audit on opt-in machines** — `AA_OSGUARD_MODE=audit`. Kernel enforcer loads, logs every decision, blocks nothing. Compare audit logs against the application-layer enforcement decisions for a week. False-positive rate must be <1% before moving on.
3. **Selective enforcement** — set `governed_uids` to one or two test users; enforcement on for them, audit for everyone else.
4. **General enforcement** — `AA_OSGUARD_MODE=enforce` with all dev UIDs governed.
5. **Strict mode** — strict-mode failures (`Init` error) become fatal. Operators must keep BPF LSM enabled on their kernels.

---

## 5. Cross-Cutting Concerns

### 5.1 Strict-Mode Semantics

The daemon already honors `AA_STRICT_MODE` ([deploy_sentinel.sh:49](../scripts/deploy_sentinel.sh#L49)). Two new failure modes inherit it:

| Failure | `AA_STRICT_MODE=true` | `AA_STRICT_MODE=false` |
|---|---|---|
| `EbpfEnforcer.Init` fails (no BPF LSM, no root, kernel < 5.15) | Daemon refuses to start | Falls back to `StubEnforcer`; log error |
| `RegisterPolicy` fails mid-flight | Daemon halts; operator must reload | Log error; continue with stale rules |

This is consistent with how the existing daemon treats database connectivity failures — fail loud in strict mode.

### 5.2 Capability Detection

Before attempting `EbpfEnforcer.Init`, probe:
- Kernel version `>= 5.15` (BPF LSM stable)
- `/sys/kernel/security/lsm` contains `bpf`
- Effective caps include `CAP_BPF` and `CAP_PERFMON` (root has both)

Surface the probe result at `/v1/osguard/metrics` even when the enforcer didn't load, so operators can see *why*.

### 5.3 Audit Pipeline Integration

`StubEnforcer` writes to `build/osguard-invocations.jsonl` ([stub.go:71](../go/internal/enforcement/osguard/stub.go) — verified by survey). The eBPF enforcer must **not** duplicate that file path; instead, it routes ring-buffer events through the existing audit buffer + PostgreSQL store ([server.go](../go/internal/daemon/server.go) lines ~121-147 per survey). Append-only contract holds — kernel events become first-class audit rows alongside hook-generated rows.

### 5.4 Performance Budget

Per-syscall overhead target: **< 5 µs at the 99th percentile** for governed UIDs, **< 100 ns** for non-governed (early return). Budget rationale: a developer's shell session issues ~10 syscalls/sec on average; even 5 µs × 10/sec is 50 µs/sec of overhead — invisible. The map lookups are O(1) and the ring-buffer write is O(struct size). Verify under `bpftool prog profile` once the programs land.

---

## 6. Milestones

| # | Milestone | Definition of Done |
|---|---|---|
| M1 | Ubuntu deploy works without eBPF | Fresh Ubuntu 22.04 VM, `deploy_sentinel.sh` succeeds, `systemctl status aafirewall-sentinel` shows `active (running)`, `validate.sh` green |
| M2 | Uninstall is clean on Ubuntu | After `uninstall.sh`, no aafirewall units, no `/etc/aafirewall`, no `/etc/claude-code/managed-settings.json` |
| M3 | `cilium/ebpf` skeleton compiles | `go build ./...` clean on Linux + macOS; new package present; `NewKernelEnforcer()` returns `StubEnforcer` until `AA_OSGUARD_MODE` is set |
| M4 | BPF programs load in audit mode | On a BPF-LSM-enabled kernel, `AA_OSGUARD_MODE=audit` daemon starts, ring buffer events flow into audit log, denied operations are logged but allowed |
| M5 | Enforcement | `AA_OSGUARD_MODE=enforce` blocks `cat ~/.ssh/id_rsa` from a non-Claude shell; audit row written; metrics counters tick |
| M6 | Strict-mode + fallback | With `AA_STRICT_MODE=true` and BPF LSM disabled, daemon refuses to start with a clear error; with strict mode off, daemon falls back to `StubEnforcer` and logs why |
| M7 | Hub Console surfaces kernel state | `/v1/osguard/metrics` wired to the existing console dashboard; operators see live deny counts |

Each milestone is independently mergeable behind the existing `git` workflow. No bundled commits — split per milestone, per the project rules.

---

## 7. Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Ubuntu users on kernels without BPF LSM (older 20.04, custom kernels) | Capability probe + `StubEnforcer` fallback + clear error in `/v1/osguard/metrics`; documented kernel requirement in `prepare.sh` |
| `bpf_d_path()` truncation on long paths (>256 bytes) | Fall back to user-space evaluation for paths that hit the limit; emit a metric so we know how often it happens |
| systemd `ProtectHome=read-only` blocks legitimate file writes during enforcement decisions | Carve out only required ReadWritePaths; test with `systemd-analyze security aafirewall-sentinel` |
| Policy churn → frequent BPF map updates → kernel verifier overhead | Diff-based `RegisterPolicy` batches updates; throttle to one reload per second under load |
| Developer notices the eBPF enforcer and tries to unload it | LSM programs cannot be detached without `CAP_SYS_ADMIN`; daemon runs as root and the developer does not |
| Doc drift between this implementation plan and [AA_Firewall_Ubuntu_Kernel_Enforcer.md](AA_Firewall_Ubuntu_Kernel_Enforcer.md) | This doc cites the design doc by section; update both together when interfaces change |

---

## 8. Open Questions for Developer Review

The architecture-decision-gate items per [CLAUDE.md](../CLAUDE.md):

1. **Hub Console deployment on Ubuntu** — same systemd unit as the Mac plist, or a separate `aafirewall-hub-console` unit? Plist today couples them.
2. **Managed hooks path** — `/etc/claude-code/managed-settings.json` is the proposal. Anthropic has not formally documented the Linux managed-settings path; need to confirm with Claude Code release notes before shipping.
3. **eBPF in containers** — Docker rootless mode (referenced in Phase 1 TDD) typically lacks `CAP_BPF`. Should the daemon detect container-mode and disable the eBPF path automatically, or should operators opt in?
4. **Single-binary embedding of BPF objects** — `bpf2go` embeds the bytecode in the Go binary, but separate BPF objects per kernel arch (`amd64`, `arm64`) inflate binary size. Acceptable, or split into per-arch builds?

These should be resolved before Milestone M4 lands.

---

## 9. References

- Design doc: [AA_Firewall_Ubuntu_Kernel_Enforcer.md](AA_Firewall_Ubuntu_Kernel_Enforcer.md)
- Final TDD: [AA_Firewall_TDD_Final_2.md](AA_Firewall_TDD_Final_2.md)
- Final PRD: [AA_Firewall_PRD_Final.md](AA_Firewall_PRD_Final.md)
- Existing setup guide: [AA_Firewall_SETUP.md](AA_Firewall_SETUP.md)
- Project rules (mandatory): [../CLAUDE.md](../CLAUDE.md)
