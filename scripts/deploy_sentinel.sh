#!/bin/bash
# Enforcer — Sentinel Deployment
# Author: Deepankar Das
#
# Deploys the Enforcer Sentinel on a developer's machine.
# Safe to re-run — skips steps that are already complete.
# Must be run as root (sudo) — the developer must NOT run this themselves.
#
# Usage:
#   sudo AA_CENTRAL_URL=https://hub.company.com:9200 ./scripts/deploy_sentinel.sh
#   sudo ./scripts/deploy_sentinel.sh --status      # Check Sentinel status
#   sudo ./scripts/deploy_sentinel.sh --stop        # Stop the Sentinel
#   sudo ./scripts/deploy_sentinel.sh --restart     # Stop + start
#   sudo ./scripts/deploy_sentinel.sh --uninstall   # Remove everything
#   sudo ./scripts/deploy_sentinel.sh --seed-auth --seed-dev-user dev --seed-dev-password 'dev1'
#
# What this does (skips if already done):
#   1. Installs Sentinel binaries to /usr/local/bin/ (root-owned)
#   2. Sets up PostgreSQL with restricted enforcer user (developer locked out)
#   3. Copies Sentinel mTLS certificates
#   4. Installs managed Claude Code hooks (developer cannot remove)
#   5. Installs LaunchDaemon (auto-start, KeepAlive, root)
#   6. Starts Sentinel Server
#   7. Registers with Management Hub
#
# The developer cannot:
#   - Read database credentials (/etc/enforcer/.db_credentials is root:600)
#   - Remove managed hooks (allowManagedHooksOnly=true)
#   - Kill the Sentinel Server (LaunchDaemon auto-restarts)
#   - Access other developers' data (Sentinel Developer Console shows own data only)
#   - Modify policies (signed bundles from Hub)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

SENTINEL_DEPLOY_SUCCEEDED=false
on_exit() {
  local code=$?
  if [[ $code -ne 0 && "$SENTINEL_DEPLOY_SUCCEEDED" != "true" ]]; then
    banner "Sentinel Deploy Failed"
  fi
}
trap on_exit EXIT

# Configuration
CENTRAL_URL="${AA_CENTRAL_URL:-}"
STRICT_MODE="${AA_STRICT_MODE:-true}"
DAEMON_PORT="${DAEMON_PORT:-9100}"
CONSOLE_PORT="${CONSOLE_PORT:-6100}"
CONFIG_DIR="/etc/enforcer"
CERT_DIR="$CONFIG_DIR/certs"
LOG_DIR="/var/log/enforcer"
LOG_FILE="$LOG_DIR/sentinel.log"
CLIENT_LOG_FILE="$LOG_DIR/sentinel-client.log"
PID_FILE="/var/run/enforcer-sentinel.pid"
MANAGED_HOOKS_DIR="/Library/Application Support/ClaudeCode"
MANAGED_HOOKS_FILE="$MANAGED_HOOKS_DIR/managed-settings.json"
PLIST_FILE="/Library/LaunchDaemons/com.enforcer.sentinel.plist"
CLIENT_PLIST_FILE="/Library/LaunchDaemons/com.enforcer.sentinel-client.plist"
INSTALL_DIR="/usr/local/bin"
DEV_USER="${SUDO_USER:-$(logname 2>/dev/null || echo '')}"
AUTHSEED_BINARY="$PROJECT_ROOT/go/bin/enforcer-authseed"

ACTION="deploy"
SEED_AUTH=false
SEED_DEV_USER="${AA_SEED_DEV_USER:-${SUDO_USER:-$(logname 2>/dev/null || echo dev)}}"
SEED_DEV_PASSWORD="${AA_SEED_DEV_PASSWORD:-}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

log_ok()    { echo -e "  ${GREEN}[OK]${NC}    $1"; }
log_skip()  { echo -e "  ${GREEN}[SKIP]${NC}  $1 (already done)"; }
log_warn()  { echo -e "  ${YELLOW}[WARN]${NC}  $1"; }
log_error() { echo -e "  ${RED}[ERROR]${NC} $1"; }
log_step()  { echo -e "\n${BLUE}[$1]${NC} $2"; }
banner() {
  local phase="$1"
  echo ""
  echo -e "${CYAN}${BOLD}╔══════════════════════════════════════════════════════════════╗${NC}"
  printf "${CYAN}${BOLD}║${NC} %-60s ${CYAN}${BOLD}║${NC}\n" ""
  printf "${CYAN}${BOLD}║${NC} %-60s ${CYAN}${BOLD}║${NC}\n" "   _   _     ___ ___ ___ _____      ___   _    _"
  printf "${CYAN}${BOLD}║${NC} %-60s ${CYAN}${BOLD}║${NC}\n" "  /_\\ /_\\   | __|_ _| _ \\ __\\ \\    / /_\\ | |  | |"
  printf "${CYAN}${BOLD}║${NC} %-60s ${CYAN}${BOLD}║${NC}\n" " / _ \\ _ \\  | _| | ||   / _| \\ \\/\\/ / _ \\| |__| |__"
  printf "${CYAN}${BOLD}║${NC} %-60s ${CYAN}${BOLD}║${NC}\n" " /_/ \\_\\/\\_\\ |_| |___|_|_\\___|  \\_/\\_/_/ \\_\\____|____|"
  printf "${CYAN}${BOLD}║${NC} %-60s ${CYAN}${BOLD}║${NC}\n" ""
  printf "${CYAN}${BOLD}║${NC} %-60s ${CYAN}${BOLD}║${NC}\n" "$phase"
  echo -e "${CYAN}${BOLD}╚══════════════════════════════════════════════════════════════╝${NC}"
  echo ""
}

resolve_sentinel_console_url() {
  local candidates=(
    "http://localhost:${CONSOLE_PORT}/login/"
    "http://localhost:${CONSOLE_PORT}/"
    "http://localhost:${DAEMON_PORT}/login/"
    "http://localhost:${DAEMON_PORT}/"
  )
  local url
  for url in "${candidates[@]}"; do
    if curl -fsS --max-time 2 "$url" >/dev/null 2>&1; then
      echo "$url"
      return 0
    fi
  done
  return 1
}

seed_tokens_in_db() {
  local admin_token="$1"
  local reviewer_token="$2"
  local operator_token="$3"

  local db_url=""
  if [[ -f "$CONFIG_DIR/.db_credentials" ]]; then
    db_url="$(cat "$CONFIG_DIR/.db_credentials")"
  fi
  if [[ -z "$db_url" ]]; then
    log_warn "Skipping DB auth seed: missing $CONFIG_DIR/.db_credentials"
    return 0
  fi

  local cmd=()
  if [[ -x "$AUTHSEED_BINARY" ]]; then
    cmd=("$AUTHSEED_BINARY")
  elif command -v go >/dev/null 2>&1; then
    cmd=(go run "$PROJECT_ROOT/go/cmd/authseed")
  else
    log_error "Cannot seed auth tokens into DB: no $AUTHSEED_BINARY and 'go' not available"
    return 1
  fi

  local args=()
  [[ -n "$admin_token" ]] && args+=(--admin-token "$admin_token")
  [[ -n "$reviewer_token" ]] && args+=(--reviewer-token "$reviewer_token")
  [[ -n "$operator_token" ]] && args+=(--operator-token "$operator_token")
  if [[ ${#args[@]} -eq 0 ]]; then
    return 0
  fi

  if ! DATABASE_URL="$db_url" AA_CONFIG_DIR="$CONFIG_DIR" AA_AUTH_ENC_KEY_FILE="$AUTH_KEY_FILE" "${cmd[@]}" "${args[@]}"; then
    log_error "Failed to seed encrypted auth tokens into PostgreSQL"
    return 1
  fi
  log_ok "Seeded encrypted auth tokens into PostgreSQL"
}

# ── Argument parsing ────────────────────────────────────────────────────────

while [[ $# -gt 0 ]]; do
  case "$1" in
    --status)     ACTION="status"; shift ;;
    --stop)       ACTION="stop"; shift ;;
    --restart)    ACTION="restart"; shift ;;
    --uninstall)  ACTION="uninstall"; shift ;;
    --seed-auth)  SEED_AUTH=true; shift ;;
    --seed-dev-user)
      if [[ $# -lt 2 ]]; then log_error "Missing value for --seed-dev-user"; exit 1; fi
      SEED_DEV_USER="$2"; shift 2 ;;
    --seed-dev-password)
      if [[ $# -lt 2 ]]; then log_error "Missing value for --seed-dev-password"; exit 1; fi
      SEED_DEV_PASSWORD="$2"; shift 2 ;;
    -h|--help)
      echo "Usage: sudo AA_CENTRAL_URL=https://hub:9200 ./scripts/deploy_sentinel.sh [OPTIONS]"
      echo ""
      echo "Options:"
      echo "  --status       Check Sentinel status"
      echo "  --stop         Stop the Sentinel"
      echo "  --restart      Stop + start"
      echo "  --uninstall    Remove Sentinel completely"
      echo "  --seed-auth    Seed Sentinel Developer login values"
      echo "  --seed-dev-user USER"
      echo "                 Seeded Sentinel Developer username label (default: dev)"
      echo "  --seed-dev-password PASS"
      echo "                 Seeded Sentinel Developer access token"
      echo "  -h, --help     Show this help"
      echo ""
      echo "Environment:"
      echo "  AA_CENTRAL_URL  Management Hub URL (required for first deploy)"
      exit 0
      ;;
    *) log_error "Unknown option: $1"; exit 1 ;;
  esac
done

# ── Root check ──────────────────────────────────────────────────────────────

if [[ $EUID -ne 0 ]]; then
  echo -e "${RED}[ERROR]${NC} This script must be run as root (sudo)."
  echo "  The Sentinel manages security enforcement — it must run as a privileged service."
  echo "  The developer must NOT run this script themselves."
  exit 1
fi

# ── Helpers ─────────────────────────────────────────────────────────────────

sentinel_is_running() {
  [[ -f "$PID_FILE" ]] && kill -0 "$(cat "$PID_FILE" 2>/dev/null)" 2>/dev/null
}

sentinel_stop() {
  if sentinel_is_running; then
    kill "$(cat "$PID_FILE")" 2>/dev/null
    rm -f "$PID_FILE"
    sleep 1
    log_ok "Sentinel stopped"
  elif lsof -ti "tcp:$DAEMON_PORT" >/dev/null 2>&1; then
    lsof -ti "tcp:$DAEMON_PORT" | xargs kill -TERM 2>/dev/null
    sleep 1
    log_ok "Sentinel stopped (by port)"
  else
    log_skip "Sentinel is not running"
  fi
  # Also stop LaunchDaemon if loaded
  launchctl unload "$PLIST_FILE" 2>/dev/null || true
  launchctl unload "$CLIENT_PLIST_FILE" 2>/dev/null || true
  pkill -f "/usr/local/bin/enforcer-client" 2>/dev/null || true
}

# ── Status ──────────────────────────────────────────────────────────────────

if [[ "$ACTION" == "status" ]]; then
  echo -e "\n${CYAN}${BOLD}Enforcer — Sentinel Status${NC}\n"
  echo "  Developer: ${DEV_USER:-unknown}"
  sentinel_is_running && log_ok "Sentinel running (PID $(cat "$PID_FILE"))" || log_warn "Sentinel not running"
  lsof -i "tcp:$DAEMON_PORT" >/dev/null 2>&1 && log_ok "Sentinel Server API on port $DAEMON_PORT" || log_warn "Sentinel Server API not on port $DAEMON_PORT"
  lsof -i "tcp:$CONSOLE_PORT" >/dev/null 2>&1 && log_ok "Sentinel Developer Console on port $CONSOLE_PORT" || log_warn "Sentinel Developer Console not on port $CONSOLE_PORT"
  pg_isready >/dev/null 2>&1 && log_ok "PostgreSQL running" || log_warn "PostgreSQL not running"
  [[ -f "$MANAGED_HOOKS_FILE" ]] && log_ok "Managed hooks installed" || log_warn "Managed hooks not installed"
  [[ -f "$PLIST_FILE" ]] && log_ok "LaunchDaemon installed" || log_warn "LaunchDaemon not installed"
  [[ -f "$CLIENT_PLIST_FILE" ]] && log_ok "Client LaunchDaemon installed" || log_warn "Client LaunchDaemon not installed"
  [[ -f "$CONFIG_DIR/.db_credentials" ]] && log_ok "DB credentials (root:600)" || log_warn "No DB credentials"
  [[ -f "$CERT_DIR/client.crt" ]] && log_ok "Sentinel certificates present" || log_warn "No Sentinel certificates"
  pgrep -f "/usr/local/bin/enforcer-client" >/dev/null 2>&1 && log_ok "Sentinel client agent process running" || log_warn "Sentinel client agent process not running"
  if [[ -n "$DEV_USER" && "$DEV_USER" != "root" ]]; then
    if su "$DEV_USER" -c "cat $CONFIG_DIR/.db_credentials" >/dev/null 2>&1; then
      log_error "Developer CAN read DB credentials — permissions incorrect!"
    else
      log_ok "Developer CANNOT read DB credentials"
    fi
  fi
  exit 0
fi

if [[ "$ACTION" == "stop" ]]; then
  echo -e "\n${CYAN}${BOLD}Stopping Sentinel...${NC}"
  sentinel_stop
  exit 0
fi

if [[ "$ACTION" == "uninstall" ]]; then
  echo -e "\n${CYAN}${BOLD}Uninstalling Sentinel...${NC}"
  sentinel_stop
  rm -f "$INSTALL_DIR/enforcer-daemon" "$INSTALL_DIR/enforcer-hook" "$INSTALL_DIR/enforcer-client"
  rm -f "$PLIST_FILE"
  rm -f "$CLIENT_PLIST_FILE"
  rm -f "$MANAGED_HOOKS_FILE"
  rm -rf "$CONFIG_DIR" "$LOG_DIR"
  log_ok "Sentinel uninstalled. Developer machine is ungoverned."
  exit 0
fi

if [[ "$ACTION" == "restart" ]]; then
  echo -e "\n${CYAN}${BOLD}Restarting Sentinel...${NC}"
  sentinel_stop
  ACTION="deploy"
fi

# ── Deploy ──────────────────────────────────────────────────────────────────

banner "Sentinel Server Deploy"
echo ""
echo "  Developer user:  ${DEV_USER:-unknown}"
echo "  Management Hub:  ${CENTRAL_URL:-not set}"

# Step 1: Check binaries
log_step "1/8" "Installing binaries"
BINARIES=("enforcer-daemon" "enforcer-hook" "enforcer-client")
ALL_INSTALLED=true
for bin in "${BINARIES[@]}"; do
  SRC_BIN="$PROJECT_ROOT/go/bin/$bin"
  DST_BIN="$INSTALL_DIR/$bin"
  if [[ ! -x "$SRC_BIN" ]]; then
    log_error "$bin not found in $PROJECT_ROOT/go/bin. Build first: cd go && make build"
    ALL_INSTALLED=false
    continue
  fi

  if [[ -x "$DST_BIN" ]]; then
    SRC_HASH="$(shasum -a 256 "$SRC_BIN" | awk '{print $1}')"
    DST_HASH="$(shasum -a 256 "$DST_BIN" | awk '{print $1}')"
    if [[ "$SRC_HASH" == "$DST_HASH" ]]; then
      log_skip "$DST_BIN"
    else
      cp "$SRC_BIN" "$DST_BIN"
      chmod 755 "$DST_BIN"
      chown root:wheel "$DST_BIN" 2>/dev/null || chown root:root "$DST_BIN"
      log_ok "Updated $DST_BIN (hash changed)"
    fi
  else
    cp "$SRC_BIN" "$DST_BIN"
    chmod 755 "$DST_BIN"
    chown root:wheel "$DST_BIN" 2>/dev/null || chown root:root "$DST_BIN"
    log_ok "Installed $DST_BIN (root-owned)"
  fi
done
[[ "$ALL_INSTALLED" == "false" ]] && exit 1

# Step 2: Directories
log_step "2/8" "Creating directories"
for dir in "$CONFIG_DIR" "$CERT_DIR" "$LOG_DIR"; do
  if [[ -d "$dir" ]]; then
    log_skip "$dir"
  else
    mkdir -p "$dir"
    chown root:wheel "$dir" 2>/dev/null || chown root:root "$dir"
    log_ok "Created $dir"
  fi
done
# CONFIG_DIR must be world-traversable (755) so the hook handler (running as
# the developer user) can read the operator token inside it.  Sensitive files
# (certs, admin token) are individually chmod 600.
# CERT_DIR and LOG_DIR stay root-only (700).
chmod 755 "$CONFIG_DIR"
chmod 700 "$CERT_DIR" "$LOG_DIR"

AUTH_KEY_FILE="$CONFIG_DIR/.auth_enc_key"
if [[ -f "$AUTH_KEY_FILE" ]]; then
  chmod 600 "$AUTH_KEY_FILE"
  chown root:wheel "$AUTH_KEY_FILE" 2>/dev/null || chown root:root "$AUTH_KEY_FILE"
  log_skip "Auth encryption key exists ($AUTH_KEY_FILE)"
else
  openssl rand -base64 32 | tr -d '\n' > "$AUTH_KEY_FILE"
  chmod 600 "$AUTH_KEY_FILE"
  chown root:wheel "$AUTH_KEY_FILE" 2>/dev/null || chown root:root "$AUTH_KEY_FILE"
  log_ok "Generated AES-256 auth encryption key at $AUTH_KEY_FILE"
fi

# Step 3: Database
log_step "3/8" "PostgreSQL setup (developer locked out)"
if [[ -f "$CONFIG_DIR/.db_credentials" ]]; then
  log_skip "Database credentials exist"
else
  if [[ -x "$SCRIPT_DIR/setup-database.sh" ]]; then
    "$SCRIPT_DIR/setup-database.sh"
  else
    log_warn "setup-database.sh not found — set up PostgreSQL manually"
  fi
fi

# Repair PostgreSQL grants every deploy.  On macOS Homebrew the developer user
# owns the database and tables.  Previous deploys may have silently failed to
# apply the init.sql grants (output was suppressed).  Running the grants again
# is idempotent and ensures the enforcer service user can INSERT + SELECT.
if [[ -f "$CONFIG_DIR/.db_credentials" ]] && command -v psql >/dev/null 2>&1; then
  DEV_USER="${SUDO_USER:-$(logname 2>/dev/null || echo '')}"
  PG_USER="${DEV_USER:-postgres}"
  REPAIR_SQL="
    CREATE TABLE IF NOT EXISTS auth_secrets (
      role TEXT PRIMARY KEY CHECK (role IN ('admin', 'reviewer', 'operator')),
      nonce_b64 TEXT NOT NULL,
      ciphertext_b64 TEXT NOT NULL,
      key_version INTEGER NOT NULL DEFAULT 1,
      updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );
    GRANT INSERT ON audit_events TO enforcer;
    GRANT SELECT ON audit_events TO enforcer;
    GRANT INSERT ON hub_policy_revisions TO enforcer;
    GRANT SELECT ON hub_policy_revisions TO enforcer;
    GRANT INSERT ON hub_client_snapshots TO enforcer;
    GRANT SELECT ON hub_client_snapshots TO enforcer;
    GRANT INSERT ON hub_enforcement_states TO enforcer;
    GRANT SELECT ON hub_enforcement_states TO enforcer;
    GRANT INSERT, UPDATE ON auth_secrets TO enforcer;
    GRANT SELECT ON auth_secrets TO enforcer;
    GRANT USAGE, SELECT ON SEQUENCE hub_policy_revisions_id_seq TO enforcer;
    GRANT USAGE, SELECT ON SEQUENCE hub_client_snapshots_id_seq TO enforcer;
    GRANT USAGE, SELECT ON SEQUENCE hub_enforcement_states_id_seq TO enforcer;
  "
  if sudo -u "$PG_USER" psql -d enforcer -c "$REPAIR_SQL" >/dev/null 2>&1; then
    log_ok "PostgreSQL grants verified (INSERT + SELECT for enforcer)"
  else
    log_warn "Could not verify PostgreSQL grants — audit storage may fail"
  fi
fi

# Step 4: Hub URL
log_step "4/8" "Management Hub configuration"
ENV_FILE="$CONFIG_DIR/sentinel.env"
if [[ -n "$CENTRAL_URL" ]]; then
  echo "AA_CENTRAL_URL=$CENTRAL_URL" > "$ENV_FILE"
  chmod 600 "$ENV_FILE"
  log_ok "Hub URL: $CENTRAL_URL"
elif [[ -f "$ENV_FILE" ]]; then
  log_skip "Hub URL already configured ($(grep AA_CENTRAL_URL "$ENV_FILE" | head -1))"
else
  log_warn "AA_CENTRAL_URL not set. Sentinel will run without Hub sync."
  log_warn "Set it: sudo AA_CENTRAL_URL=https://hub:9200 ./scripts/deploy_sentinel.sh"
fi

# Optional auth seeding for Sentinel login.
if [[ "$SEED_AUTH" == "true" ]]; then
  if [[ -z "$SEED_DEV_PASSWORD" ]]; then
    log_warn "Seed requested without --seed-dev-password; generating random token"
    SEED_DEV_PASSWORD="$(openssl rand -base64 24 | tr -d '\n')"
  fi
  echo -n "$SEED_DEV_PASSWORD" > "$CONFIG_DIR/.operator_token"
  echo -n "$SEED_DEV_USER" > "$CONFIG_DIR/.seed_developer_username"
  # .operator_token must be world-readable so the hook handler (which runs as
  # the developer user, not root) can authenticate to the daemon.  This is an
  # operator-level token with limited privileges (no admin access).
  chmod 644 "$CONFIG_DIR/.operator_token"
  chmod 600 "$CONFIG_DIR/.seed_developer_username"
  chown root:wheel "$CONFIG_DIR/.operator_token" "$CONFIG_DIR/.seed_developer_username" 2>/dev/null || chown root:root "$CONFIG_DIR/.operator_token" "$CONFIG_DIR/.seed_developer_username"
  log_ok "Seeded Sentinel developer credentials in $CONFIG_DIR (.operator_token + .seed_developer_username)"
  seed_tokens_in_db "" "" "$SEED_DEV_PASSWORD"
fi

# Always ensure an operator token exists — the hook handler needs it to
# authenticate to the daemon.  Without it, hooks get 401 and fail-closed.
if [[ ! -f "$CONFIG_DIR/.operator_token" ]]; then
  HOOK_TOKEN="$(openssl rand -base64 24 | tr -d '\n')"
  echo -n "$HOOK_TOKEN" > "$CONFIG_DIR/.operator_token"
  chown root:wheel "$CONFIG_DIR/.operator_token" 2>/dev/null || chown root:root "$CONFIG_DIR/.operator_token"
  log_ok "Generated operator token for hook handler ($CONFIG_DIR/.operator_token)"
fi
# The operator token MUST be world-readable (0644) so the hook handler
# (running as the developer user) can authenticate to the daemon.
# Enforce this on every deploy in case a previous run set 0600.
chmod 644 "$CONFIG_DIR/.operator_token"

# Ensure a baseline policy exists and is valid so daemon does not default-deny everything.
if [[ ! -f "$PROJECT_ROOT/policies/default.yaml" ]]; then
  log_error "No policy bundle found at $PROJECT_ROOT/policies/default.yaml"
  log_error "Cannot start Sentinel without baseline policy."
  exit 1
fi

POLICY_DST="$CONFIG_DIR/default.yaml"
POLICY_REPAIRED=false
if [[ -f "$POLICY_DST" ]]; then
  if grep -q "policy_id:" "$POLICY_DST" 2>/dev/null; then
    if cmp -s "$PROJECT_ROOT/policies/default.yaml" "$POLICY_DST"; then
      log_skip "Policy exists and is up to date at $POLICY_DST"
    else
      cp "$PROJECT_ROOT/policies/default.yaml" "$POLICY_DST"
      POLICY_REPAIRED=true
      log_ok "Updated baseline policy at $POLICY_DST (baseline changed)"
    fi
  else
    cp "$PROJECT_ROOT/policies/default.yaml" "$POLICY_DST"
    POLICY_REPAIRED=true
    log_ok "Repaired invalid policy file at $POLICY_DST (restored baseline)"
  fi
else
  cp "$PROJECT_ROOT/policies/default.yaml" "$POLICY_DST"
  POLICY_REPAIRED=true
  log_ok "Copied baseline policy bundle to $CONFIG_DIR"
fi
chmod 644 "$POLICY_DST"
chown root:wheel "$POLICY_DST" 2>/dev/null || chown root:root "$POLICY_DST"

if [[ -f "$PROJECT_ROOT/policies/network-allowlist.yaml" ]]; then
  cp "$PROJECT_ROOT/policies/network-allowlist.yaml" "$CONFIG_DIR/network-allowlist.yaml"
  chmod 644 "$CONFIG_DIR/network-allowlist.yaml"
  chown root:wheel "$CONFIG_DIR/network-allowlist.yaml" 2>/dev/null || chown root:root "$CONFIG_DIR/network-allowlist.yaml"
fi

# Step 5: Client certificates
log_step "5/8" "Client mTLS certificates"
if [[ -f "$CERT_DIR/client.crt" && -f "$CERT_DIR/ca.crt" ]]; then
  log_skip "Client certificates exist in $CERT_DIR"
else
  # Check if Hub certs exist in project (single-machine mode)
  if [[ -f "$PROJECT_ROOT/certs/client.crt" ]]; then
    cp "$PROJECT_ROOT/certs/client.crt" "$CERT_DIR/client.crt"
    cp "$PROJECT_ROOT/certs/client.key" "$CERT_DIR/client.key"
    cp "$PROJECT_ROOT/certs/ca.crt" "$CERT_DIR/ca.crt"
    chmod 600 "$CERT_DIR/client.key"
    log_ok "Copied Sentinel certificates from project"
  else
    log_warn "No Sentinel certificates found. Copy from Hub: scp hub:/etc/enforcer/certs/client.* $CERT_DIR/"
  fi
fi

# Step 6: Managed hooks
log_step "6/8" "Managed Claude Code hooks (developer cannot remove)"
if [[ -f "$MANAGED_HOOKS_FILE" ]]; then
  log_skip "Managed hooks exist at $MANAGED_HOOKS_FILE"
else
  mkdir -p "$MANAGED_HOOKS_DIR"
  cat > "$MANAGED_HOOKS_FILE" << 'HOOKS_EOF'
{
  "hooks": {
    "PreToolUse": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/enforcer-hook pre_tool_call",
            "timeout": 300,
            "statusMessage": "Enforcer: Evaluating policy..."
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/enforcer-hook post_tool_call"
          }
        ]
      }
    ]
  },
  "allowManagedHooksOnly": true
}
HOOKS_EOF
  chmod 644 "$MANAGED_HOOKS_FILE"
  chown root:wheel "$MANAGED_HOOKS_FILE" 2>/dev/null || chown root:root "$MANAGED_HOOKS_FILE"
  log_ok "Managed hooks installed — developer cannot modify or remove"
fi

# Step 7: LaunchDaemon
log_step "7/8" "LaunchDaemon (auto-start, KeepAlive)"
# Always regenerate plist from current config so AA_CENTRAL_URL/certs/db env cannot go stale.
DB_URL=""
[[ -f "$CONFIG_DIR/.db_credentials" ]] && DB_URL=$(cat "$CONFIG_DIR/.db_credentials")
HUB_URL=""
[[ -f "$ENV_FILE" ]] && HUB_URL=$(grep AA_CENTRAL_URL "$ENV_FILE" 2>/dev/null | cut -d= -f2-)

cat > "$PLIST_FILE" << PLIST_EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.enforcer.sentinel</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/enforcer-daemon</string>
    </array>
    <key>KeepAlive</key>
    <true/>
    <key>RunAtLoad</key>
    <true/>
    <key>StandardOutPath</key>
    <string>$LOG_FILE</string>
    <key>StandardErrorPath</key>
    <string>$LOG_FILE</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>DAEMON_PORT</key>
        <string>$DAEMON_PORT</string>
        <key>AA_STRICT_MODE</key>
        <string>$STRICT_MODE</string>
        <key>DATABASE_URL</key>
        <string>$DB_URL</string>
        <key>AA_CENTRAL_URL</key>
        <string>$HUB_URL</string>
        <key>CERT_DIR</key>
        <string>$CERT_DIR</string>
        <key>AA_CONFIG_DIR</key>
        <string>$CONFIG_DIR</string>
        <key>AA_AUTH_ENC_KEY_FILE</key>
        <string>$AUTH_KEY_FILE</string>
        <key>AA_POLICY_DIR</key>
        <string>$CONFIG_DIR</string>
        <key>AA_GOVERNED_USER</key>
        <string>$DEV_USER</string>
    </dict>
</dict>
</plist>
PLIST_EOF
chmod 644 "$PLIST_FILE"
log_ok "Sentinel daemon LaunchDaemon plist updated at $PLIST_FILE"

cat > "$CLIENT_PLIST_FILE" << CLIENT_PLIST_EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.enforcer.sentinel-client</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/enforcer-client</string>
    </array>
    <key>KeepAlive</key>
    <true/>
    <key>RunAtLoad</key>
    <true/>
    <key>StandardOutPath</key>
    <string>$CLIENT_LOG_FILE</string>
    <key>StandardErrorPath</key>
    <string>$CLIENT_LOG_FILE</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>AA_CENTRAL_URL</key>
        <string>$HUB_URL</string>
        <key>CERT_DIR</key>
        <string>$CERT_DIR</string>
        <key>AA_CONFIG_DIR</key>
        <string>$CONFIG_DIR</string>
        <key>DAEMON_PORT</key>
        <string>$DAEMON_PORT</string>
        <key>AA_SYNC_INTERVAL</key>
        <string>5s</string>
    </dict>
</dict>
</plist>
CLIENT_PLIST_EOF
chmod 644 "$CLIENT_PLIST_FILE"
log_ok "Sentinel client LaunchDaemon plist updated at $CLIENT_PLIST_FILE"

# Ensure no stale local daemon process blocks LaunchDaemon bind.
lsof -ti "tcp:$DAEMON_PORT" 2>/dev/null | xargs kill -TERM 2>/dev/null || true
sleep 1

# Reload to apply updated environment variables.
launchctl unload "$PLIST_FILE" 2>/dev/null || true
if launchctl load "$PLIST_FILE" 2>/dev/null; then
  log_ok "Sentinel daemon LaunchDaemon loaded (KeepAlive — developer cannot kill)"
else
  log_warn "Sentinel daemon LaunchDaemon load returned non-zero; trying kickstart fallback"
  if launchctl kickstart -k "system/com.enforcer.sentinel" 2>/dev/null; then
    log_ok "Sentinel daemon LaunchDaemon kickstart succeeded"
  else
    log_error "Sentinel daemon service failed to start via launchctl load/kickstart"
    exit 1
  fi
fi
launchctl unload "$CLIENT_PLIST_FILE" 2>/dev/null || true
if launchctl load "$CLIENT_PLIST_FILE" 2>/dev/null; then
  log_ok "Sentinel client LaunchDaemon loaded (policy sync + registration active)"
else
  log_warn "Sentinel client LaunchDaemon load returned non-zero; trying kickstart fallback"
  if launchctl kickstart -k "system/com.enforcer.sentinel-client" 2>/dev/null; then
    log_ok "Sentinel client LaunchDaemon kickstart succeeded"
  else
    log_error "Sentinel client service failed to start via launchctl load/kickstart"
    exit 1
  fi
fi

# Step 8: Health check
log_step "8/8" "Health check"
HEALTH_OK=false
for _ in $(seq 1 30); do
  if curl -sf "http://localhost:$DAEMON_PORT/v1/health" >/dev/null 2>&1; then
    HEALTH_OK=true
    break
  fi
  sleep 1
done
if [[ "$HEALTH_OK" == "true" ]]; then
  log_ok "Sentinel is healthy"
  curl -s "http://localhost:$DAEMON_PORT/v1/health" | python3 -m json.tool 2>/dev/null || true
else
  log_error "Sentinel health check failed after 30s. Check: curl http://localhost:$DAEMON_PORT/v1/health"
  exit 1
fi

# Summary
SENTINEL_DEPLOY_SUCCEEDED=true
banner "Sentinel Deploy Complete"
SENTINEL_CONSOLE_URL="$(resolve_sentinel_console_url || true)"
echo ""
echo "  Sentinel Server API:        http://localhost:$DAEMON_PORT"
if [[ -n "$SENTINEL_CONSOLE_URL" ]]; then
  echo "  Sentinel Developer Console:  $SENTINEL_CONSOLE_URL (developer-personal view)"
else
  echo "  Sentinel Developer Console:  not reachable on expected ports (${CONSOLE_PORT}, ${DAEMON_PORT})"
fi
echo "  Management Hub:    ${CENTRAL_URL:-not configured}"
echo "  Logs:              $LOG_FILE"
echo "  Managed hooks:     $MANAGED_HOOKS_FILE"
echo "  LaunchDaemon:      $PLIST_FILE (Sentinel daemon, KeepAlive, runs as root)"
echo "  Client service:    $CLIENT_PLIST_FILE (registration/policy sync, KeepAlive)"
echo "  Developer user:    ${DEV_USER:-unknown} (NO access to config, creds, or policy)"
if [[ "$SEED_AUTH" == "true" ]]; then
  echo ""
  echo "  Sentinel login seed:"
  echo "    Username:        $SEED_DEV_USER"
  echo "    Access token:    $SEED_DEV_PASSWORD"
fi
echo ""
echo "  The developer opens VS Code → Claude Code reads managed hooks → governance is active."
echo "  Re-run this script anytime — it is idempotent."
echo ""
