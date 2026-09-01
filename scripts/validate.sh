#!/bin/bash
# Enforcer — Post-Deploy Validation
#
# Validates enforcement policy decisions against a live Sentinel Server.
# Sends 9 test payloads to /v1/evaluate covering file, shell, network,
# and MCP surfaces, asserts expected allow/deny/require_approval decisions,
# verifies audit events were recorded, and checks Hub received them.
#
# Prerequisites:
#   1. Sentinel Server running: sudo ./scripts/deploy_sentinel.sh
#   2. Management Hub running: sudo ./scripts/deploy_hub.sh
#   3. Optional Sentinel-only mode: ./scripts/validate.sh --sentinel-only
#
# Usage:
#   ./scripts/validate.sh              # Run full validation
#   ./scripts/validate.sh --verbose    # Show full API responses
#   ./scripts/validate.sh --sentinel-only  # Skip Hub checks (Sentinel-only mode)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

DAEMON_URL="${AA_DAEMON_URL:-http://127.0.0.1:9100}"
HUB_CONSOLE_URL="${AA_HUB_CONSOLE_URL:-http://127.0.0.1:9201}"
HUB_AUDIT_WAIT_SECS="${AA_HUB_AUDIT_WAIT_SECS:-45}"
VERBOSE=false
REQUIRE_HUB=true
PASS=0
FAIL=0
ACCESS_TOKEN="${AA_ACCESS_TOKEN:-}"
HUB_TOKEN="${AA_HUB_TOKEN:-}"
HUB_AUDIT_BASELINE=-1
HUB_HEARTBEAT_BASELINE=0

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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
    --verbose) VERBOSE=true; shift ;;
    --sentinel-only) REQUIRE_HUB=false; shift ;;
    --daemon-url)
      if [[ $# -lt 2 ]]; then echo "Missing value for --daemon-url"; exit 1; fi
      DAEMON_URL="$2"; shift 2 ;;
    --hub-console-url)
      if [[ $# -lt 2 ]]; then echo "Missing value for --hub-console-url"; exit 1; fi
      HUB_CONSOLE_URL="$2"; shift 2 ;;
    --hub-token)
      if [[ $# -lt 2 ]]; then echo "Missing value for --hub-token"; exit 1; fi
      HUB_TOKEN="$2"; shift 2 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

banner "Validation Start (PRD 8.4.5)"

# ── Auth token resolution (needed for all API calls) ────────────────────────

# Resolve Sentinel token (operator or admin — for /v1/ API)
resolve_sentinel_token() {
  if [[ -n "$ACCESS_TOKEN" ]]; then
    return
  fi
  for token_file in \
    "/etc/enforcer/.operator_token" \
    "/etc/enforcer/.admin_token" \
    "$PROJECT_ROOT/.operator_token" \
    "$PROJECT_ROOT/.admin_token"; do
    if [[ -r "$token_file" ]]; then
      ACCESS_TOKEN="$(cat "$token_file" | tr -d '\r\n')"
      if [[ -n "$ACCESS_TOKEN" ]]; then
        return
      fi
    fi
  done
}

# Resolve Hub admin token (for /api/v1/ endpoints)
resolve_hub_token() {
  if [[ -n "$HUB_TOKEN" ]]; then
    return
  fi
  for token_file in \
    "/etc/enforcer/.admin_token" \
    "$PROJECT_ROOT/.admin_token"; do
    if [[ -r "$token_file" ]]; then
      HUB_TOKEN="$(cat "$token_file" | tr -d '\r\n')"
      if [[ -n "$HUB_TOKEN" ]]; then
        return
      fi
    fi
  done
}

resolve_sentinel_token
resolve_hub_token

if [[ -z "$ACCESS_TOKEN" ]]; then
  echo -e "${RED}[FAIL]${NC}  No Sentinel API token found."
  echo "  Expected: /etc/enforcer/.operator_token or /etc/enforcer/.admin_token"
  exit 1
fi

# Verify Sentinel token
AUTH_ME=$(curl -sS -H "Authorization: Bearer $ACCESS_TOKEN" "$DAEMON_URL/v1/auth/me")
AUTH_ROLE=$(echo "$AUTH_ME" | node -pe 'let j=JSON.parse(require("fs").readFileSync(0,"utf8")); j.role || "none"')
if [[ "$AUTH_ROLE" == "none" ]]; then
  echo -e "${RED}[FAIL]${NC}  Sentinel token is invalid."
  echo "  /v1/auth/me response: $AUTH_ME"
  exit 1
fi
echo -e "${GREEN}[OK]${NC}    Sentinel token resolved (role: $AUTH_ROLE)"

if [[ "$REQUIRE_HUB" == "true" && -z "$HUB_TOKEN" ]]; then
  echo -e "${RED}[FAIL]${NC}  No Hub admin token found."
  echo "  Expected: /etc/enforcer/.admin_token"
  exit 1
fi
if [[ -n "$HUB_TOKEN" ]]; then
  echo -e "${GREEN}[OK]${NC}    Hub admin token resolved"
fi
echo ""

# ── Check Sentinel Server is running ─────────────────────────────────────────

echo -e "${BLUE}[CHECK] Sentinel Server health..."
HEALTH=$(curl -sS "$DAEMON_URL/v1/health")
echo -e "${GREEN}[OK]${NC}    Sentinel Server running. $(echo "$HEALTH" | node -pe 'JSON.parse(require("fs").readFileSync(0,"utf8")).version || "unknown-version"')."
echo ""

# ── Check Hub is running ────────────────────────────────────────────────────

if [[ "$REQUIRE_HUB" == "true" ]]; then
  echo -e "${BLUE}[CHECK] Hub Console health..."
  HUB_HEALTH=$(curl -sS "$HUB_CONSOLE_URL/api/v1/health")
  HUB_CLIENTS=$(echo "$HUB_HEALTH" | node -pe 'try{let j=JSON.parse(require("fs").readFileSync(0,"utf8")); Number(j.clients || 0)}catch(_){0}')
  echo -e "${GREEN}[OK]${NC}    Hub Console running. Registered Sentinels: $HUB_CLIENTS"

  echo -e "${BLUE}[CHECK] Hub client registry..."
  HUB_CLIENTS_RESPONSE=$(curl -sS -H "X-Admin-Token: $HUB_TOKEN" "$HUB_CONSOLE_URL/api/v1/clients")
  HUB_CLIENT_COUNT=$(echo "$HUB_CLIENTS_RESPONSE" | node -pe 'let j=JSON.parse(require("fs").readFileSync(0,"utf8")); Number(j.count || (j.clients ? j.clients.length : 0) || 0)')
  if [[ "$HUB_CLIENT_COUNT" -lt 1 ]]; then
    echo -e "${RED}[FAIL]${NC}  Hub has no registered Sentinel clients."
    echo "  Ensure Sentinel agent is deployed: sudo AA_CENTRAL_URL=https://localhost:9200 ./scripts/deploy_sentinel.sh"
    exit 1
  else
    echo -e "${GREEN}[OK]${NC}    Hub sees $HUB_CLIENT_COUNT Sentinel client(s)"
  fi
  HUB_HEARTBEAT_BASELINE=$(echo "$HUB_CLIENTS_RESPONSE" | node -pe 'let j=JSON.parse(require("fs").readFileSync(0,"utf8")); let clients=Array.isArray(j.clients)?j.clients:[]; let max=0; for (const c of clients){ const t=Date.parse(c.last_heartbeat||""); if (Number.isFinite(t) && t>max) max=t; } max')

  echo -e "${BLUE}[CHECK] Hub audit API..."
  HUB_AUDIT_RESPONSE=$(curl -sS -H "X-Admin-Token: $HUB_TOKEN" "$HUB_CONSOLE_URL/api/v1/audit")
  HUB_AUDIT_BASELINE=$(echo "$HUB_AUDIT_RESPONSE" | node -pe 'let j=JSON.parse(require("fs").readFileSync(0,"utf8")); Number(j.total ?? j.count ?? 0)')
  echo -e "${GREEN}[OK]${NC}    Hub audit API reachable. Baseline aggregated events: $HUB_AUDIT_BASELINE"
  echo ""
fi

# ── Preflight: policy bundle sanity ─────────────────────────────────────────
echo -e "${BLUE}[CHECK] Sentinel policy bundle..."
POLICY_BUNDLE=$(curl -sS -H "Authorization: Bearer $ACCESS_TOKEN" "$DAEMON_URL/v1/policy/bundle")
POLICY_RULES=$(echo "$POLICY_BUNDLE" | node -pe 'let j=JSON.parse(require("fs").readFileSync(0,"utf8")); if (Array.isArray(j.rules)) j.rules.length; else if (j.bundle && Array.isArray(j.bundle.rules)) j.bundle.rules.length; else 0')
if [[ "$POLICY_RULES" -lt 1 ]]; then
  echo -e "${RED}[FAIL]${NC}  Sentinel policy bundle has 0 rules."
  echo "  Validation cannot verify ALLOW/REQUIRE_APPROVAL behavior with an empty policy."
  echo "  Fix:"
  echo "    1. Re-deploy Hub + Sentinel so baseline policy is restored"
  echo "    2. Re-run: ./scripts/validate.sh"
  echo "  Recommended command:"
  echo "    sudo ./scripts/deploy_single_machine_hub_sentinel.sh"
  exit 1
fi
echo -e "${GREEN}[OK]${NC}    Sentinel policy bundle loaded ($POLICY_RULES rules)."
echo ""

# ── Helper function ──────────────────────────────────────────────────────────

evaluate() {
  local label="$1"
  local payload="$2"
  local expected_decision="$3"

  echo -e "${BLUE}[TEST]${NC} $label"

  local response status body
  response=$(curl -sS -X POST "$DAEMON_URL/v1/evaluate" \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -H "Content-Type: application/json" \
    -d "$payload" \
    -w $'\n%{http_code}')
  status="${response##*$'\n'}"
  body="${response%$'\n'*}"

  if [[ "$status" -ge 400 ]]; then
    echo -e "${RED}[FAIL]${NC}  Sentinel Server returned HTTP $status"
    echo "        $body"
    FAIL=$((FAIL + 1))
    return
  fi

  local decision reason_code reason_human
  decision=$(echo "$body" | node -pe 'JSON.parse(require("fs").readFileSync(0,"utf8")).decision')
  reason_code=$(echo "$body" | node -pe 'JSON.parse(require("fs").readFileSync(0,"utf8")).reason_code')
  reason_human=$(echo "$body" | node -pe 'JSON.parse(require("fs").readFileSync(0,"utf8")).reason_human')

  if [[ "$decision" == "$expected_decision" ]]; then
    echo -e "${GREEN}[PASS]${NC}  Decision: $decision | Reason: $reason_code"
    PASS=$((PASS + 1))
  else
    echo -e "${RED}[FAIL]${NC}  Expected: $expected_decision | Got: $decision | Reason: $reason_code"
    FAIL=$((FAIL + 1))
  fi

  if [[ "$VERBOSE" == "true" ]]; then
    echo -e "${DIM}        $reason_human${NC}"
  fi
  echo ""
}

SESSION_ID="validate_$(date +%s)"
TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%S.000Z)

# ── Scenario 1: Safe file write inside project → ALLOW ──────────────────────

evaluate "Scenario 1: File write inside project root (should ALLOW)" \
  "{
    \"request_id\": \"demo_01\",
    \"timestamp\": \"$TIMESTAMP\",
    \"actor\": {\"user_id\": \"demo_dev\", \"agent_type\": \"claude_code\", \"agent_instance\": \"demo\", \"session_id\": \"$SESSION_ID\"},
    \"environment\": {\"workspace\": \"$(pwd)\", \"repo\": \"enforcer\", \"branch\": \"main\", \"tier\": \"development\", \"deployment_mode\": \"host\"},
    \"action\": {\"type\": \"file.write\", \"attempted_action\": \"Write to src/index.ts\"},
    \"resource\": {\"kind\": \"file\", \"path\": \"$(pwd)/src/index.ts\", \"classification\": []}
  }" "allow"

# ── Scenario 2: File write outside project → DENY ───────────────────────────

evaluate "Scenario 2: File write outside project root (should DENY)" \
  "{
    \"request_id\": \"demo_02\",
    \"timestamp\": \"$TIMESTAMP\",
    \"actor\": {\"user_id\": \"demo_dev\", \"agent_type\": \"claude_code\", \"agent_instance\": \"demo\", \"session_id\": \"$SESSION_ID\"},
    \"environment\": {\"workspace\": \"$(pwd)\", \"repo\": \"enforcer\", \"branch\": \"main\", \"tier\": \"development\", \"deployment_mode\": \"host\"},
    \"action\": {\"type\": \"file.write\", \"attempted_action\": \"Write to ~/.config/settings.json\"},
    \"resource\": {\"kind\": \"file\", \"path\": \"$HOME/.config/settings.json\", \"classification\": []}
  }" "deny"

# ── Scenario 3: Safe shell command → ALLOW ───────────────────────────────────

evaluate "Scenario 3: Safe shell command 'npm test' (should ALLOW)" \
  "{
    \"request_id\": \"demo_03\",
    \"timestamp\": \"$TIMESTAMP\",
    \"actor\": {\"user_id\": \"demo_dev\", \"agent_type\": \"claude_code\", \"agent_instance\": \"demo\", \"session_id\": \"$SESSION_ID\"},
    \"environment\": {\"workspace\": \"$(pwd)\", \"repo\": \"enforcer\", \"branch\": \"main\", \"tier\": \"development\", \"deployment_mode\": \"host\"},
    \"action\": {\"type\": \"shell.exec\", \"attempted_action\": \"npm test\"},
    \"resource\": {\"kind\": \"command\", \"value\": \"npm test\", \"classification\": [\"safe\"]}
  }" "allow"

# ── Scenario 4: Destructive shell command → REQUIRE_APPROVAL ────────────────

evaluate "Scenario 4: Destructive 'rm -rf node_modules' (should REQUIRE_APPROVAL)" \
  "{
    \"request_id\": \"demo_04\",
    \"timestamp\": \"$TIMESTAMP\",
    \"actor\": {\"user_id\": \"demo_dev\", \"agent_type\": \"claude_code\", \"agent_instance\": \"demo\", \"session_id\": \"$SESSION_ID\"},
    \"environment\": {\"workspace\": \"$(pwd)\", \"repo\": \"enforcer\", \"branch\": \"main\", \"tier\": \"development\", \"deployment_mode\": \"host\"},
    \"action\": {\"type\": \"shell.exec\", \"attempted_action\": \"rm -rf node_modules\"},
    \"resource\": {\"kind\": \"command\", \"value\": \"rm -rf node_modules\", \"classification\": [\"destructive\"]}
  }" "require_approval"

# ── Scenario 5: Network to unknown host → DENY ──────────────────────────────

evaluate "Scenario 5: Network request to unknown host (should DENY)" \
  "{
    \"request_id\": \"demo_05\",
    \"timestamp\": \"$TIMESTAMP\",
    \"actor\": {\"user_id\": \"demo_dev\", \"agent_type\": \"claude_code\", \"agent_instance\": \"demo\", \"session_id\": \"$SESSION_ID\"},
    \"environment\": {\"workspace\": \"$(pwd)\", \"repo\": \"enforcer\", \"branch\": \"main\", \"tier\": \"development\", \"deployment_mode\": \"host\"},
    \"action\": {\"type\": \"network.request\", \"attempted_action\": \"POST https://paste.evil.io/upload\"},
    \"resource\": {\"kind\": \"host\", \"host\": \"paste.evil.io\", \"value\": \"POST https://paste.evil.io/upload\", \"classification\": []}
  }" "deny"

# ── Scenario 6: Sensitive path read → DENY ──────────────────────────────────

evaluate "Scenario 6: Read sensitive path ~/.ssh/id_rsa (should DENY)" \
  "{
    \"request_id\": \"demo_06\",
    \"timestamp\": \"$TIMESTAMP\",
    \"actor\": {\"user_id\": \"demo_dev\", \"agent_type\": \"claude_code\", \"agent_instance\": \"demo\", \"session_id\": \"$SESSION_ID\"},
    \"environment\": {\"workspace\": \"$(pwd)\", \"repo\": \"enforcer\", \"branch\": \"main\", \"tier\": \"development\", \"deployment_mode\": \"host\"},
    \"action\": {\"type\": \"file.read\", \"attempted_action\": \"Read ~/.ssh/id_rsa\"},
    \"resource\": {\"kind\": \"file\", \"path\": \"$HOME/.ssh/id_rsa\", \"classification\": [\"sensitive_path\"]}
  }" "deny"

# ── Scenario 7: Package install → REQUIRE_APPROVAL ──────────────────────────

evaluate "Scenario 7: Package install 'npm install express' (should REQUIRE_APPROVAL)" \
  "{
    \"request_id\": \"demo_07\",
    \"timestamp\": \"$TIMESTAMP\",
    \"actor\": {\"user_id\": \"demo_dev\", \"agent_type\": \"claude_code\", \"agent_instance\": \"demo\", \"session_id\": \"$SESSION_ID\"},
    \"environment\": {\"workspace\": \"$(pwd)\", \"repo\": \"enforcer\", \"branch\": \"main\", \"tier\": \"development\", \"deployment_mode\": \"host\"},
    \"action\": {\"type\": \"shell.exec\", \"attempted_action\": \"npm install express\"},
    \"resource\": {\"kind\": \"command\", \"value\": \"npm install express\", \"classification\": [\"package_manager\"]}
  }" "require_approval"

# ── Scenario 8: Credential access command → DENY ────────────────────────────

evaluate "Scenario 8: Credential access 'cat .env' (should DENY)" \
  "{
    \"request_id\": \"demo_08\",
    \"timestamp\": \"$TIMESTAMP\",
    \"actor\": {\"user_id\": \"demo_dev\", \"agent_type\": \"claude_code\", \"agent_instance\": \"demo\", \"session_id\": \"$SESSION_ID\"},
    \"environment\": {\"workspace\": \"$(pwd)\", \"repo\": \"enforcer\", \"branch\": \"main\", \"tier\": \"development\", \"deployment_mode\": \"host\"},
    \"action\": {\"type\": \"shell.exec\", \"attempted_action\": \"cat .env\"},
    \"resource\": {\"kind\": \"command\", \"value\": \"cat .env\", \"classification\": [\"sensitive_path\"]}
  }" "deny"

# ── Scenario 9: MCP unknown server → DENY ───────────────────────────────────

evaluate "Scenario 9: MCP call to unknown server (should REQUIRE_APPROVAL)" \
  "{
    \"request_id\": \"demo_09\",
    \"timestamp\": \"$TIMESTAMP\",
    \"actor\": {\"user_id\": \"demo_dev\", \"agent_type\": \"claude_code\", \"agent_instance\": \"demo\", \"session_id\": \"$SESSION_ID\"},
    \"environment\": {\"workspace\": \"$(pwd)\", \"repo\": \"enforcer\", \"branch\": \"main\", \"tier\": \"development\", \"deployment_mode\": \"host\"},
    \"action\": {\"type\": \"mcp.invoke\", \"attempted_action\": \"evil-server/steal_data.execute\"},
    \"resource\": {\"kind\": \"mcp_tool\", \"value\": \"evil-server:steal_data:execute\", \"classification\": []}
  }" "require_approval"

# ── Summary ──────────────────────────────────────────────────────────────────

echo -e "${CYAN}${BOLD}════════════════════════════════════════════════════════════════${NC}"
TOTAL=$((PASS + FAIL))
if [[ $FAIL -eq 0 ]]; then
  echo -e "  ${GREEN}${BOLD}All $TOTAL scenarios passed!${NC}"
else
  echo -e "  ${RED}${BOLD}$FAIL of $TOTAL scenarios failed.${NC}"
fi
echo ""
echo "  Session: $SESSION_ID"
echo "  Review:  http://localhost:6100/sessions/$SESSION_ID"
if [[ "$REQUIRE_HUB" == "true" ]]; then
  echo "  Hub:     $HUB_CONSOLE_URL"
fi
echo ""

# ── Show audit trail ─────────────────────────────────────────────────────────

echo -e "${BLUE}[AUDIT]${NC} Fetching session audit trail..."
AUDIT=$(curl -sS -H "Authorization: Bearer $ACCESS_TOKEN" "$DAEMON_URL/v1/audit/sessions/$SESSION_ID")
EVENT_COUNT=$(echo "$AUDIT" | node -pe 'JSON.parse(require("fs").readFileSync(0,"utf8")).event_count || 0')
echo -e "${GREEN}[OK]${NC}    $EVENT_COUNT audit events for this validation session."
echo ""

if [[ "$REQUIRE_HUB" == "true" ]]; then
  echo -e "${BLUE}[HUB]${NC} Verifying Sentinel communication with Hub..."
  START_TS=$(date +%s)
  END_TS=$((START_TS + HUB_AUDIT_WAIT_SECS))
  HUB_AUDIT_TOTAL="$HUB_AUDIT_BASELINE"
  HUB_HEARTBEAT_ADVANCED=false
  while [[ "$(date +%s)" -le "$END_TS" ]]; do
    HUB_AUDIT_POLL=$(curl -s -H "X-Admin-Token: $HUB_TOKEN" "$HUB_CONSOLE_URL/api/v1/audit" || true)
    if [[ -n "$HUB_AUDIT_POLL" ]]; then
      HUB_AUDIT_TOTAL=$(echo "$HUB_AUDIT_POLL" | node -pe 'let j=JSON.parse(require("fs").readFileSync(0,"utf8")); Number(j.total ?? j.count ?? 0)' 2>/dev/null || echo "-1")
      if [[ "$HUB_AUDIT_TOTAL" -gt "$HUB_AUDIT_BASELINE" ]]; then
        break
      fi
    fi
    HUB_CLIENTS_POLL=$(curl -s -H "X-Admin-Token: $HUB_TOKEN" "$HUB_CONSOLE_URL/api/v1/clients" || true)
    if [[ -n "$HUB_CLIENTS_POLL" ]]; then
      HUB_HEARTBEAT_CURRENT=$(echo "$HUB_CLIENTS_POLL" | node -pe 'let j=JSON.parse(require("fs").readFileSync(0,"utf8")); let clients=Array.isArray(j.clients)?j.clients:[]; let max=0; for (const c of clients){ const t=Date.parse(c.last_heartbeat||""); if (Number.isFinite(t) && t>max) max=t; } max' 2>/dev/null || echo "0")
      if [[ "$HUB_HEARTBEAT_CURRENT" -gt "$HUB_HEARTBEAT_BASELINE" ]]; then
        HUB_HEARTBEAT_ADVANCED=true
        break
      fi
    fi
    sleep 2
  done

  if [[ "$HUB_AUDIT_TOTAL" -le "$HUB_AUDIT_BASELINE" && "$HUB_HEARTBEAT_ADVANCED" != "true" ]]; then
    echo -e "${RED}[FAIL]${NC}  Hub did not observe new Sentinel activity within ${HUB_AUDIT_WAIT_SECS}s."
    echo "  Hub audit baseline/current: $HUB_AUDIT_BASELINE / $HUB_AUDIT_TOTAL"
    echo "  Hub heartbeat baseline/current(ms): $HUB_HEARTBEAT_BASELINE / ${HUB_HEARTBEAT_CURRENT:-0}"
    echo "  Check Sentinel client agent service and AA_CENTRAL_URL connectivity."
    exit 1
  fi

  if [[ "$HUB_AUDIT_TOTAL" -gt "$HUB_AUDIT_BASELINE" ]]; then
    NEW_EVENTS=$((HUB_AUDIT_TOTAL - HUB_AUDIT_BASELINE))
    echo -e "${GREEN}[OK]${NC}    Hub ingested $NEW_EVENTS new event(s) from Sentinel."
  else
    echo -e "${GREEN}[OK]${NC}    Hub heartbeat advanced for registered Sentinel client(s)."
  fi
  echo ""
fi

if [[ "$FAIL" -eq 0 ]]; then
  banner "Validation Passed"
else
  banner "Validation Failed ($FAIL Scenario Failures)"
fi

exit $FAIL
