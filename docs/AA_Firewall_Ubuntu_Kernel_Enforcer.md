> Author: Deepankar Das

# AA Firewall — Ubuntu Deployment + Kernel Enforcer (eBPF)

> How to deploy AA Firewall on Ubuntu, plus the implementation guide for the eBPF kernel enforcer.

---

## Part 1: Running AA Firewall on Ubuntu

### Ubuntu Compatibility

| Component | Ubuntu-Ready? | Notes |
|---|---|---|
| Go binaries (all 5) | Yes | Cross-compile on macOS or build natively |
| `setup-database.sh` | Yes | Has `apt-get` and `systemctl` fallbacks |
| `deploy_hub.sh` | Mostly | Uses `nohup` for process management (works), minor `chown root:wheel` has `root:root` fallback |
| `deploy_sentinel.sh` | No | Uses macOS LaunchDaemons — needs systemd conversion |
| `uninstall.sh` | No | Same LaunchDaemon issue |
| `validate.sh` | Yes | Pure curl + node |
| `prepare.sh` | Yes | Has `apt-get` fallbacks |
| `build.sh` | Yes | Standard npm + Go toolchain |

### What Needs Systemd Equivalents on Ubuntu

The macOS deploy scripts use LaunchDaemons (plist + launchctl). On Ubuntu these need systemd unit files:

| macOS | Ubuntu Equivalent |
|---|---|
| `/Library/LaunchDaemons/com.aafirewall.sentinel.plist` | `/etc/systemd/system/aafirewall-sentinel.service` |
| `/Library/LaunchDaemons/com.aafirewall.sentinel-client.plist` | `/etc/systemd/system/aafirewall-sentinel-client.service` |
| `launchctl load <plist>` | `systemctl enable --now aafirewall-sentinel` |
| `launchctl unload <plist>` | `systemctl disable --now aafirewall-sentinel` |
| `launchctl bootout system <plist>` | `systemctl stop aafirewall-sentinel` |
| `/Library/Application Support/ClaudeCode/managed-settings.json` | `/etc/claude-code/managed-settings.json` (or per-user `~/.claude/settings.json`) |
| `chown root:wheel` | `chown root:root` (fallback already exists in scripts) |

### Quick Start on Ubuntu (Manual — No Deploy Script)

Until the deploy scripts are converted to systemd, run the binaries directly:

```bash
# 1. Install prerequisites
sudo apt update
sudo apt install -y golang-go nodejs npm postgresql postgresql-contrib

# 2. Start PostgreSQL
sudo systemctl start postgresql
sudo systemctl enable postgresql

# 3. Clone and build
git clone <repo-url> AAFirewall && cd AAFirewall
./scripts/prepare.sh
./scripts/build.sh

# 4. Set up the database
sudo ./scripts/setup-database.sh

# 5. Generate certificates
./scripts/generate-certs.sh

# 6. Start the Hub (background)
export DATABASE_URL="$(sudo cat /etc/aafirewall/.db_credentials)"
export CERT_DIR="/etc/aafirewall/certs"
nohup ./go/bin/aafirewall-central > /var/log/aafirewall/hub.log 2>&1 &

# 7. Start the Sentinel daemon (background)
nohup ./go/bin/aafirewall-daemon > /var/log/aafirewall/sentinel.log 2>&1 &

# 8. Install hooks for Claude Code
mkdir -p ~/.claude
# Copy managed-settings.json or project-level .claude/settings.json
# (see AA_Firewall_SETUP.md for hook JSON format)

# 9. Validate
./scripts/validate.sh --hub-token <your-admin-token>
```

### Creating Systemd Service Files (Optional)

Example service file for the Sentinel daemon:

```ini
# /etc/systemd/system/aafirewall-sentinel.service
[Unit]
Description=AA Firewall Sentinel Daemon
After=network.target postgresql.service

[Service]
Type=simple
ExecStart=/usr/local/bin/aafirewall-daemon
Restart=always
RestartSec=3
Environment=DATABASE_URL=postgresql://aafirewall:PASSWORD@localhost:5432/aa_firewall?sslmode=prefer
Environment=AA_STRICT_MODE=true

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now aafirewall-sentinel
```

---

## Part 2: Kernel Enforcer (eBPF)

> Implementation guide for the OS-level kernel enforcer on Ubuntu using eBPF.
> This closes the "raw terminal bypass" gap: even if a developer bypasses Claude Code hooks and runs commands directly in a terminal, the kernel enforcer intercepts the syscall before it executes.

---

## Overview

The kernel enforcer attaches eBPF programs to Linux Security Module (LSM) hooks. These programs run inside the kernel's BPF virtual machine and can allow or deny syscalls before they complete. The enforcer implements the existing `KernelEnforcer` interface defined in `go/internal/enforcement/osguard/kernel.go` — the daemon code doesn't change.

**What it intercepts:**

| Syscall | LSM Hook | What It Governs |
|---|---|---|
| `file.open` | `bpf_lsm/file_open` | File reads/writes outside project directory, sensitive path access |
| `process.execve` | `bpf_lsm/bprm_check_security` | Destructive commands (`rm`, `curl`, etc.) |
| `network.connect` | `bpf_lsm/socket_connect` | Egress to non-allowlisted hosts |

**Requirements:**
- Ubuntu 22.04+ (kernel 5.15+ with BPF LSM enabled)
- Root access (or `CAP_BPF` + `CAP_PERFMON`)
- Build tools: `clang`, `llvm`, `libbpf-dev`, `linux-headers-$(uname -r)`

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│  User Space                                         │
│                                                     │
│  ┌───────────────┐      ┌────────────────────────┐  │
│  │ aafirewall-   │      │ EbpfEnforcer           │  │
│  │ daemon        │─────▶│ (Go, cilium/ebpf)      │  │
│  │               │      │                        │  │
│  │ Policy Engine │      │ - Loads BPF programs   │  │
│  │ Audit Store   │      │ - Updates BPF maps     │  │
│  │ HTTP API      │◀─────│ - Reads event ring buf │  │
│  └───────────────┘      └──────────┬─────────────┘  │
│                                    │ bpf() syscall   │
├────────────────────────────────────┼────────────────┤
│  Kernel Space                      │                 │
│                                    ▼                 │
│  ┌─────────────────────────────────────────────────┐ │
│  │ BPF Programs (attached to LSM hooks)            │ │
│  │                                                 │ │
│  │  file_open_guard ──── bpf_lsm/file_open         │ │
│  │  exec_guard ───────── bpf_lsm/bprm_check        │ │
│  │  connect_guard ────── bpf_lsm/socket_connect     │ │
│  │                                                 │ │
│  │  Policy lookup via BPF maps (O(1)):             │ │
│  │   - denied_paths_map (hash map)                 │ │
│  │   - denied_execs_map (hash map)                 │ │
│  │   - allowed_hosts_map (hash map)                │ │
│  │   - governed_uids_map (array)                   │ │
│  │   - config_map (array: workspace root, mode)    │ │
│  │                                                 │ │
│  │  Events → BPF ring buffer → user space          │ │
│  └─────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

---

## Prerequisites (Ubuntu)

```bash
# Verify kernel supports BPF LSM
cat /boot/config-$(uname -r) | grep BPF_LSM
# Expected: CONFIG_BPF_LSM=y

# Check LSM is enabled (must include "bpf" in the list)
cat /sys/kernel/security/lsm
# If "bpf" is missing, add it to GRUB:
#   GRUB_CMDLINE_LINUX="lsm=lockdown,capability,landlock,yama,apparmor,bpf"
#   sudo update-grub && sudo reboot

# Install build tools
sudo apt update
sudo apt install -y clang llvm libbpf-dev linux-headers-$(uname -r) golang-go

# Verify
clang --version    # 14+
go version         # 1.21+
```

---

## File Structure

```
go/internal/enforcement/osguard/
├── kernel.go              # KernelEnforcer interface (exists)
├── stub.go                # StubEnforcer (exists)
├── ebpf_enforcer.go       # EbpfEnforcer — Go loader (new)
├── ebpf_enforcer_test.go  # Integration tests (new)
├── bpf/
│   ├── file_guard.c       # BPF program: file.open governance (new)
│   ├── exec_guard.c       # BPF program: execve governance (new)
│   ├── connect_guard.c    # BPF program: connect governance (new)
│   ├── common.h           # Shared types, map definitions (new)
│   └── Makefile           # Compiles .c → .o (BPF bytecode) (new)
├── bpf_generated.go       # go:generate output from bpf2go (new)
└── osguard_test.go        # Existing stub tests (exists)
```

---

## Implementation Steps

### Step 1: BPF shared types (`bpf/common.h`)

Define the shared structures used by BPF programs and the Go loader:

```c
#ifndef __AAFIREWALL_COMMON_H
#define __AAFIREWALL_COMMON_H

#define MAX_PATH_LEN 256
#define MAX_HOST_LEN 128
#define MAX_ENTRIES  1024

// Event sent from BPF to user space via ring buffer
struct af_event {
    __u32 pid;
    __u32 uid;
    __u64 timestamp;
    __u32 syscall_type;    // 0=file_open, 1=execve, 2=connect
    __u32 decision;        // 0=allow, 1=deny
    char  path[MAX_PATH_LEN];
    char  comm[16];        // process name
};

// Config pushed from user space
struct af_config {
    __u32 mode;            // 0=off, 1=audit, 2=enforce
    __u32 workspace_len;
    char  workspace_root[MAX_PATH_LEN];
};

#endif
```

### Step 2: File guard BPF program (`bpf/file_guard.c`)

```c
// SPDX-License-Identifier: GPL-2.0
#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include "common.h"

// Maps
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, char[MAX_PATH_LEN]);
    __type(value, __u8);
} denied_paths SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, struct af_config);
} config_map SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 64);
    __type(key, __u32);
    __type(value, __u8);
} governed_uids SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} events SEC(".maps");

SEC("lsm/file_open")
int BPF_PROG(file_open_guard, struct file *file, int ret)
{
    // If a prior LSM already denied, respect that
    if (ret != 0)
        return ret;

    __u32 uid = bpf_get_current_uid_gid() & 0xFFFFFFFF;
    __u32 pid = bpf_get_current_pid_tgid() >> 32;

    // Check if this UID is governed
    if (!bpf_map_lookup_elem(&governed_uids, &uid))
        return 0; // Not governed, allow

    // Read config
    __u32 key = 0;
    struct af_config *cfg = bpf_map_lookup_elem(&config_map, &key);
    if (!cfg || cfg->mode == 0)
        return 0; // Disabled

    // Get file path (best effort — BPF has limited string ops)
    char path[MAX_PATH_LEN] = {};
    bpf_d_path(&file->f_path, path, sizeof(path));

    // Check denied paths
    if (bpf_map_lookup_elem(&denied_paths, path)) {
        // Emit event
        struct af_event *evt = bpf_ringbuf_reserve(&events, sizeof(*evt), 0);
        if (evt) {
            evt->pid = pid;
            evt->uid = uid;
            evt->timestamp = bpf_ktime_get_ns();
            evt->syscall_type = 0;
            evt->decision = 1; // deny
            __builtin_memcpy(evt->path, path, MAX_PATH_LEN);
            bpf_get_current_comm(evt->comm, sizeof(evt->comm));
            bpf_ringbuf_submit(evt, 0);
        }

        if (cfg->mode == 2) // enforce
            return -EPERM;
    }

    // Check workspace boundary
    // (simplified — full prefix matching in BPF requires loop unrolling)

    return 0;
}

char LICENSE[] SEC("license") = "GPL";
```

### Step 3: Exec guard (`bpf/exec_guard.c`)

Same pattern as file guard, attached to `bpf_lsm/bprm_check_security`:

- Looks up the executable name in `denied_execs` BPF map
- If matched and mode is `enforce`, returns `-EPERM`
- Emits event to ring buffer

### Step 4: Connect guard (`bpf/connect_guard.c`)

Attached to `bpf_lsm/socket_connect`:

- Extracts destination IP/port from `sockaddr`
- Looks up in `allowed_hosts` BPF map
- If not found and mode is `enforce`, returns `-EPERM`
- Emits event to ring buffer

### Step 5: Build BPF bytecode (`bpf/Makefile`)

```makefile
CLANG ?= clang
ARCH := $(shell uname -m | sed 's/x86_64/x86/' | sed 's/aarch64/arm64/')

TARGETS = file_guard.o exec_guard.o connect_guard.o

all: vmlinux.h $(TARGETS)

vmlinux.h:
	bpftool btf dump file /sys/kernel/btf/vmlinux format c > vmlinux.h

%.o: %.c common.h vmlinux.h
	$(CLANG) -g -O2 -target bpf -D__TARGET_ARCH_$(ARCH) \
		-I. -c $< -o $@

clean:
	rm -f *.o vmlinux.h
```

### Step 6: Go loader (`ebpf_enforcer.go`)

Uses `cilium/ebpf` (pure Go, no CGO) to:

1. Load compiled BPF objects (`.o` files)
2. Attach programs to LSM hooks
3. Populate BPF maps with policy rules from the daemon
4. Read events from ring buffer and route to audit pipeline

```go
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64 enforcer ./bpf/file_guard.c -- -I./bpf

package osguard

import (
    "github.com/cilium/ebpf"
    "github.com/cilium/ebpf/link"
    "github.com/cilium/ebpf/ringbuf"
)

type EbpfEnforcer struct {
    config    EnforcerConfig
    objs      enforcerObjects    // generated by bpf2go
    links     []link.Link
    reader    *ringbuf.Reader
    metrics   KernelMetrics
    // ... same fields as StubEnforcer for metrics
}

func NewEbpfEnforcer() *EbpfEnforcer {
    return &EbpfEnforcer{}
}

func (e *EbpfEnforcer) Init(config EnforcerConfig) error {
    // 1. Load BPF objects
    // 2. Attach to LSM hooks
    // 3. Populate maps with config (workspace root, governed UIDs)
    // 4. Start ring buffer reader goroutine
    return nil
}

func (e *EbpfEnforcer) RegisterPolicy(rules []KernelRule) error {
    // Update BPF maps: denied_paths, denied_execs, allowed_hosts
    return nil
}

func (e *EbpfEnforcer) EvaluateSyscall(req SyscallRequest) SyscallDecision {
    // For user-space callers (daemon evaluate endpoint).
    // The real enforcement happens in-kernel via BPF programs.
    // This method exists for the interface but most decisions
    // are made in-kernel before this is ever called.
    return SyscallDecision{Allow: true, ReasonCode: "EBPF_IN_KERNEL"}
}

func (e *EbpfEnforcer) GetMetrics() KernelMetrics {
    return e.metrics
}

func (e *EbpfEnforcer) Shutdown() error {
    // Detach BPF programs, close ring buffer, free maps
    return nil
}
```

### Step 7: Wire into daemon

In the daemon startup, check `AA_OSGUARD_MODE`:

```go
// In daemon/server.go or cmd/daemon/main.go
mode := os.Getenv("AA_OSGUARD_MODE") // "enforce", "audit", or "off"
if mode == "enforce" || mode == "audit" {
    enforcer := osguard.NewEbpfEnforcer()
    err := enforcer.Init(osguard.EnforcerConfig{
        Mode:          mode,
        WorkspaceRoot: workspaceRoot,
        GovernedUIDs:  []int{uid},
        DeniedPaths:   policyEngine.GetDeniedPaths(),
        DeniedExecs:   policyEngine.GetDeniedExecs(),
        AllowedHosts:  policyEngine.GetAllowedHosts(),
    })
    if err != nil {
        slog.Error("Failed to init kernel enforcer", "error", err)
        // Fall back to stub or fail based on strict mode
    }
    defer enforcer.Shutdown()
}
```

### Step 8: Add API endpoint

Expose kernel metrics and recent events at `/v1/osguard/metrics`:

```json
{
  "mode": "enforce",
  "total_evaluated": 1847,
  "total_denied": 23,
  "total_allowed": 1824,
  "by_syscall_type": {
    "file.open": 1200,
    "process.execve": 347,
    "network.connect": 300
  },
  "uptime_seconds": 3600
}
```

---

## Build and Test on Ubuntu

```bash
# 1. Install prerequisites
sudo apt update
sudo apt install -y clang llvm libbpf-dev linux-headers-$(uname -r) bpftool

# 2. Verify BPF LSM is enabled
cat /sys/kernel/security/lsm | grep bpf
# If missing, enable it:
#   sudo sed -i 's/GRUB_CMDLINE_LINUX=""/GRUB_CMDLINE_LINUX="lsm=lockdown,capability,landlock,yama,apparmor,bpf"/' /etc/default/grub
#   sudo update-grub && sudo reboot

# 3. Build BPF bytecode
cd go/internal/enforcement/osguard/bpf
make

# 4. Generate Go bindings
cd ../
go generate ./...

# 5. Build daemon with eBPF support
cd ../../..
make build

# 6. Run with kernel enforcement
sudo AA_OSGUARD_MODE=enforce ./bin/aafirewall-daemon

# 7. Test: try accessing a denied path from another terminal
cat ~/.ssh/id_rsa    # Should be denied by kernel enforcer

# 8. Check metrics
curl -s http://localhost:9100/v1/osguard/metrics | python3 -m json.tool
```

---

## Testing Strategy

### Unit tests (run without root)

Test the Go loader logic, map population, and event parsing using mock BPF objects.

### Integration tests (require root + BPF LSM)

```bash
sudo go test -v ./internal/enforcement/osguard/ -tags=ebpf -run TestEbpf
```

Tests:
1. Load BPF programs successfully
2. File open inside workspace — allowed
3. File open on denied path — denied (enforce mode) or logged (audit mode)
4. Execve of denied binary — denied
5. Connect to non-allowlisted host — denied
6. Connect to allowlisted host — allowed
7. Non-governed UID — all allowed (BPF skips)
8. Policy update propagates to BPF maps
9. Events appear in ring buffer
10. Metrics counters increment correctly

### Audit mode (safe rollout)

Start with `AA_OSGUARD_MODE=audit` — logs all decisions but doesn't block anything. Review the audit trail, then switch to `enforce` once confident.

---

## Rollout Plan

1. **Audit mode first** — `AA_OSGUARD_MODE=audit` on a test machine. Verify events are logged correctly, no false positives on legitimate developer workflows.

2. **Selective enforcement** — Use `governed_uids` to enforce only for specific users while others remain in audit mode.

3. **Full enforcement** — `AA_OSGUARD_MODE=enforce` once audit data confirms no false positives.

4. **Fallback** — If the eBPF enforcer fails to load (missing kernel support, insufficient privileges), fall back to `StubEnforcer` and log a warning. The managed hooks layer continues to provide enforcement via Claude Code.

---

## Dependencies

| Package | Purpose | CGO Required |
|---|---|---|
| `github.com/cilium/ebpf` | Load BPF programs, manage maps, read ring buffer | No (pure Go) |
| `clang` + `llvm` | Compile BPF C programs to bytecode | Build-time only |
| `libbpf-dev` | BPF helper headers for C programs | Build-time only |
| `linux-headers` | Kernel headers for vmlinux.h generation | Build-time only |
| `bpftool` | Generate vmlinux.h from BTF | Build-time only |

Runtime dependencies: none beyond the Linux kernel itself. The compiled BPF bytecode is embedded in the Go binary via `go:embed` or `bpf2go`.

---

## Platform Notes

- **Ubuntu 22.04+**: Kernel 5.15+ with BPF LSM support. This is the target platform.
- **macOS**: Not applicable. macOS uses Endpoint Security Framework (System Extension), which requires Apple Developer credentials and notarization. Separate implementation needed.
- **Other Linux**: Any distribution with kernel 5.7+ and BPF LSM enabled should work. Tested on Ubuntu.
