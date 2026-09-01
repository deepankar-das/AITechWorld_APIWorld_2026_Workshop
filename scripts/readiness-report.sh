#!/bin/bash
# Enforcer — Readiness Gate Report
#
# Queries /v1/metrics and formats readiness gate results.
#
# Usage:
#   ./scripts/readiness-report.sh           # Report against running Sentinel Server
#   ./scripts/readiness-report.sh --json    # Output raw JSON

set -euo pipefail

DAEMON_URL="${AA_DAEMON_URL:-http://127.0.0.1:9100}"
JSON_ONLY=false

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
DIM='\033[2m'
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

while [[ $# -gt 0 ]]; do
  case "$1" in
    --json) JSON_ONLY=true; shift ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

# Fetch metrics
METRICS=$(curl -sf "$DAEMON_URL/v1/metrics" 2>/dev/null) || {
  echo -e "${RED}[ERROR]${NC} Cannot reach Sentinel Server at $DAEMON_URL"
  echo "  Start it with: ./scripts/deploy.sh"
  exit 1
}

if [[ "$JSON_ONLY" == "true" ]]; then
  echo "$METRICS" | node -pe 'JSON.stringify(JSON.parse(require("fs").readFileSync(0,"utf8")), null, 2)'
  exit 0
fi

banner "Readiness Gate Report Start"

# Parse and display gates
node -e "
  const metrics = JSON.parse(process.argv[1]);

  // System status
  console.log('  Policy: ' + metrics.policy.version + ' (' + metrics.policy.rules + ' rules)');
  console.log('  Buffer: ' + metrics.buffer.count + '/' + metrics.buffer.maxSize + ' events');
  console.log('  Store:  ' + metrics.store.totalEvents + ' events');
  console.log('  Approvals: ' + metrics.approval.totalCreated + ' created, ' + metrics.approval.pendingCount + ' pending');
  console.log('');

  // Gates
  console.log('  Readiness Gates:');
  console.log('  ─────────────────────────────────────────────────');

  let allPass = true;
  for (const gate of metrics.readinessGates) {
    const icon = gate.pass ? '\x1b[32m✓\x1b[0m' : '\x1b[31m✗\x1b[0m';
    const status = gate.pass ? '\x1b[32mPASS\x1b[0m' : '\x1b[31mFAIL\x1b[0m';
    if (!gate.pass) allPass = false;
    const name = gate.name.padEnd(30);
    const target = gate.target.padEnd(12);
    console.log('  ' + icon + ' ' + name + target + 'Actual: ' + gate.actual + '  ' + status);
  }

  console.log('  ─────────────────────────────────────────────────');
  if (allPass) {
    console.log('  \x1b[32m\x1b[1mAll readiness gates PASS.\x1b[0m');
  } else {
    console.log('  \x1b[31m\x1b[1mSome gates did not pass. Review before Phase 2.\x1b[0m');
  }
  console.log('');
" "$METRICS"

banner "Readiness Gate Report Complete"
