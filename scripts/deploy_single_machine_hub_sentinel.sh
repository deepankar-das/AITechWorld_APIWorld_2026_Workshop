#!/bin/bash
# Enforcer — Single-Machine Full Deploy + Validation
# Author: Deepankar Das
#
# Deploys Hub + Sentinel on one machine, then validates enforcement.
# All output is streamed to terminal and captured in a log file.
# Any failure aborts execution immediately.
#
# Usage:
#   sudo ./scripts/deploy_single_machine_hub_sentinel.sh
#   sudo ./scripts/deploy_single_machine_hub_sentinel.sh --seed-auth \
#     --seed-hub-admin-user admin --seed-hub-admin-password adm1 \
#     --seed-dev-user dev --seed-dev-password dev1
#   sudo ./scripts/deploy_single_machine_hub_sentinel.sh --skip-prepare --skip-build

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

SEED_AUTH=true
SKIP_PREPARE=false
SKIP_BUILD=false
SKIP_VALIDATION=false
RESET_SEEDED_SESSION_DATA=false
HUB_URL="${AA_CENTRAL_URL:-https://localhost:9200}"
REGISTRATION_WAIT_SECS="${AA_REGISTRATION_WAIT_SECS:-90}"
HUB_ADMIN_USER="${AA_SEED_ADMIN_USER:-admin}"
HUB_ADMIN_PASSWORD="${AA_SEED_ADMIN_PASSWORD:-adm1}"
DEV_USER_LABEL="${AA_SEED_DEV_USER:-${SUDO_USER:-$(logname 2>/dev/null || echo dev)}}"
DEV_PASSWORD="${AA_SEED_DEV_PASSWORD:-dev1}"
AUTHSEED_BINARY="$PROJECT_ROOT/go/bin/enforcer-authseed"
AUTH_KEY_FILE="${AA_AUTH_ENC_KEY_FILE:-/etc/enforcer/.auth_enc_key}"
DB_CREDENTIALS_FILE="${AA_DB_CREDENTIALS_FILE:-/etc/enforcer/.db_credentials}"
LOG_FILE=""
DEPLOY_START_TIME=$(date +%s)
INVOKING_USER="${SUDO_USER:-}"
CURRENT_STAGE="initialization"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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

log_step() { echo -e "\n${BLUE}[STEP]${NC}  $1"; }
log_ok()   { echo -e "  ${GREEN}[OK]${NC}    $1"; }
die()      { echo -e "\n${RED}${BOLD}[FATAL]${NC} $1"; exit 1; }

run_as_invoking_user() {
  if [[ -n "$INVOKING_USER" && "$INVOKING_USER" != "root" ]]; then
    sudo -u "$INVOKING_USER" "$@"
  else
    "$@"
  fi
}

fix_build_artifact_permissions() {
  if [[ -z "$INVOKING_USER" || "$INVOKING_USER" == "root" ]]; then
    return
  fi

  local user_group
  user_group="$(id -gn "$INVOKING_USER" 2>/dev/null || echo "staff")"
  local paths=(
    "$PROJECT_ROOT/console/.next"
    "$PROJECT_ROOT/console/out"
    "$PROJECT_ROOT/console/node_modules"
    "$PROJECT_ROOT/node_modules"
    "$PROJECT_ROOT/build"
    "$PROJECT_ROOT/dist"
    "$PROJECT_ROOT/go/bin"
  )

  for p in "${paths[@]}"; do
    if [[ -e "$p" ]]; then
      chown -R "$INVOKING_USER:$user_group" "$p" 2>/dev/null || true
    fi
  done

  # Also reclaim any stray root-owned generated files under console.
  find "$PROJECT_ROOT/console" \
    \( -path "$PROJECT_ROOT/console/.next" -o -path "$PROJECT_ROOT/console/out" \) \
    -prune -o -user root -print0 2>/dev/null \
    | xargs -0 chown "$INVOKING_USER:$user_group" 2>/dev/null || true
}

resolve_sentinel_console_url() {
  local candidates=(
    "http://localhost:6100/login/"
    "http://localhost:6100/"
    "http://localhost:9100/login/"
    "http://localhost:9100/"
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

wait_for_hub_registration() {
  local client_count=0
  local start_ts deadline_ts
  start_ts=$(date +%s)
  deadline_ts=$((start_ts + REGISTRATION_WAIT_SECS))

  while [[ "$(date +%s)" -le "$deadline_ts" ]]; do
    client_count=$(curl -sf -H "X-AA-Token: ${HUB_ADMIN_TOKEN:-adm1}" "http://localhost:9201/api/v1/clients" \
      | node -pe 'const fs=require("fs");const j=JSON.parse(fs.readFileSync(0,"utf8")); Number(j.count || (j.clients ? j.clients.length : 0) || 0)' \
      || echo 0)
    if [[ "$client_count" -ge 1 ]]; then
      echo "$client_count"
      return 0
    fi
    sleep 2
  done

  echo 0
  return 1
}

resolve_sentinel_token() {
  local token_file token
  for token_file in /etc/enforcer/.operator_token /etc/enforcer/.admin_token; do
    if [[ -r "$token_file" ]]; then
      token="$(tr -d '\r\n' < "$token_file" 2>/dev/null || true)"
      if [[ -n "$token" ]]; then
        echo "$token"
        return 0
      fi
    fi
  done
  return 1
}

sentinel_policy_rule_count() {
  local token="$1"
  curl -sS -H "Authorization: Bearer $token" "http://localhost:9100/v1/policy/bundle" 2>/dev/null \
    | node -e 'const fs=require("fs");try{const j=JSON.parse(fs.readFileSync(0,"utf8")); const n=(j&&j.rules&&Array.isArray(j.rules))?j.rules.length:0; process.stdout.write(String(n));}catch(_){process.stdout.write("0");}'
}

seed_auth_tokens_postgres_idempotent() {
  if [[ "$SEED_AUTH" != "true" ]]; then
    return 0
  fi

  local admin_token="$HUB_ADMIN_PASSWORD"
  local operator_token="$DEV_PASSWORD"
  local db_url=""

  if [[ -z "$admin_token" && -r /etc/enforcer/.admin_token ]]; then
    admin_token="$(tr -d '\r\n' < /etc/enforcer/.admin_token 2>/dev/null || true)"
  fi
  if [[ -z "$operator_token" && -r /etc/enforcer/.operator_token ]]; then
    operator_token="$(tr -d '\r\n' < /etc/enforcer/.operator_token 2>/dev/null || true)"
  fi
  if [[ -r "$DB_CREDENTIALS_FILE" ]]; then
    db_url="$(cat "$DB_CREDENTIALS_FILE")"
  fi

  if [[ -z "$db_url" ]]; then
    die "Cannot seed auth tokens into PostgreSQL: missing DB credentials at $DB_CREDENTIALS_FILE"
  fi
  if [[ ! -r "$AUTH_KEY_FILE" ]]; then
    die "Cannot seed auth tokens into PostgreSQL: missing auth key file $AUTH_KEY_FILE"
  fi
  if [[ -z "$admin_token" && -z "$operator_token" ]]; then
    die "Cannot seed auth tokens into PostgreSQL: no admin/operator token available"
  fi

  local cmd=()
  if [[ -x "$AUTHSEED_BINARY" ]]; then
    cmd=("$AUTHSEED_BINARY")
  elif command -v go >/dev/null 2>&1; then
    cmd=(go run "$PROJECT_ROOT/go/cmd/authseed")
  else
    die "Cannot seed auth tokens into PostgreSQL: no $AUTHSEED_BINARY and 'go' is unavailable"
  fi

  local args=()
  [[ -n "$admin_token" ]] && args+=(--admin-token "$admin_token")
  [[ -n "$operator_token" ]] && args+=(--operator-token "$operator_token")

  DATABASE_URL="$db_url" AA_AUTH_ENC_KEY_FILE="$AUTH_KEY_FILE" "${cmd[@]}" "${args[@]}"
  log_ok "Seeded auth credentials into PostgreSQL (idempotent upsert)"
}

reset_seeded_session_data_postgres() {
  if [[ "$RESET_SEEDED_SESSION_DATA" != "true" ]]; then
    return 0
  fi

  if ! command -v psql >/dev/null 2>&1; then
    die "Cannot reset seeded session data: psql is not installed"
  fi

  # Delete only demo/seeded session artifacts. Credentials and auth secrets are untouched.
  local reset_sql="
    WITH deleted AS (
      DELETE FROM audit_events
      WHERE session_id LIKE 'demo_%'
         OR event->>'request_id' LIKE 'demo_%'
         OR event->'actor'->>'user_id' = 'demo_dev'
      RETURNING 1
    )
    SELECT COUNT(*) FROM deleted;
  "

  local pg_run_user
  pg_run_user="${SUDO_USER:-$(logname 2>/dev/null || echo '')}"
  if [[ -z "$pg_run_user" || "$pg_run_user" == "root" ]]; then
    pg_run_user="postgres"
  fi

  local deleted_count=""
  if deleted_count="$(sudo -u "$pg_run_user" psql -d enforcer -tAc "$reset_sql" 2>/dev/null)"; then
    :
  elif [[ "$pg_run_user" != "postgres" ]] && deleted_count="$(sudo -u postgres psql -d enforcer -tAc "$reset_sql" 2>/dev/null)"; then
    pg_run_user="postgres"
  else
    die "Failed to reset seeded session data in PostgreSQL (tried users: ${pg_run_user} and postgres)"
  fi

  deleted_count="$(echo "$deleted_count" | tr -d '[:space:]')"
  [[ -z "$deleted_count" ]] && deleted_count="0"
  log_ok "Reset seeded session data in PostgreSQL (deleted $deleted_count demo audit event rows)"
}

repair_policy_stack() {
  local hub_repair_cmd sentinel_repair_cmd
  log_step "Repair policy bundle and restart Hub + Sentinel"
  cp "$PROJECT_ROOT/policies/default.yaml" /etc/enforcer/default.yaml
  chmod 644 /etc/enforcer/default.yaml
  chown root:wheel /etc/enforcer/default.yaml 2>/dev/null || chown root:root /etc/enforcer/default.yaml
  if [[ -f "$PROJECT_ROOT/policies/network-allowlist.yaml" ]]; then
    cp "$PROJECT_ROOT/policies/network-allowlist.yaml" /etc/enforcer/network-allowlist.yaml
    chmod 644 /etc/enforcer/network-allowlist.yaml
    chown root:wheel /etc/enforcer/network-allowlist.yaml 2>/dev/null || chown root:root /etc/enforcer/network-allowlist.yaml
  fi
  log_ok "Baseline policy restored under /etc/enforcer"

  hub_repair_cmd=("./scripts/deploy_hub.sh" "--restart")
  if [[ "$SEED_AUTH" == "true" ]]; then
    hub_repair_cmd+=(--seed-auth --seed-admin-user "$HUB_ADMIN_USER")
    if [[ -n "$HUB_ADMIN_PASSWORD" ]]; then
      hub_repair_cmd+=(--seed-admin-password "$HUB_ADMIN_PASSWORD")
    fi
  fi
  "${hub_repair_cmd[@]}"
  log_ok "Hub restart completed after policy repair"

  sentinel_repair_cmd=("./scripts/deploy_sentinel.sh" "--restart")
  if [[ "$SEED_AUTH" == "true" ]]; then
    sentinel_repair_cmd+=(--seed-auth --seed-dev-user "$DEV_USER_LABEL")
    if [[ -n "$DEV_PASSWORD" ]]; then
      sentinel_repair_cmd+=(--seed-dev-password "$DEV_PASSWORD")
    fi
  fi
  AA_CENTRAL_URL="$HUB_URL" "${sentinel_repair_cmd[@]}"
  log_ok "Sentinel restart completed after policy repair"
}

on_exit() {
  local code=$?
  if [[ $code -ne 0 && "$CURRENT_STAGE" != "completed" ]]; then
    banner "Deployment Failed"
    echo "  Stage:   $CURRENT_STAGE"
    echo "  Exit:    $code"
    echo "  Log:     $LOG_FILE"
    echo ""
    echo "  To retry this stage:"
    case "$CURRENT_STAGE" in
      "deploy hub")
        echo "    sudo ./scripts/deploy_hub.sh --seed-auth --seed-admin-user admin --seed-admin-password adm1"
        echo ""
        echo "  If PostgreSQL setup fails, try a clean start:"
        echo "    sudo ./scripts/uninstall.sh --drop-database"
        echo "    sudo ./scripts/deploy_single_machine_hub_sentinel.sh ..."
        ;;
      "deploy sentinel")
        echo "    sudo AA_CENTRAL_URL=https://localhost:9200 ./scripts/deploy_sentinel.sh --seed-auth --seed-dev-user dev --seed-dev-password dev1"
        ;;
      "validation")
        echo "    ./scripts/validate.sh"
        ;;
      *)
        echo "    Re-run: sudo ./scripts/deploy_single_machine_hub_sentinel.sh ..."
        ;;
    esac
    echo ""
  fi
}
trap on_exit EXIT

while [[ $# -gt 0 ]]; do
  case "$1" in
    --seed-auth) SEED_AUTH=true; shift ;;
    --seed-hub-admin-user) HUB_ADMIN_USER="$2"; shift 2 ;;
    --seed-hub-admin-password) HUB_ADMIN_PASSWORD="$2"; shift 2 ;;
    --seed-dev-user) DEV_USER_LABEL="$2"; shift 2 ;;
    --seed-dev-password) DEV_PASSWORD="$2"; shift 2 ;;
    --hub-url|--central-url) HUB_URL="$2"; shift 2 ;;
    --registration-wait-secs) REGISTRATION_WAIT_SECS="$2"; shift 2 ;;
    --skip-prepare) SKIP_PREPARE=true; shift ;;
    --skip-build) SKIP_BUILD=true; shift ;;
    --reset-seeded-session-data|--reset-demo-session-data) RESET_SEEDED_SESSION_DATA=true; shift ;;
    --skip-validation) SKIP_VALIDATION=true; shift ;;
    --log-file) LOG_FILE="$2"; shift 2 ;;
    -h|--help)
      cat <<'HELP'
Usage: sudo ./scripts/deploy_single_machine_hub_sentinel.sh [OPTIONS]

Options:
  --seed-auth                    Seed Hub + Sentinel credentials (default: enabled)
  --seed-hub-admin-user USER     Hub admin username label (default: admin)
  --seed-hub-admin-password PASS Hub admin access token (default: adm1)
  --seed-dev-user USER           Sentinel developer username label (default: dev)
  --seed-dev-password PASS       Sentinel developer access token (default: dev1)
  --hub-url URL                  Hub URL for Sentinel registration (default: https://localhost:9200)
  --registration-wait-secs N     Max wait for Hub client registration (default: 90)
  --skip-prepare                 Skip ./scripts/prepare.sh
  --skip-build                   Skip ./scripts/build.sh and Go build
  --reset-seeded-session-data    Delete seeded session audit rows from PostgreSQL
  --reset-demo-session-data      Alias for --reset-seeded-session-data
  --skip-validation              Skip post-deploy enforcement validation
  --log-file PATH                Override log file path
  -h, --help                     Show this help
HELP
      exit 0
      ;;
    *) die "Unknown option: $1" ;;
  esac
done

if [[ $EUID -ne 0 ]]; then
  die "This script must be run with sudo/root."
fi

if [[ ! -d "$PROJECT_ROOT/scripts" ]]; then
  die "Project root not detected: $PROJECT_ROOT"
fi

if [[ -z "$LOG_FILE" ]]; then
  mkdir -p "$PROJECT_ROOT/build/logs"
  LOG_FILE="$PROJECT_ROOT/build/logs/deploy-single-machine-$(date +%Y%m%d-%H%M%S).log"
else
  mkdir -p "$(dirname "$LOG_FILE")"
fi

# Stream to terminal with colors, write to log file with ANSI codes stripped.
exec > >(tee >(sed 's/\x1b\[[0-9;]*m//g' >> "$LOG_FILE")) 2>&1

banner "Single-Machine Deploy (Hub + Sentinel)"
echo ""
echo "  Log file:         $LOG_FILE"
echo "  Invoking user:    ${INVOKING_USER:-root}"
echo "  Hub URL:          $HUB_URL"
echo "  Seed auth:        $SEED_AUTH"
echo "  Hub admin seed:   $HUB_ADMIN_USER / $HUB_ADMIN_PASSWORD"
echo "  Sentinel seed:    $DEV_USER_LABEL / $DEV_PASSWORD"
echo "  Reg wait timeout: ${REGISTRATION_WAIT_SECS}s"
echo "  Skip prepare:     $SKIP_PREPARE"
echo "  Skip build:       $SKIP_BUILD"
echo "  Reset seeded data: $RESET_SEEDED_SESSION_DATA"
echo "  Validation:       $([[ \"$SKIP_VALIDATION\" == \"true\" ]] && echo skipped || echo enabled)"
echo ""

cd "$PROJECT_ROOT"

CURRENT_STAGE="build artifact permissions"
fix_build_artifact_permissions

if [[ "$SKIP_PREPARE" != "true" ]]; then
  CURRENT_STAGE="prepare"
  log_step "Prepare machine prerequisites"
  run_as_invoking_user ./scripts/prepare.sh
  log_ok "prepare.sh completed"
fi

if [[ "$SKIP_BUILD" != "true" ]]; then
  CURRENT_STAGE="build"
  log_step "Build TypeScript and Go components"
  # Clean generated Next.js artifacts before build to avoid stale permission issues.
  run_as_invoking_user /bin/bash -lc "rm -rf \"$PROJECT_ROOT/console/.next\" \"$PROJECT_ROOT/console/out\""
  run_as_invoking_user ./scripts/build.sh
  run_as_invoking_user /bin/bash -lc "cd \"$PROJECT_ROOT/go\" && make build"
  log_ok "build completed"
fi

CURRENT_STAGE="deploy hub"
log_step "Deploy Management Hub"
HUB_CMD=("./scripts/deploy_hub.sh")
if [[ "$SEED_AUTH" == "true" ]]; then
  HUB_CMD+=(--seed-auth --seed-admin-user "$HUB_ADMIN_USER")
  if [[ -n "$HUB_ADMIN_PASSWORD" ]]; then
    HUB_CMD+=(--seed-admin-password "$HUB_ADMIN_PASSWORD")
  fi
fi
"${HUB_CMD[@]}"
log_ok "Hub deployment completed"

CURRENT_STAGE="deploy sentinel"
log_step "Deploy Sentinel (Hub-connected)"
SENTINEL_CMD=("./scripts/deploy_sentinel.sh")
if [[ "$SEED_AUTH" == "true" ]]; then
  SENTINEL_CMD+=(--seed-auth --seed-dev-user "$DEV_USER_LABEL")
  if [[ -n "$DEV_PASSWORD" ]]; then
    SENTINEL_CMD+=(--seed-dev-password "$DEV_PASSWORD")
  fi
fi
AA_CENTRAL_URL="$HUB_URL" "${SENTINEL_CMD[@]}"
log_ok "Sentinel deployment completed"

CURRENT_STAGE="seed auth postgres"
log_step "Ensure auth seeding in PostgreSQL (idempotent)"
seed_auth_tokens_postgres_idempotent

CURRENT_STAGE="reset seeded session data"
log_step "Reset seeded session data (optional)"
reset_seeded_session_data_postgres

CURRENT_STAGE="verify registration"
log_step "Verify Hub client registration"
CLIENT_COUNT="$(wait_for_hub_registration || true)"
if [[ "${CLIENT_COUNT:-0}" -lt 1 ]]; then
  echo "  Hub URL configured for Sentinel: $HUB_URL"
  echo "  Current Hub clients payload:"
  curl -s -H "X-AA-Token: ${HUB_ADMIN_TOKEN:-adm1}" "http://localhost:9201/api/v1/clients" | python3 -m json.tool 2>/dev/null || true
  echo "  Sentinel daemon health:"
  curl -s "http://localhost:9100/v1/health" | python3 -m json.tool 2>/dev/null || true
  echo "  Last Sentinel log lines:"
  tail -n 40 /var/log/enforcer/sentinel.log 2>/dev/null || true
  echo "  Last Sentinel client log lines:"
  tail -n 40 /var/log/enforcer/sentinel-client.log 2>/dev/null || true
  die "Hub reports zero registered Sentinel clients after ${REGISTRATION_WAIT_SECS}s."
fi
log_ok "Hub sees $CLIENT_COUNT registered Sentinel client(s)"

CURRENT_STAGE="verify sentinel policy bundle"
log_step "Verify Sentinel runtime policy bundle"
SENTINEL_TOKEN="$(resolve_sentinel_token || true)"
if [[ -z "$SENTINEL_TOKEN" ]]; then
  die "Cannot resolve Sentinel API token from /etc/enforcer. Run with --seed-auth or configure token files."
fi
POLICY_RULES="$(sentinel_policy_rule_count "$SENTINEL_TOKEN")"
if [[ "${POLICY_RULES:-0}" -lt 1 ]]; then
  echo "  Sentinel currently reports $POLICY_RULES rules at /v1/policy/bundle."
  echo "  Attempting automatic policy repair and service restart..."
  repair_policy_stack
  CLIENT_COUNT="$(wait_for_hub_registration || true)"
  if [[ "${CLIENT_COUNT:-0}" -lt 1 ]]; then
    die "Hub has no registered Sentinel clients after policy repair."
  fi
  SENTINEL_TOKEN="$(resolve_sentinel_token || true)"
  POLICY_RULES="$(sentinel_policy_rule_count "$SENTINEL_TOKEN")"
fi
if [[ "${POLICY_RULES:-0}" -lt 1 ]]; then
  echo "  Hub policy snapshot:"
  curl -s -H "X-AA-Token: ${HUB_ADMIN_TOKEN:-adm1}" "http://localhost:9201/api/v1/policy" | python3 -m json.tool 2>/dev/null || true
  echo "  Sentinel health:"
  curl -s "http://localhost:9100/v1/health" | python3 -m json.tool 2>/dev/null || true
  echo "  Last Sentinel log lines:"
  tail -n 60 /var/log/enforcer/sentinel.log 2>/dev/null || true
  echo "  Last Sentinel client log lines:"
  tail -n 60 /var/log/enforcer/sentinel-client.log 2>/dev/null || true
  die "Sentinel policy bundle is still empty after auto-repair."
fi
log_ok "Sentinel runtime policy bundle loaded ($POLICY_RULES rules)"

if [[ "$SKIP_VALIDATION" != "true" ]]; then
  CURRENT_STAGE="validation"
  log_step "Run post-deploy enforcement validation (Hub + Sentinel)"
  ./scripts/validate.sh --hub-token "$HUB_ADMIN_PASSWORD"
  log_ok "validate.sh passed"
fi

CURRENT_STAGE="completed"

TOTAL_SECS=$(( $(date +%s) - DEPLOY_START_TIME ))
banner "Deployment Complete"
SENTINEL_CONSOLE_URL="$(resolve_sentinel_console_url || true)"
echo ""
echo "  Total time:        ${TOTAL_SECS}s"
echo "  Hub Console:       http://localhost:9201"
if [[ -n "$SENTINEL_CONSOLE_URL" ]]; then
  echo "  Sentinel Console:  $SENTINEL_CONSOLE_URL"
else
  echo "  Sentinel Console:  not reachable on expected ports (6100, 9100)"
fi
if [[ "$SEED_AUTH" == "true" ]]; then
  echo "  Hub login token:   user=$HUB_ADMIN_USER token=$HUB_ADMIN_PASSWORD (admin)"
  echo "  Sentinel token:    user=$DEV_USER_LABEL token=$DEV_PASSWORD (operator)"
fi
echo "  Validation:        $([[ \"$SKIP_VALIDATION\" == \"true\" ]] && echo skipped || echo passed)"
echo "  Log file:          $LOG_FILE"
echo ""
