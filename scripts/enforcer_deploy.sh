#!/bin/bash
# Enforcer — Unified Deployment Script
#
# Deploys Enforcer Hub Server, Sentinel Server, or both.
# Handles certificate generation, service installation, and configuration.
#
# Usage:
#   sudo ./scripts/enforcer_deploy.sh hub              # Deploy Hub Server
#   sudo ./scripts/enforcer_deploy.sh sentinel         # Deploy Sentinel for current user
#   sudo ./scripts/enforcer_deploy.sh sentinel --all-users  # Deploy Sentinel for all users
#   sudo ./scripts/enforcer_deploy.sh sentinel --user deepankardas  # Deploy for specific user
#   sudo ./scripts/enforcer_deploy.sh full              # Deploy Hub + Sentinel (single machine)
#   sudo ./scripts/enforcer_deploy.sh status            # Check deployment status
#   sudo ./scripts/enforcer_deploy.sh uninstall         # Remove everything
#
# Hub Server is installed to /opt/enforcer/
# Sentinel Server is installed as a LaunchDaemon per-user or system-wide

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Installation paths
INSTALL_DIR="/opt/enforcer"
CERT_DIR="/opt/enforcer/certs"
CONFIG_DIR="/opt/enforcer/config"
DATA_DIR="/opt/enforcer/data"
LOG_DIR="/var/log/enforcer"
BIN_DIR="/usr/local/bin"

# Service names
CENTRAL_SERVICE="com.enforcer.central"
CLIENT_SERVICE="com.enforcer.client"
CENTRAL_PLIST="/Library/LaunchDaemons/${CENTRAL_SERVICE}.plist"
CLIENT_PLIST="/Library/LaunchDaemons/${CLIENT_SERVICE}.plist"

# Ports
CENTRAL_PORT=9200
ADMIN_PORT=9201
DAEMON_PORT=9100
PROXY_PORT=9101

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

MODE=""
TARGET_USER=""
ALL_USERS=false
CENTRAL_URL=""

# ── Parse Arguments ──────────────────────────────────────────────────────────

while [[ $# -gt 0 ]]; do
  case "$1" in
    central|hub)    MODE="central"; shift ;;
    client|sentinel)     MODE="client"; shift ;;
    full)       MODE="full"; shift ;;
    status)     MODE="status"; shift ;;
    uninstall)  MODE="uninstall"; shift ;;
    --all-users) ALL_USERS=true; shift ;;
    --user)     TARGET_USER="$2"; shift 2 ;;
    --central-url|--hub-url) CENTRAL_URL="$2"; shift 2 ;;
    --help|-h)
      echo "Enforcer — Unified Deployment"
      echo ""
      echo "Usage: sudo $0 <mode> [options]"
      echo ""
      echo "Modes:"
      echo "  hub                  Deploy Hub Server to /opt/enforcer/"
      echo "  sentinel             Deploy Sentinel Server for governed users"
      echo "  full                 Deploy both (single-machine setup)"
      echo "  status               Check deployment status"
      echo "  uninstall            Remove all Enforcer components"
      echo ""
      echo "Legacy aliases:"
      echo "  central -> hub"
      echo "  client  -> sentinel"
      echo ""
      echo "Sentinel options:"
      echo "  --all-users          Install Sentinel governance for all users on this machine"
      echo "  --user USERNAME      Install Sentinel governance for a specific user"
      echo "  --hub-url URL        Hub Server URL (default: https://localhost:9200)"
      echo ""
      echo "Examples:"
      echo "  sudo $0 hub                               # Hub Server on this machine"
      echo "  sudo $0 sentinel --hub-url https://security.corp.com:9200"
      echo "  sudo $0 sentinel --user deepankardas      # Govern specific user"
      echo "  sudo $0 sentinel --all-users              # Govern all users"
      echo "  sudo $0 full                              # Everything on one machine"
      echo "  sudo $0 status                            # Check what's running"
      echo "  sudo $0 uninstall                         # Remove everything"
      exit 0
      ;;
    *) echo "Unknown option: $1. Use --help for usage."; exit 1 ;;
  esac
done

if [[ -z "$MODE" ]]; then
  echo "Usage: sudo $0 <hub|sentinel|full|status|uninstall> [options]"
  echo "Run with --help for details."
  exit 1
fi

# ── Root Check (MANDATORY) ────────────────────────────────────────────────────
# Enforcer installation MUST be done by a superuser/root.
# This is a security requirement, not a convenience.
  # If a regular user could install the Sentinel, they could also
# tamper with the installation — defeating the entire purpose.

if [[ $EUID -ne 0 ]]; then
  if [[ "$MODE" == "status" ]]; then
    # Status check is allowed for any user (read-only)
    :
  else
    echo ""
    echo -e "${RED}${BOLD}[ERROR] Root/superuser access required.${NC}"
    echo ""
    echo "  Enforcer must be installed by an administrator (root/sudo)."
    echo "  This is a security requirement — the governed developer must not"
    echo "  be able to install, modify, or uninstall the firewall."
    echo ""
    echo "  Run: ${BOLD}sudo $0 $MODE${NC}"
    echo ""
    exit 1
  fi
fi

# ── Banner ───────────────────────────────────────────────────────────────────

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

# ── Status ───────────────────────────────────────────────────────────────────

if [[ "$MODE" == "status" ]]; then
  banner "Deployment Status"

  echo "  Installation:"
  [[ -d "$INSTALL_DIR" ]] && echo -e "    ${GREEN}[OK]${NC}    Installed at $INSTALL_DIR" || echo -e "    ${YELLOW}[--]${NC}    Not installed"
  [[ -d "$CERT_DIR" ]] && echo -e "    ${GREEN}[OK]${NC}    Certificates at $CERT_DIR" || echo -e "    ${YELLOW}[--]${NC}    No certificates"

  echo ""
  echo "  Services:"
  if launchctl list 2>/dev/null | grep -q "$CENTRAL_SERVICE"; then
    echo -e "    ${GREEN}[ON]${NC}    Hub Server ($CENTRAL_SERVICE)"
  else
    echo -e "    ${YELLOW}[OFF]${NC}   Hub Server"
  fi

  if launchctl list 2>/dev/null | grep -q "$CLIENT_SERVICE"; then
    echo -e "    ${GREEN}[ON]${NC}    Sentinel Server ($CLIENT_SERVICE)"
  else
    echo -e "    ${YELLOW}[OFF]${NC}   Sentinel Server"
  fi

  echo ""
  echo "  Connectivity:"
  curl -sf "http://127.0.0.1:${DAEMON_PORT}/v1/health" >/dev/null 2>&1 && echo -e "    ${GREEN}[OK]${NC}    Local Sentinel Server on port $DAEMON_PORT" || echo -e "    ${YELLOW}[--]${NC}    Local Sentinel Server not responding"
  curl -sf "http://127.0.0.1:${ADMIN_PORT}/api/v1/health" >/dev/null 2>&1 && echo -e "    ${GREEN}[OK]${NC}    Hub Admin API on port $ADMIN_PORT" || echo -e "    ${YELLOW}[--]${NC}    Hub Admin API not responding"

  echo ""
  echo "  Managed hooks:"
  MANAGED_SETTINGS="/Library/Application Support/ClaudeCode/managed-settings.json"
  [[ -f "$MANAGED_SETTINGS" ]] && echo -e "    ${GREEN}[ON]${NC}    Managed hooks active (developer cannot remove)" || echo -e "    ${YELLOW}[OFF]${NC}   No managed hooks"

  echo ""
  banner "Deployment Status Complete"
  exit 0
fi

# ── Uninstall ────────────────────────────────────────────────────────────────

if [[ "$MODE" == "uninstall" ]]; then
  banner "Uninstalling Enforcer"

  # Stop services
  launchctl bootout system "$CENTRAL_PLIST" 2>/dev/null || true
  launchctl bootout system "$CLIENT_PLIST" 2>/dev/null || true
  echo -e "${GREEN}[OK]${NC} Services stopped"

  # Remove plist files
  rm -f "$CENTRAL_PLIST" "$CLIENT_PLIST"
  echo -e "${GREEN}[OK]${NC} Service plists removed"

  # Remove managed hooks
  rm -f "/Library/Application Support/ClaudeCode/managed-settings.json"
  echo -e "${GREEN}[OK]${NC} Managed hooks removed"

  # Remove bin links
  rm -f "$BIN_DIR/enforcer-central" "$BIN_DIR/enforcer-client" "$BIN_DIR/enforcer-status"
  echo -e "${GREEN}[OK]${NC} Binaries removed from $BIN_DIR"

  # Preserve data and logs (audit evidence)
  echo -e "${YELLOW}[NOTE]${NC} Preserved: $INSTALL_DIR (config, certs)"
  echo -e "${YELLOW}[NOTE]${NC} Preserved: $LOG_DIR (audit logs)"
  echo -e "${YELLOW}[NOTE]${NC} Preserved: PostgreSQL data (audit_events table)"
  echo ""
  echo "  To fully remove all data: sudo rm -rf $INSTALL_DIR $LOG_DIR"
  echo "  To drop the database: dropdb enforcer"
  banner "Uninstall Complete"
  exit 0
fi

# ── Common Setup ─────────────────────────────────────────────────────────────

setup_directories() {
  mkdir -p "$INSTALL_DIR" "$CERT_DIR" "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR"

  # All directories owned by root:wheel
  chown -R root:wheel "$INSTALL_DIR"
  chown root:wheel "$LOG_DIR"

  # Permissions:
  #   root:  rwx on all dirs
  #   group: r-x (can read, can enter, cannot write)
  #   other: r-x on log dir (developer can read logs), --- on everything else
  chmod 700 "$INSTALL_DIR"       # Only root can access the install dir
  chmod 700 "$CERT_DIR"          # Only root can access certs (private keys)
  chmod 755 "$CONFIG_DIR"        # Developer can read config (policies), cannot write
  chmod 700 "$DATA_DIR"          # Only root can access runtime data
  chmod 755 "$LOG_DIR"           # Developer can read logs, cannot write/delete

  echo -e "${GREEN}[OK]${NC} Directories created with root-only write permissions"
}

setup_certs() {
  if [[ -f "$CERT_DIR/ca.pem" && -f "$CERT_DIR/server.pem" && -f "$CERT_DIR/client.pem" ]]; then
    echo -e "${GREEN}[OK]${NC} TLS certificates already exist"
    return
  fi

  echo -e "${YELLOW}[CERTS]${NC} Generating TLS certificates for mTLS..."
  CERT_DIR="$CERT_DIR" "$SCRIPT_DIR/generate-certs.sh" "$CERT_DIR"
  echo -e "${GREEN}[OK]${NC} TLS certificates generated"
}

setup_admin_token() {
  local token_file="$CONFIG_DIR/.admin_token"
  if [[ -f "$token_file" ]]; then
    ADMIN_TOKEN=$(cat "$token_file")
    echo -e "${GREEN}[OK]${NC} Admin token exists"
  else
    ADMIN_TOKEN=$(openssl rand -hex 32)
    echo "$ADMIN_TOKEN" > "$token_file"
    chmod 600 "$token_file"
    echo -e "${GREEN}[OK]${NC} Admin token generated"
    echo -e "${YELLOW}[IMPORTANT]${NC} Admin token (save this): ${CYAN}${ADMIN_TOKEN}${NC}"
  fi
}

copy_application() {
  # Copy application files to /opt/enforcer/app/
  local app_dir="$INSTALL_DIR/app"
  mkdir -p "$app_dir"

  # Detect: are we running from a distribution tarball (compiled JS) or source?
  if [[ -f "$PROJECT_ROOT/version.json" && -d "$PROJECT_ROOT/lib" ]]; then
    # Distribution mode: tarball contains compiled JS in lib/, bin/, config/
    echo -e "${GREEN}[OK]${NC} Installing from distribution package"
    cp -r "$PROJECT_ROOT/lib" "$app_dir/"
    cp -r "$PROJECT_ROOT/bin" "$app_dir/" 2>/dev/null || true
    cp -r "$PROJECT_ROOT/config" "$app_dir/" 2>/dev/null || true
    cp -r "$PROJECT_ROOT/node_modules" "$app_dir/" 2>/dev/null || true
    cp "$PROJECT_ROOT/package.json" "$app_dir/"
    cp "$PROJECT_ROOT/version.json" "$app_dir/"
  else
    # Development mode: source code present, needs tsx to run
    echo -e "${YELLOW}[DEV]${NC} Installing from source (development mode)"
    cp -r "$PROJECT_ROOT/src" "$app_dir/"
    cp -r "$PROJECT_ROOT/types" "$app_dir/"
    cp -r "$PROJECT_ROOT/policies" "$app_dir/"
    cp -r "$PROJECT_ROOT/node_modules" "$app_dir/" 2>/dev/null || true
    cp "$PROJECT_ROOT/package.json" "$app_dir/"
    cp "$PROJECT_ROOT/tsconfig.json" "$app_dir/"
    cp "$PROJECT_ROOT/tsconfig.build.json" "$app_dir/" 2>/dev/null || true
  fi

  # Lock down application directory
  # Developer can read (needed for hook handler execution via node)
  # Developer CANNOT write, delete, or modify any application file
  chown -R root:wheel "$app_dir"
  find "$app_dir" -type d -exec chmod 755 {} \;   # Dirs: root rwx, others r-x
  find "$app_dir" -type f -exec chmod 644 {} \;   # Files: root rw, others r--
  # node_modules binaries need execute
  find "$app_dir/node_modules/.bin" -type f -exec chmod 755 {} \; 2>/dev/null || true

  # Copy policies to protected config location
  # Developer can READ policies (needs to know what rules apply)
  # Developer CANNOT write, delete, or modify policies
  cp -r "$PROJECT_ROOT/policies/"* "$CONFIG_DIR/" 2>/dev/null || true
  chown -R root:wheel "$CONFIG_DIR"
  find "$CONFIG_DIR" -type d -exec chmod 755 {} \;
  find "$CONFIG_DIR" -type f -exec chmod 644 {} \;

  # Admin token: root-only read (developer cannot see it)
  if [[ -f "$CONFIG_DIR/.admin_token" ]]; then
    chmod 600 "$CONFIG_DIR/.admin_token"
  fi

  # Private keys: root-only read
  if [[ -f "$CERT_DIR/server-key.pem" ]]; then chmod 600 "$CERT_DIR/server-key.pem"; fi
  if [[ -f "$CERT_DIR/client-key.pem" ]]; then chmod 600 "$CERT_DIR/client-key.pem"; fi
  if [[ -f "$CERT_DIR/ca-key.pem" ]]; then chmod 600 "$CERT_DIR/ca-key.pem"; fi

  echo -e "${GREEN}[OK]${NC} Application installed to $app_dir"
  echo -e "${GREEN}[OK]${NC} File permissions locked: developer can read, cannot write/delete/modify"
}

setup_database() {
  if ! command -v psql >/dev/null 2>&1; then
    echo -e "${YELLOW}[WARN]${NC} PostgreSQL not found. Audit persistence will use SQLite fallback."
    return
  fi

  if ! pg_isready -q >/dev/null 2>&1; then
    echo -e "${YELLOW}[WARN]${NC} PostgreSQL not running. Start with: brew services start postgresql@16"
    return
  fi

  local invoking_user
  invoking_user="${SUDO_USER:-$(logname 2>/dev/null || echo root)}"

  # Create database if not exists (run as the invoking user, not root)
  if su "$invoking_user" -c "psql -d postgres -tAc \"SELECT 1 FROM pg_database WHERE datname='enforcer'\"" 2>/dev/null | grep -q 1; then
    echo -e "${GREEN}[OK]${NC} Database enforcer exists"
  else
    su "$invoking_user" -c "createdb enforcer" 2>/dev/null && echo -e "${GREEN}[OK]${NC} Database enforcer created" || echo -e "${YELLOW}[WARN]${NC} Could not create database"
  fi

  # Run schema migration
  if su "$invoking_user" -c "psql -d enforcer -c 'SELECT 1 FROM audit_events LIMIT 0'" >/dev/null 2>&1; then
    echo -e "${GREEN}[OK]${NC} Schema already applied"
  else
    su "$invoking_user" -c "psql -d enforcer -f '$PROJECT_ROOT/docker/init.sql'" >/dev/null 2>&1 && echo -e "${GREEN}[OK]${NC} Schema migration applied" || echo -e "${YELLOW}[WARN]${NC} Schema migration failed"
  fi
}

# ── Deploy Central Server ────────────────────────────────────────────────────

deploy_central() {
  banner "Deploying Hub Server"

  setup_directories
  setup_certs
  setup_admin_token
  copy_application
  setup_database

  local app_dir="$INSTALL_DIR/app"
  local node_path=$(which node)
  local npx_path=$(which npx)

  # Create wrapper script in /usr/local/bin
  cat > "$BIN_DIR/enforcer-central" << WRAPPER
#!/bin/bash
cd "$app_dir"
exec npx tsx src/central/server.ts "\$@"
WRAPPER
  chmod 755 "$BIN_DIR/enforcer-central"
  echo -e "${GREEN}[OK]${NC} Hub Server binary: $BIN_DIR/enforcer-central"

  # Create LaunchDaemon plist
  cat > "$CENTRAL_PLIST" << PLISTEOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${CENTRAL_SERVICE}</string>
    <key>ProgramArguments</key>
    <array>
        <string>${npx_path}</string>
        <string>tsx</string>
        <string>${app_dir}/src/central/server.ts</string>
    </array>
    <key>WorkingDirectory</key>
    <string>${app_dir}</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin</string>
        <key>CENTRAL_PORT</key>
        <string>${CENTRAL_PORT}</string>
        <key>CONSOLE_API_PORT</key>
        <string>${ADMIN_PORT}</string>
        <key>CERT_DIR</key>
        <string>${CERT_DIR}</string>
        <key>AA_ADMIN_TOKEN</key>
        <string>${ADMIN_TOKEN}</string>
        <key>AA_POLICY_DIR</key>
        <string>${CONFIG_DIR}</string>
        <key>NODE_ENV</key>
        <string>production</string>
    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <dict><key>SuccessfulExit</key><false/></dict>
    <key>StandardOutPath</key>
    <string>${LOG_DIR}/central.log</string>
    <key>StandardErrorPath</key>
    <string>${LOG_DIR}/central.error.log</string>
</dict>
</plist>
PLISTEOF

  chmod 644 "$CENTRAL_PLIST"
  chown root:wheel "$CENTRAL_PLIST"

  # Load service
  launchctl bootstrap system "$CENTRAL_PLIST" 2>/dev/null || launchctl load "$CENTRAL_PLIST" 2>/dev/null || true
  sleep 2

  echo ""
  echo -e "${GREEN}${BOLD}Central Server Deployed${NC}"
  echo ""
  echo "  Installed:     $INSTALL_DIR"
  echo "  Client API:    https://0.0.0.0:${CENTRAL_PORT} (mTLS)"
  echo "  Admin API:     http://0.0.0.0:${ADMIN_PORT}"
  echo "  Certificates:  $CERT_DIR"
  echo "  Admin token:   $CONFIG_DIR/.admin_token"
  echo "  Logs:          $LOG_DIR/central.log"
  echo "  Binary:        $BIN_DIR/enforcer-central"
  echo ""
  banner "Hub Server Deploy Complete"
}

# ── Deploy Client Agent ──────────────────────────────────────────────────────

deploy_client() {
  local governed_user="${TARGET_USER:-${SUDO_USER:-$(logname 2>/dev/null || echo $(whoami))}}"

  if [[ "$ALL_USERS" == "true" ]]; then
    banner "Deploying Sentinel Server (All Users)"
    echo -e "${YELLOW}[NOTE]${NC} Client will govern ALL users on this machine"
  else
    banner "Deploying Sentinel Server (User: $governed_user)"
  fi

  setup_directories
  setup_certs
  setup_admin_token
  copy_application
  setup_database

  local app_dir="$INSTALL_DIR/app"
  local npx_path=$(which npx)
  local central_url="${CENTRAL_URL:-https://localhost:${CENTRAL_PORT}}"

  # Create wrapper script
  cat > "$BIN_DIR/enforcer-client" << WRAPPER
#!/bin/bash
cd "$app_dir"
exec npx tsx src/daemon/server.ts "\$@"
WRAPPER
  chmod 755 "$BIN_DIR/enforcer-client"
  echo -e "${GREEN}[OK]${NC} Sentinel binary: $BIN_DIR/enforcer-client"

  # Create status script
  cat > "$BIN_DIR/enforcer-status" << WRAPPER
#!/bin/bash
"$SCRIPT_DIR/enforcer_deploy.sh" status
WRAPPER
  chmod 755 "$BIN_DIR/enforcer-status"

  # Create LaunchDaemon plist for Sentinel (runs as system — developer cannot stop)
  cat > "$CLIENT_PLIST" << PLISTEOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${CLIENT_SERVICE}</string>
    <key>ProgramArguments</key>
    <array>
        <string>${npx_path}</string>
        <string>tsx</string>
        <string>${app_dir}/src/daemon/server.ts</string>
    </array>
    <key>WorkingDirectory</key>
    <string>${app_dir}</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin</string>
        <key>DAEMON_PORT</key>
        <string>${DAEMON_PORT}</string>
        <key>PROXY_PORT</key>
        <string>${PROXY_PORT}</string>
        <key>AA_ADMIN_TOKEN</key>
        <string>${ADMIN_TOKEN}</string>
        <key>AA_POLICY_DIR</key>
        <string>${CONFIG_DIR}</string>
        <key>AA_CENTRAL_URL</key>
        <string>${central_url}</string>
        <key>CERT_DIR</key>
        <string>${CERT_DIR}</string>
        <key>DATABASE_URL</key>
        <string>postgresql://${governed_user}@localhost:5432/enforcer</string>
        <key>NODE_ENV</key>
        <string>production</string>
    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <dict><key>SuccessfulExit</key><false/></dict>
    <key>StandardOutPath</key>
    <string>${LOG_DIR}/client.log</string>
    <key>StandardErrorPath</key>
    <string>${LOG_DIR}/client.error.log</string>
</dict>
</plist>
PLISTEOF

  chmod 644 "$CLIENT_PLIST"
  chown root:wheel "$CLIENT_PLIST"

  # Install managed hooks for Claude Code (developer cannot remove)
  local managed_dir="/Library/Application Support/ClaudeCode"
  local managed_settings="$managed_dir/managed-settings.json"
  local handler_path="$app_dir/src/enforcement/hook-handler.ts"

  mkdir -p "$managed_dir"

  cat > "$managed_settings" << MANAGEDEOF
{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Read", "hooks": [{"type": "command", "command": "npx tsx $handler_path pre_tool_call"}]},
      {"matcher": "Edit", "hooks": [{"type": "command", "command": "npx tsx $handler_path pre_tool_call"}]},
      {"matcher": "Write", "hooks": [{"type": "command", "command": "npx tsx $handler_path pre_tool_call"}]},
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "npx tsx $handler_path pre_tool_call"}]},
      {"matcher": "Glob", "hooks": [{"type": "command", "command": "npx tsx $handler_path pre_tool_call"}]},
      {"matcher": "Grep", "hooks": [{"type": "command", "command": "npx tsx $handler_path pre_tool_call"}]}
    ],
    "PostToolUse": [
      {"matcher": "Read", "hooks": [{"type": "command", "command": "npx tsx $handler_path post_tool_call"}]},
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "npx tsx $handler_path post_tool_call"}]}
    ]
  },
  "allowManagedHooksOnly": true
}
MANAGEDEOF

  chmod 644 "$managed_settings"
  chown root:wheel "$managed_settings"
  echo -e "${GREEN}[OK]${NC} Managed hooks installed (developer cannot remove, allowManagedHooksOnly=true)"

  # Load service
  launchctl bootstrap system "$CLIENT_PLIST" 2>/dev/null || launchctl load "$CLIENT_PLIST" 2>/dev/null || true
  sleep 2

  echo ""
  echo -e "${GREEN}${BOLD}Client Agent Deployed${NC}"
  echo ""
  echo "  Installed:       $INSTALL_DIR"
  echo "  Sentinel Server API:  http://127.0.0.1:${DAEMON_PORT}"
  echo "  Sentinel Server Egress:   http://127.0.0.1:${PROXY_PORT}"
  echo "  Hub Server:  ${central_url}"
  echo "  Governed user:   ${governed_user}${ALL_USERS:+ (all users)}"
  echo "  Managed hooks:   $managed_settings"
  echo "  Logs:            $LOG_DIR/client.log"
  echo "  Binary:          $BIN_DIR/enforcer-client"
  echo ""
  echo "  The developer ${BOLD}CANNOT${NC}:"
  echo "    - Stop the Sentinel Server (it's a system LaunchDaemon)"
  echo "    - Edit policies ($CONFIG_DIR owned by root)"
  echo "    - Remove hooks (managed by system, allowManagedHooksOnly=true)"
  echo "    - Access admin token ($CONFIG_DIR/.admin_token mode 600)"
  echo "    - Delete audit logs ($LOG_DIR owned by root)"
  echo ""
  echo "  The developer needs to: Reload VS Code (Cmd+Shift+P → Reload Window)"
  echo ""
  banner "Sentinel Server Deploy Complete"
}

# ── Execute ──────────────────────────────────────────────────────────────────

case "$MODE" in
  central)
    deploy_central
    ;;
  client)
    deploy_client
    ;;
  full)
    deploy_central
    deploy_client
    banner "Full Deploy Complete"
    echo ""
    echo -e "${GREEN}${BOLD}Full deployment complete.${NC}"
    echo "  Central + Client running on this machine."
    echo "  Hub Admin Console: http://localhost:${ADMIN_PORT}"
    echo ""
    ;;
esac
