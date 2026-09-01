#!/bin/bash
# Enforcer — Install Claude Code Hooks
#
# Adds Enforcer enforcement hooks to Claude Code's settings.json.
# Hooks intercept tool calls and route them through the Sentinel Server for policy evaluation.
#
# Usage:
#   ./scripts/install-hooks.sh              # Install hooks
#   ./scripts/install-hooks.sh --uninstall  # Remove hooks
#   ./scripts/install-hooks.sh --status     # Check if hooks are installed

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
SETTINGS_FILE="$HOME/.claude/settings.json"
HANDLER_PATH="$PROJECT_ROOT/src/enforcement/hook-handler.ts"

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
      echo "Usage: $0 [--uninstall] [--status]"
      echo ""
      echo "Options:"
      echo "  --uninstall  Remove Enforcer hooks from Claude Code settings"
      echo "  --status     Check if hooks are currently installed"
      exit 0
      ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

# ── Check prerequisites ──────────────────────────────────────────────────────

if [[ ! -f "$SETTINGS_FILE" ]]; then
  echo -e "${RED}[ERROR]${NC} Claude Code settings not found at $SETTINGS_FILE"
  echo "  Ensure Claude Code is installed and has been run at least once."
  exit 1
fi

if [[ ! -f "$HANDLER_PATH" ]]; then
  echo -e "${RED}[ERROR]${NC} Hook handler not found at $HANDLER_PATH"
  echo "  Run from the Enforcer project root."
  exit 1
fi

# ── Status check ─────────────────────────────────────────────────────────────

if [[ "$STATUS_ONLY" == "true" ]]; then
  # Managed hooks (enterprise) — takes precedence over everything
  MANAGED="/Library/Application Support/ClaudeCode/managed-settings.json"
  if [[ -f "$MANAGED" ]]; then
    MANAGED_ONLY=$(node -pe 'JSON.parse(require("fs").readFileSync("'"$MANAGED"'","utf8")).allowManagedHooksOnly || false' 2>/dev/null || echo "false")
    echo -e "${GREEN}[ENFORCED]${NC}  Managed hooks active (allowManagedHooksOnly=$MANAGED_ONLY)"
  else
    # No managed hooks — check project-level hooks as fallback
    if grep -q "enforcer" "$SETTINGS_FILE" 2>/dev/null; then
      echo -e "${GREEN}[INSTALLED]${NC} Project hooks active in $SETTINGS_FILE"
    else
      echo -e "${YELLOW}[NOT INSTALLED]${NC} No hooks found"
      echo "  Install managed hooks: sudo ./scripts/deploy_sentinel.sh"
      echo "  Install project hooks: ./scripts/install-hooks.sh"
    fi
  fi

  # Hook binary
  if [[ -x "/usr/local/bin/enforcer-hook" ]]; then
    echo -e "${GREEN}[OK]${NC}        Hook binary: /usr/local/bin/enforcer-hook"
  else
    echo -e "${RED}[MISSING]${NC}     Hook binary not found at /usr/local/bin/enforcer-hook"
  fi

  # Daemon health
  if curl -sf http://127.0.0.1:9100/v1/health >/dev/null 2>&1; then
    echo -e "${GREEN}[OK]${NC}        Sentinel daemon running on port 9100"
  else
    echo -e "${RED}[DOWN]${NC}      Sentinel daemon not running on port 9100"
  fi
  exit 0
fi

# ── Uninstall ────────────────────────────────────────────────────────────────

if [[ "$UNINSTALL" == "true" ]]; then
  banner "Hook Uninstall Start"
  # Remove hooks using Node.js for safe JSON manipulation (handles both old and new key formats)
  node -e "
    const fs = require('fs');
    const settings = JSON.parse(fs.readFileSync('$SETTINGS_FILE', 'utf-8'));
    if (settings.hooks) {
      for (const key of ['preToolUse', 'postToolUse', 'PreToolUse', 'PostToolUse']) {
        if (Array.isArray(settings.hooks[key])) {
          settings.hooks[key] = settings.hooks[key].filter(
            h => !JSON.stringify(h).includes('enforcer-hook') && !JSON.stringify(h).includes('hook-handler.ts')
          );
          if (settings.hooks[key].length === 0) delete settings.hooks[key];
        }
      }
      if (Object.keys(settings.hooks).length === 0) delete settings.hooks;
    }
    fs.writeFileSync('$SETTINGS_FILE', JSON.stringify(settings, null, 2) + '\n');
  "
  echo -e "${GREEN}[OK]${NC} Enforcer hooks removed from $SETTINGS_FILE"
  banner "Hook Uninstall Complete"
  exit 0
fi

# ── Install ──────────────────────────────────────────────────────────────────
banner "Hook Install Start"

# Backup current settings
cp "$SETTINGS_FILE" "${SETTINGS_FILE}.bak.$(date +%Y%m%d%H%M%S)"

# Add hooks using Node.js for safe JSON manipulation
# Correct format: camelCase keys, individual matchers, command field (not hooks array)
node -e "
  const fs = require('fs');
  const settings = JSON.parse(fs.readFileSync('$SETTINGS_FILE', 'utf-8'));

  // Initialize hooks object if not present
  if (!settings.hooks) settings.hooks = {};

  const preCmd = 'npx tsx $HANDLER_PATH pre_tool_call';
  const postCmd = 'npx tsx $HANDLER_PATH post_tool_call';

  // Tools to intercept — one entry per tool (Claude Code requires individual matchers)
  const tools = ['Read', 'Edit', 'Write', 'Bash', 'Glob', 'Grep', 'WebFetch', 'WebSearch'];

  // Remove any existing Enforcer hooks first (check both old and new key formats)
  for (const key of ['preToolUse', 'postToolUse', 'PreToolUse', 'PostToolUse']) {
    if (Array.isArray(settings.hooks[key])) {
      settings.hooks[key] = settings.hooks[key].filter(
        h => !JSON.stringify(h).includes('enforcer-hook') && !JSON.stringify(h).includes('hook-handler.ts')
      );
      if (settings.hooks[key].length === 0) delete settings.hooks[key];
    }
  }

  // Add preToolUse hooks (blocking — can prevent execution)
  if (!Array.isArray(settings.hooks.preToolUse)) settings.hooks.preToolUse = [];
  for (const tool of tools) {
    settings.hooks.preToolUse.push({
      matcher: tool,
      command: preCmd + ' # enforcer-hook',
      type: 'command'
    });
  }

  // Add postToolUse hooks (audit enrichment — non-blocking)
  if (!Array.isArray(settings.hooks.postToolUse)) settings.hooks.postToolUse = [];
  for (const tool of tools) {
    settings.hooks.postToolUse.push({
      matcher: tool,
      command: postCmd + ' # enforcer-hook',
      type: 'command'
    });
  }

  fs.writeFileSync('$SETTINGS_FILE', JSON.stringify(settings, null, 2) + '\n');
"

banner "Hooks Installed Successfully"
echo -e "  Settings:  $SETTINGS_FILE"
echo -e "  Handler:   $HANDLER_PATH"
echo -e "  Sentinel Server:    http://127.0.0.1:9100"
echo -e "  Tools:     Read, Edit, Write, Bash, Glob, Grep, WebFetch, WebSearch"
echo ""
echo -e "  ${YELLOW}Before testing:${NC}"
echo -e "    1. Start the Sentinel Server:  ./scripts/deploy.sh"
echo -e "    2. Reload VS Code:    Cmd+Shift+P → 'Reload Window'"
echo ""
echo -e "  ${YELLOW}To uninstall:${NC}"
echo -e "    ./scripts/install-hooks.sh --uninstall"
echo ""
banner "Hook Install Complete"
