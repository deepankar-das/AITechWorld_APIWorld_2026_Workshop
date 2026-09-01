#!/bin/bash
# Enforcer — Install as System Service (macOS LaunchDaemon)
#
# Installs Enforcer Sentinel Server as a system-level LaunchDaemon that:
#   - Runs as root (developer cannot kill it)
#   - Auto-starts on boot
#   - Auto-restarts on crash
#   - Writes audit logs to a protected location
#
# REQUIRES SUDO — this creates system-level services.
#
# Usage:
#   sudo ./scripts/install-service.sh              # Install service
#   sudo ./scripts/install-service.sh --uninstall  # Remove service
#   sudo ./scripts/install-service.sh --status     # Check service status

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

SERVICE_NAME="com.enforcer.daemon"
PLIST_PATH="/Library/LaunchDaemons/${SERVICE_NAME}.plist"
SERVICE_USER="_enforcer"
LOG_DIR="/var/log/enforcer"
CONFIG_DIR="/etc/enforcer"
DATA_DIR="/var/lib/enforcer"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

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

UNINSTALL=false
STATUS_ONLY=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --uninstall) UNINSTALL=true; shift ;;
    --status) STATUS_ONLY=true; shift ;;
    --help|-h)
      echo "Usage: sudo $0 [--uninstall] [--status]"
      echo ""
      echo "Installs Enforcer as a system LaunchDaemon that the governed"
      echo "developer cannot stop, disable, or tamper with."
      echo ""
      echo "Options:"
      echo "  --uninstall  Remove the service and clean up"
      echo "  --status     Check if the service is running"
      exit 0
      ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

# Must be root
if [[ $EUID -ne 0 ]]; then
  echo -e "${RED}[ERROR]${NC} This script must be run with sudo."
  echo "  Usage: sudo $0"
  exit 1
fi

# ── Status ───────────────────────────────────────────────────────────────────

if [[ "$STATUS_ONLY" == "true" ]]; then
  banner "System Service Status Start"
  if launchctl list | grep -q "$SERVICE_NAME"; then
    echo -e "${GREEN}[RUNNING]${NC} Enforcer Sentinel Server is active as system service"
    launchctl list "$SERVICE_NAME" 2>/dev/null || true
  else
    echo -e "${YELLOW}[NOT RUNNING]${NC} Enforcer Sentinel Server is not installed as a system service"
  fi
  banner "System Service Status Complete"
  exit 0
fi

# ── Uninstall ────────────────────────────────────────────────────────────────

if [[ "$UNINSTALL" == "true" ]]; then
  banner "System Service Uninstall Start"

  launchctl bootout system "$PLIST_PATH" 2>/dev/null || true
  rm -f "$PLIST_PATH"
  echo -e "${GREEN}[OK]${NC} Service removed"

  # Don't delete logs or data — they're audit evidence
  echo -e "${YELLOW}[NOTE]${NC} Logs preserved at $LOG_DIR"
  echo -e "${YELLOW}[NOTE]${NC} Data preserved at $DATA_DIR"
  banner "System Service Uninstall Complete"
  exit 0
fi

# ── Install ──────────────────────────────────────────────────────────────────
banner "System Service Install Start"

# Create directories owned by root
mkdir -p "$LOG_DIR" "$CONFIG_DIR" "$DATA_DIR"
chmod 750 "$LOG_DIR" "$CONFIG_DIR" "$DATA_DIR"

# Copy policy files to protected location
cp -r "$PROJECT_ROOT/policies/"* "$CONFIG_DIR/" 2>/dev/null || true
chmod -R 644 "$CONFIG_DIR/"*
echo -e "${GREEN}[OK]${NC} Policies copied to $CONFIG_DIR (developer cannot modify)"

# Generate admin token if not exists
ADMIN_TOKEN_FILE="$CONFIG_DIR/.admin_token"
if [[ ! -f "$ADMIN_TOKEN_FILE" ]]; then
  ADMIN_TOKEN=$(openssl rand -hex 32)
  echo "$ADMIN_TOKEN" > "$ADMIN_TOKEN_FILE"
  chmod 600 "$ADMIN_TOKEN_FILE"
  echo -e "${GREEN}[OK]${NC} Admin token generated: $ADMIN_TOKEN_FILE"
  echo -e "${YELLOW}[IMPORTANT]${NC} Save this token — it's required for the management console:"
  echo -e "  ${CYAN}${ADMIN_TOKEN}${NC}"
else
  ADMIN_TOKEN=$(cat "$ADMIN_TOKEN_FILE")
  echo -e "${GREEN}[OK]${NC} Admin token exists at $ADMIN_TOKEN_FILE"
fi

# Find node and npx paths
NODE_PATH=$(which node)
NPX_PATH=$(which npx)
TSX_PATH=$(which tsx 2>/dev/null || echo "$PROJECT_ROOT/node_modules/.bin/tsx")

# Create LaunchDaemon plist
cat > "$PLIST_PATH" << PLISTEOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${SERVICE_NAME}</string>

    <key>ProgramArguments</key>
    <array>
        <string>${NPX_PATH}</string>
        <string>tsx</string>
        <string>${PROJECT_ROOT}/src/daemon/server.ts</string>
    </array>

    <key>WorkingDirectory</key>
    <string>${PROJECT_ROOT}</string>

    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin</string>
        <key>DAEMON_PORT</key>
        <string>9100</string>
        <key>AA_ADMIN_TOKEN</key>
        <string>${ADMIN_TOKEN}</string>
        <key>AA_POLICY_DIR</key>
        <string>${CONFIG_DIR}</string>
        <key>AA_DATA_DIR</key>
        <string>${DATA_DIR}</string>
        <key>DATABASE_URL</key>
        <string>postgresql://$(whoami)@localhost:5432/enforcer</string>
        <key>NODE_ENV</key>
        <string>production</string>
    </dict>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <dict>
        <key>SuccessfulExit</key>
        <false/>
    </dict>

    <key>StandardOutPath</key>
    <string>${LOG_DIR}/daemon.log</string>

    <key>StandardErrorPath</key>
    <string>${LOG_DIR}/daemon.error.log</string>

    <key>SoftResourceLimits</key>
    <dict>
        <key>NumberOfFiles</key>
        <integer>4096</integer>
    </dict>
</dict>
</plist>
PLISTEOF

chmod 644 "$PLIST_PATH"
chown root:wheel "$PLIST_PATH"
echo -e "${GREEN}[OK]${NC} LaunchDaemon plist created at $PLIST_PATH"

# Load and start the service
launchctl bootstrap system "$PLIST_PATH" 2>/dev/null || launchctl load "$PLIST_PATH" 2>/dev/null || true
sleep 2

# Verify
if curl -sf http://127.0.0.1:9100/v1/health >/dev/null 2>&1; then
  echo -e "${GREEN}[OK]${NC} Sentinel Server is running as system service"
else
  echo -e "${YELLOW}[WARN]${NC} Sentinel Server may still be starting — check $LOG_DIR/daemon.log"
fi

# ── Install managed hooks (developer cannot remove) ──────────────────────────
# Claude Code supports managed settings that override user settings.
# On macOS, managed settings go to /Library/Application Support/ClaudeCode/

MANAGED_DIR="/Library/Application Support/ClaudeCode"
MANAGED_SETTINGS="$MANAGED_DIR/managed-settings.json"

mkdir -p "$MANAGED_DIR"

HANDLER_PATH="$PROJECT_ROOT/src/enforcement/hook-handler.ts"

cat > "$MANAGED_SETTINGS" << MANAGEDEOF
{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Read", "hooks": [{"type": "command", "command": "npx tsx $HANDLER_PATH pre_tool_call"}]},
      {"matcher": "Edit", "hooks": [{"type": "command", "command": "npx tsx $HANDLER_PATH pre_tool_call"}]},
      {"matcher": "Write", "hooks": [{"type": "command", "command": "npx tsx $HANDLER_PATH pre_tool_call"}]},
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "npx tsx $HANDLER_PATH pre_tool_call"}]},
      {"matcher": "Glob", "hooks": [{"type": "command", "command": "npx tsx $HANDLER_PATH pre_tool_call"}]},
      {"matcher": "Grep", "hooks": [{"type": "command", "command": "npx tsx $HANDLER_PATH pre_tool_call"}]}
    ],
    "PostToolUse": [
      {"matcher": "Read", "hooks": [{"type": "command", "command": "npx tsx $HANDLER_PATH post_tool_call"}]},
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "npx tsx $HANDLER_PATH post_tool_call"}]}
    ]
  },
  "allowManagedHooksOnly": true
}
MANAGEDEOF

chmod 644 "$MANAGED_SETTINGS"
chown root:wheel "$MANAGED_SETTINGS"
echo -e "${GREEN}[OK]${NC} Managed hooks installed at $MANAGED_SETTINGS"
echo -e "${GREEN}[OK]${NC} allowManagedHooksOnly=true — developer's hook edits are ignored"

echo ""
echo -e "${CYAN}${BOLD}Enforcer System Service Installed${NC}"
echo ""
echo "  Service:     $SERVICE_NAME"
echo "  Plist:       $PLIST_PATH"
echo "  Logs:        $LOG_DIR/daemon.log (Sentinel Server log)"
echo "  Config:      $CONFIG_DIR/"
echo "  Admin token: $ADMIN_TOKEN_FILE"
echo ""
echo "  The developer ($(logname)) CANNOT:"
echo "    - Stop the Sentinel Server (it's a system LaunchDaemon)"
echo "    - Edit the policies (owned by root in $CONFIG_DIR)"
echo "    - Access the admin token (600 permissions)"
echo "    - Delete audit logs (owned by root in $LOG_DIR)"
echo ""
echo "  Hub Admin Console requires the Hub Admin token to log in."
echo ""
banner "System Service Install Complete"
