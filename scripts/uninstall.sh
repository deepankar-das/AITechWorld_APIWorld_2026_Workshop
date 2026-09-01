#!/bin/bash
# Enforcer — Uninstall
# Author: Deepankar Das
#
# Removes all Enforcer components: services, binaries, hooks, configs, logs.
# PostgreSQL audit database is preserved by default (use --drop-database to remove).
#
# Usage:
#   sudo ./scripts/uninstall.sh                # Full uninstall (preserves audit DB)
#   sudo ./scripts/uninstall.sh --drop-database # Full uninstall + drop PostgreSQL DB

set -euo pipefail

DROP_DATABASE=false

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

log_ok()   { echo -e "${GREEN}[OK]${NC}    $1"; }
log_skip() { echo -e "${YELLOW}[SKIP]${NC}  $1"; }
log_step() { echo -e "${BLUE}[STEP]${NC}  $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC}  $1"; }

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

while [[ $# -gt 0 ]]; do
  case "$1" in
    --drop-database) DROP_DATABASE=true; shift ;;
    --help|-h)
      echo "Usage: sudo $0 [--drop-database]"
      echo ""
      echo "Options:"
      echo "  --drop-database  Also drop the PostgreSQL enforcer database (audit evidence)"
      exit 0
      ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

if [[ $EUID -ne 0 ]]; then
  echo -e "${RED}[ERROR]${NC} This script must be run with sudo."
  echo "  Usage: sudo $0"
  exit 1
fi

banner "Uninstall"

# ── 1. Stop services (LaunchDaemons) ────────────────────────────────────────

log_step "Stopping services..."

PLISTS=(
  "/Library/LaunchDaemons/com.enforcer.daemon.plist"
  "/Library/LaunchDaemons/com.enforcer.sentinel.plist"
  "/Library/LaunchDaemons/com.enforcer.sentinel-client.plist"
  "/Library/LaunchDaemons/com.enforcer.central.plist"
  "/Library/LaunchDaemons/com.enforcer.client.plist"
)

for plist in "${PLISTS[@]}"; do
  if [[ -f "$plist" ]]; then
    launchctl bootout system "$plist" 2>/dev/null || launchctl unload "$plist" 2>/dev/null || true
    rm -f "$plist"
    log_ok "Removed $(basename "$plist")"
  fi
done

# Kill any remaining enforcer processes
for proc in enforcer-daemon enforcer-hook enforcer-client enforcer-central; do
  if pkill -f "/usr/local/bin/$proc" 2>/dev/null; then
    log_ok "Stopped $proc"
  fi
done

# Kill by known ports
for port in 9100 9101 9200 9201; do
  pid=$(lsof -ti "tcp:$port" 2>/dev/null || true)
  if [[ -n "$pid" ]]; then
    kill -TERM $pid 2>/dev/null || true
    log_ok "Stopped process on port $port (pid $pid)"
  fi
done

# Clean up PID files
rm -f /var/run/enforcer-*.pid /tmp/enforcer-*.pid 2>/dev/null || true

# ── 2. Remove binaries ─────────────────────────────────────────────────────

log_step "Removing binaries..."

BINARIES=(
  "/usr/local/bin/enforcer-daemon"
  "/usr/local/bin/enforcer-hook"
  "/usr/local/bin/enforcer-client"
  "/usr/local/bin/enforcer-central"
  "/usr/local/bin/enforcer-authseed"
  "/usr/local/bin/enforcer-status"
)

for bin in "${BINARIES[@]}"; do
  if [[ -f "$bin" ]]; then
    rm -f "$bin"
    log_ok "Removed $bin"
  fi
done

# ── 3. Remove managed hooks ────────────────────────────────────────────────

log_step "Removing managed hooks..."

MANAGED_HOOKS="/Library/Application Support/ClaudeCode/managed-settings.json"
if [[ -f "$MANAGED_HOOKS" ]]; then
  rm -f "$MANAGED_HOOKS"
  log_ok "Removed $MANAGED_HOOKS"
else
  log_skip "No managed hooks found"
fi

# ── 4. Remove project-level hooks ──────────────────────────────────────────

log_step "Removing user-level Claude Code hooks..."

# Resolve the real user (the one who ran sudo)
REAL_USER="${SUDO_USER:-$(whoami)}"
REAL_HOME=$(eval echo "~$REAL_USER")

CLAUDE_SETTINGS="$REAL_HOME/.claude/settings.json"
if [[ -f "$CLAUDE_SETTINGS" ]]; then
  if command -v node >/dev/null 2>&1; then
    node -e "
      const fs = require('fs');
      const f = '$CLAUDE_SETTINGS';
      try {
        const s = JSON.parse(fs.readFileSync(f, 'utf8'));
        let changed = false;
        for (const key of ['preToolUse','postToolUse','PreToolUse','PostToolUse']) {
          if (s.hooks && Array.isArray(s.hooks[key])) {
            const before = s.hooks[key].length;
            s.hooks[key] = s.hooks[key].filter(h => {
              const cmd = h.command || (h.hooks && h.hooks[0] && h.hooks[0].command) || '';
              return !cmd.includes('enforcer');
            });
            if (s.hooks[key].length === 0) delete s.hooks[key];
            if (s.hooks[key]?.length !== before) changed = true;
          }
        }
        if (s.hooks && Object.keys(s.hooks).length === 0) delete s.hooks;
        if (changed) {
          fs.writeFileSync(f, JSON.stringify(s, null, 2) + '\n');
          console.log('removed');
        } else {
          console.log('none');
        }
      } catch(e) { console.log('none'); }
    " | while read -r result; do
      if [[ "$result" == "removed" ]]; then
        log_ok "Removed enforcer hooks from $CLAUDE_SETTINGS"
      else
        log_skip "No enforcer hooks in $CLAUDE_SETTINGS"
      fi
    done
  else
    log_warn "Node.js not found — cannot clean $CLAUDE_SETTINGS (remove enforcer hook entries manually)"
  fi
else
  log_skip "No Claude Code settings found at $CLAUDE_SETTINGS"
fi

# ── 5. Remove config, certs, tokens ────────────────────────────────────────

log_step "Removing configuration..."

CONFIG_DIRS=(
  "/etc/enforcer"
  "/opt/enforcer"
)

for dir in "${CONFIG_DIRS[@]}"; do
  if [[ -d "$dir" ]]; then
    rm -rf "$dir"
    log_ok "Removed $dir"
  fi
done

# ── 6. Remove logs ─────────────────────────────────────────────────────────

log_step "Removing logs..."

LOG_DIRS=(
  "/var/log/enforcer"
  "/var/lib/enforcer"
)

for dir in "${LOG_DIRS[@]}"; do
  if [[ -d "$dir" ]]; then
    rm -rf "$dir"
    log_ok "Removed $dir"
  fi
done

# Clean up legacy log locations
for f in /tmp/enforcer-daemon.log /tmp/enforcer-console.log; do
  if [[ -f "$f" ]]; then
    rm -f "$f"
    log_ok "Removed $f"
  fi
done

# ── 7. PostgreSQL database ─────────────────────────────────────────────────

log_step "PostgreSQL audit database..."

if [[ "$DROP_DATABASE" == "true" ]]; then
  if command -v dropdb >/dev/null 2>&1; then
    # Under sudo, whoami is root — but macOS Homebrew PG has no root role.
    # Try the real user first, then postgres.
    PG_USER="${SUDO_USER:-$(whoami)}"
    DB_EXISTS=false
    if psql -U "$PG_USER" -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='enforcer'" 2>/dev/null | grep -q 1; then
      DB_EXISTS=true
    elif sudo -u postgres psql -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='enforcer'" 2>/dev/null | grep -q 1; then
      DB_EXISTS=true
      PG_USER="postgres"
    fi

    if [[ "$DB_EXISTS" == "true" ]]; then
      # Terminate active connections before dropping
      psql -U "$PG_USER" -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='enforcer' AND pid <> pg_backend_pid();" >/dev/null 2>&1 || true
      if dropdb -U "$PG_USER" enforcer 2>/dev/null; then
        log_ok "Dropped database enforcer"
      elif sudo -u postgres dropdb enforcer 2>/dev/null; then
        log_ok "Dropped database enforcer"
      else
        log_warn "Could not drop database enforcer — drop manually: sudo -u postgres dropdb enforcer"
      fi
    else
      log_skip "Database enforcer does not exist"
    fi
    # Remove the dedicated database user.
    # Must run AFTER dropping the database — the role has grants in it.
    # Revoke residual privileges first (e.g. on schema public) to avoid
    # "role cannot be dropped because some objects depend on it".
    psql -U "$PG_USER" -d postgres -c "REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM enforcer; REVOKE ALL ON SCHEMA public FROM enforcer;" 2>/dev/null || true
    if psql -U "$PG_USER" -d postgres -tAc "SELECT 1 FROM pg_roles WHERE rolname='enforcer'" 2>/dev/null | grep -q 1; then
      if psql -U "$PG_USER" -d postgres -c "DROP ROLE enforcer" 2>/dev/null; then
        log_ok "Dropped database role enforcer"
      elif sudo -u postgres psql -c "DROP ROLE enforcer" 2>/dev/null; then
        log_ok "Dropped database role enforcer"
      else
        log_warn "Could not drop role enforcer — drop manually: psql -d postgres -c 'DROP ROLE enforcer'"
      fi
    fi
  else
    log_warn "psql/dropdb not found — cannot drop database"
  fi
else
  log_skip "Database enforcer preserved (audit evidence). Use --drop-database to remove."
fi

# ── Summary ─────────────────────────────────────────────────────────────────

banner "Uninstall Complete"
echo "  Removed:"
echo "    - LaunchDaemon services"
echo "    - Binaries from /usr/local/bin/"
echo "    - Managed hooks (/Library/Application Support/ClaudeCode/)"
echo "    - User-level Claude Code hooks"
echo "    - Configuration (/etc/enforcer/, /opt/enforcer/)"
echo "    - Logs (/var/log/enforcer/, /var/lib/enforcer/)"
if [[ "$DROP_DATABASE" == "true" ]]; then
  echo "    - PostgreSQL database enforcer"
else
  echo ""
  echo "  Preserved:"
  echo "    - PostgreSQL database enforcer (contains audit evidence)"
  echo "      To remove: sudo $0 --drop-database"
fi
echo ""
echo "  Claude Code is now ungoverned. Restart VS Code to pick up hook changes."
echo ""
