#!/bin/bash
# Enforcer — Management Hub Deployment
# Author: Deepankar Das
#
# Deploys the Enforcer Management Hub on the security team's server.
# Safe to re-run — skips steps that are already complete.
# Must be run as root (sudo).
#
# Usage:
#   sudo ./scripts/deploy_hub.sh                    # Full Hub deployment
#   sudo ./scripts/deploy_hub.sh --status           # Check Hub status
#   sudo ./scripts/deploy_hub.sh --stop             # Stop the Hub
#   sudo ./scripts/deploy_hub.sh --restart          # Stop + start
#   sudo ./scripts/deploy_hub.sh --seed-auth --seed-admin-user admin --seed-admin-password 'adm1'
#
# What this does (skips if already done):
#   1. Verifies Hub binary is built
#   2. Creates /etc/enforcer/ directories (if not exist)
#   3. Sets up PostgreSQL with restricted enforcer user (if not exist)
#   4. Generates mTLS certificates (if not exist)
#   5. Copies policy bundle (if not exist)
#   6. Starts Hub process (kills existing if running)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

HUB_DEPLOY_SUCCEEDED=false
on_exit() {
  local code=$?
  if [[ $code -ne 0 && "$HUB_DEPLOY_SUCCEEDED" != "true" ]]; then
    banner "Hub Server Deploy Failed"
  fi
}
trap on_exit EXIT

# Configuration
HUB_PORT="${HUB_PORT:-9200}"
CONSOLE_PORT="${HUB_CONSOLE_PORT:-9201}"
CERT_DIR="${CERT_DIR:-/etc/enforcer/certs}"
CONFIG_DIR="${AA_CONFIG_DIR:-/etc/enforcer}"
POLICY_DIR="${AA_POLICY_DIR:-$CONFIG_DIR}"
LOG_DIR="/var/log/enforcer"
LOG_FILE="$LOG_DIR/hub.log"
PID_FILE="/var/run/enforcer-hub.pid"
HUB_BINARY="$PROJECT_ROOT/go/bin/enforcer-central"
AUTHSEED_BINARY="$PROJECT_ROOT/go/bin/enforcer-authseed"

ACTION="deploy"
SEED_AUTH=false
SEED_ADMIN_USER="${AA_SEED_ADMIN_USER:-admin}"
SEED_ADMIN_PASSWORD="${AA_SEED_ADMIN_PASSWORD:-}"
POLICY_REPAIRED=false
HUB_RESTART_REQUIRED=false

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

# ── Argument parsing ────────────────────────────────────────────────────────

while [[ $# -gt 0 ]]; do
  case "$1" in
    --status)    ACTION="status"; shift ;;
    --stop)      ACTION="stop"; shift ;;
    --restart)   ACTION="restart"; shift ;;
    --uninstall) ACTION="uninstall"; shift ;;
    --seed-auth) SEED_AUTH=true; shift ;;
    --seed-admin-user)
      if [[ $# -lt 2 ]]; then log_error "Missing value for --seed-admin-user"; exit 1; fi
      SEED_ADMIN_USER="$2"; shift 2 ;;
    --seed-admin-password)
      if [[ $# -lt 2 ]]; then log_error "Missing value for --seed-admin-password"; exit 1; fi
      SEED_ADMIN_PASSWORD="$2"; shift 2 ;;
    -h|--help)
      echo "Usage: sudo ./scripts/deploy_hub.sh [--status|--stop|--restart|--uninstall|--seed-auth|--seed-admin-user USER|--seed-admin-password PASS|-h]"
      echo ""
      echo "Deploys the Enforcer Management Hub. Safe to re-run."
      echo "All steps check for existing state and skip if already complete."
      echo ""
      echo "Seeding options:"
      echo "  --seed-auth                 Seed Hub admin login values"
      echo "  --seed-admin-user USER      Seeded admin username label (default: admin)"
      echo "  --seed-admin-password PASS  Seeded admin access token"
      exit 0
      ;;
    *) log_error "Unknown option: $1"; exit 1 ;;
  esac
done

# ── Root check ──────────────────────────────────────────────────────────────

if [[ $EUID -ne 0 ]]; then
  echo -e "${RED}[ERROR]${NC} This script must be run as root (sudo)."
  echo "  The Hub manages security policies and audit data — it must run as a privileged service."
  exit 1
fi

# ── Helper: is Hub running? ─────────────────────────────────────────────────

hub_is_running() {
  [[ -f "$PID_FILE" ]] && kill -0 "$(cat "$PID_FILE" 2>/dev/null)" 2>/dev/null
}

hub_stop() {
  if hub_is_running; then
    kill "$(cat "$PID_FILE")" 2>/dev/null
    rm -f "$PID_FILE"
    sleep 1
    log_ok "Hub stopped"
  elif lsof -ti "tcp:$HUB_PORT" >/dev/null 2>&1; then
    lsof -ti "tcp:$HUB_PORT" | xargs kill -TERM 2>/dev/null
    sleep 1
    log_ok "Hub stopped (by port)"
  else
    log_skip "Hub is not running"
  fi
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

migrate_hub_schema() {
  local pg_run_user dev_user tmp_sql
  dev_user="${SUDO_USER:-$(logname 2>/dev/null || echo '')}"
  pg_run_user="$dev_user"
  if [[ -z "$pg_run_user" || "$pg_run_user" == "root" ]]; then
    pg_run_user="postgres"
  fi

  tmp_sql="$(mktemp /tmp/enforcer-hub-migrate.XXXXXX.sql)"
  cat > "$tmp_sql" <<'SQL'
CREATE TABLE IF NOT EXISTS hub_policy_revisions (
    id BIGSERIAL PRIMARY KEY,
    version TEXT NOT NULL,
    hash TEXT NOT NULL,
    bundle TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS hub_client_snapshots (
    id BIGSERIAL PRIMARY KEY,
    client_id TEXT NOT NULL,
    hostname TEXT NOT NULL,
    governed_users JSONB NOT NULL DEFAULT '[]'::jsonb,
    registered_at TIMESTAMPTZ NOT NULL,
    last_heartbeat TIMESTAMPTZ NOT NULL,
    policy_version TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'online',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS hub_enforcement_states (
    id BIGSERIAL PRIMARY KEY,
    enabled BOOLEAN NOT NULL,
    changed_by TEXT NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS auth_secrets (
    role TEXT PRIMARY KEY CHECK (role IN ('admin', 'reviewer', 'operator')),
    nonce_b64 TEXT NOT NULL,
    ciphertext_b64 TEXT NOT NULL,
    key_version INTEGER NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_hub_policy_created_at ON hub_policy_revisions (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_hub_policy_hash ON hub_policy_revisions (hash);
CREATE INDEX IF NOT EXISTS idx_hub_client_id_created ON hub_client_snapshots (client_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_hub_client_last_heartbeat ON hub_client_snapshots (last_heartbeat DESC);
CREATE INDEX IF NOT EXISTS idx_hub_enforcement_changed_at ON hub_enforcement_states (changed_at DESC);
CREATE INDEX IF NOT EXISTS idx_auth_secrets_updated_at ON auth_secrets (updated_at DESC);

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'enforcer') THEN
        EXECUTE 'GRANT INSERT ON hub_policy_revisions TO enforcer';
        EXECUTE 'GRANT SELECT ON hub_policy_revisions TO enforcer';
        EXECUTE 'GRANT INSERT ON hub_client_snapshots TO enforcer';
        EXECUTE 'GRANT SELECT ON hub_client_snapshots TO enforcer';
        EXECUTE 'GRANT INSERT ON hub_enforcement_states TO enforcer';
        EXECUTE 'GRANT SELECT ON hub_enforcement_states TO enforcer';
        EXECUTE 'GRANT INSERT, UPDATE ON auth_secrets TO enforcer';
        EXECUTE 'GRANT SELECT ON auth_secrets TO enforcer';
        EXECUTE 'GRANT USAGE, SELECT ON SEQUENCE hub_policy_revisions_id_seq TO enforcer';
        EXECUTE 'GRANT USAGE, SELECT ON SEQUENCE hub_client_snapshots_id_seq TO enforcer';
        EXECUTE 'GRANT USAGE, SELECT ON SEQUENCE hub_enforcement_states_id_seq TO enforcer';
    END IF;
END
$$;
SQL
  chmod 644 "$tmp_sql"
  chown "$pg_run_user" "$tmp_sql" 2>/dev/null || true

  export PGOPTIONS="-c enforcer.developer_user=${dev_user}"
  local migration_output
  if migration_output="$(sudo -u "$pg_run_user" psql -v ON_ERROR_STOP=1 -d enforcer -f "$tmp_sql" 2>&1)"; then
    rm -f "$tmp_sql"
    log_ok "Database schema migration ensured (Hub tables + grants)"
    return 0
  fi

  rm -f "$tmp_sql"
  log_error "Database schema migration failed (user: $pg_run_user)."
  echo "$migration_output" | sed 's/^/  [DB] /'
  log_error "Run manually: sudo -u $pg_run_user psql -d enforcer"
  return 1
}

# ── Status ──────────────────────────────────────────────────────────────────

if [[ "$ACTION" == "status" ]]; then
  echo -e "\n${CYAN}${BOLD}Enforcer — Management Hub Status${NC}\n"
  hub_is_running && log_ok "Hub process running (PID $(cat "$PID_FILE"))" || log_warn "Hub process not running"
  lsof -i "tcp:$HUB_PORT" >/dev/null 2>&1 && log_ok "Hub Server API on port $HUB_PORT" || log_warn "Hub Server API not on port $HUB_PORT"
  lsof -i "tcp:$CONSOLE_PORT" >/dev/null 2>&1 && log_ok "Hub Console on port $CONSOLE_PORT" || log_warn "Hub Console not on port $CONSOLE_PORT"
  pg_isready >/dev/null 2>&1 && log_ok "PostgreSQL running" || log_warn "PostgreSQL not running"
  [[ -f "$CERT_DIR/server.crt" ]] && log_ok "mTLS certs in $CERT_DIR" || log_warn "No mTLS certs"
  [[ -f "$CONFIG_DIR/.db_credentials" ]] && log_ok "DB credentials in $CONFIG_DIR" || log_warn "No DB credentials"
  exit 0
fi

if [[ "$ACTION" == "stop" ]]; then
  echo -e "\n${CYAN}${BOLD}Stopping Management Hub...${NC}"
  hub_stop
  exit 0
fi

if [[ "$ACTION" == "uninstall" ]]; then
  echo -e "\n${CYAN}${BOLD}Uninstalling Management Hub...${NC}"
  hub_stop
  rm -f "$PID_FILE"
  # Remove config but preserve database (audit data is evidence)
  rm -rf "$CERT_DIR"
  rm -f "$CONFIG_DIR/.db_credentials"
  rm -f "$CONFIG_DIR/sentinel.env"
  rm -f "$POLICY_DIR/default.yaml" "$POLICY_DIR/network-allowlist.yaml"
  rm -rf "$LOG_DIR"
  log_ok "Hub configuration removed"
  log_warn "PostgreSQL database 'enforcer' preserved (contains audit evidence)"
  log_warn "To drop it: sudo -u postgres dropdb enforcer"
  exit 0
fi

if [[ "$ACTION" == "restart" ]]; then
  echo -e "\n${CYAN}${BOLD}Restarting Management Hub...${NC}"
  hub_stop
  ACTION="deploy"
fi

# ── Deploy ────────────────────────────────────────────────────────────────

banner "Hub Server Deploy"

# Step 1: Binary
log_step "1/7" "Checking Hub binary"
if [[ -x "$HUB_BINARY" ]]; then
  log_ok "$HUB_BINARY"
else
  log_error "Hub binary not found. Build first: cd go && make build"
  exit 1
fi

# Step 2: Directories
log_step "2/7" "Creating directories"
for dir in "$CONFIG_DIR" "$CERT_DIR" "$LOG_DIR" /var/run; do
  if [[ -d "$dir" ]]; then
    log_skip "$dir exists"
  else
    mkdir -p "$dir"
    chmod 700 "$dir"
    log_ok "Created $dir"
  fi
done

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
log_step "3/7" "PostgreSQL setup"
if [[ -f "$CONFIG_DIR/.db_credentials" ]]; then
  log_skip "Database credentials exist ($CONFIG_DIR/.db_credentials)"
  migrate_hub_schema
else
  if [[ -x "$SCRIPT_DIR/setup-database.sh" ]]; then
    "$SCRIPT_DIR/setup-database.sh"
    migrate_hub_schema
  else
    log_warn "setup-database.sh not found — set up PostgreSQL manually"
  fi
fi

# Optional auth seeding for Hub login.
if [[ "$SEED_AUTH" == "true" ]]; then
  if [[ -z "$SEED_ADMIN_PASSWORD" ]]; then
    log_warn "Seed requested without --seed-admin-password; generating random token"
    SEED_ADMIN_PASSWORD="$(openssl rand -base64 24 | tr -d '\n')"
  fi
  echo -n "$SEED_ADMIN_PASSWORD" > "$CONFIG_DIR/.admin_token"
  echo -n "$SEED_ADMIN_USER" > "$CONFIG_DIR/.seed_admin_username"
  chmod 600 "$CONFIG_DIR/.admin_token" "$CONFIG_DIR/.seed_admin_username"
  chown root:wheel "$CONFIG_DIR/.admin_token" "$CONFIG_DIR/.seed_admin_username" 2>/dev/null || chown root:root "$CONFIG_DIR/.admin_token" "$CONFIG_DIR/.seed_admin_username"
  log_ok "Seeded Hub admin credentials in $CONFIG_DIR (.admin_token + .seed_admin_username)"
  seed_tokens_in_db "$SEED_ADMIN_PASSWORD" "" ""
fi

# Step 4: Certificates
log_step "4/7" "mTLS certificates"
if [[ -f "$CERT_DIR/server.crt" && -f "$CERT_DIR/ca.crt" ]]; then
  log_skip "Certificates exist in $CERT_DIR"
else
  if [[ -x "$SCRIPT_DIR/generate-certs.sh" ]]; then
    "$SCRIPT_DIR/generate-certs.sh" --output "$CERT_DIR"
    log_ok "Certificates generated"
  else
    log_warn "generate-certs.sh not found — place certs manually in $CERT_DIR"
  fi
fi

# Step 5: Policy bundle
log_step "5/7" "Policy bundle"
if [[ -f "$PROJECT_ROOT/policies/default.yaml" ]]; then
  if [[ -f "$POLICY_DIR/default.yaml" ]]; then
    if grep -q "policy_id:" "$POLICY_DIR/default.yaml" 2>/dev/null; then
      if cmp -s "$PROJECT_ROOT/policies/default.yaml" "$POLICY_DIR/default.yaml"; then
        log_skip "Policy exists and is up to date at $POLICY_DIR/default.yaml"
      else
        cp "$PROJECT_ROOT/policies/default.yaml" "$POLICY_DIR/default.yaml"
        POLICY_REPAIRED=true
        log_ok "Updated policy bundle at $POLICY_DIR/default.yaml (baseline changed)"
      fi
    else
      cp "$PROJECT_ROOT/policies/default.yaml" "$POLICY_DIR/default.yaml"
      POLICY_REPAIRED=true
      log_ok "Repaired invalid policy file at $POLICY_DIR/default.yaml (restored baseline)"
    fi
  else
    cp "$PROJECT_ROOT/policies/default.yaml" "$POLICY_DIR/default.yaml"
    POLICY_REPAIRED=true
    log_ok "Copied policy bundle to $POLICY_DIR"
  fi
  cp "$PROJECT_ROOT/policies/network-allowlist.yaml" "$POLICY_DIR/network-allowlist.yaml" 2>/dev/null || true
else
  log_warn "No policy bundle found — Hub starts with empty policy"
fi

# Step 6: Start Hub
log_step "6/7" "Starting Management Hub"
if hub_is_running; then
  if [[ "$POLICY_REPAIRED" == "true" ]]; then
    HUB_RESTART_REQUIRED=true
    log_warn "Hub running with previously invalid policy; restart required to load repaired policy"
  fi

  # Auth config is loaded at process startup. If we seed auth, restart so the
  # running Hub picks up new role tokens immediately.
  if [[ "$SEED_AUTH" == "true" ]]; then
    HUB_RESTART_REQUIRED=true
    log_warn "Hub auth seed applied; restart required to load updated auth tokens"
  fi

  # Older Hub binaries may not expose /api/v1/auth/me (404). Probe route shape
  # without credentials: expected status is 401 (or 403), never 404.
  auth_route_code="$(curl -s -o /dev/null -w "%{http_code}" --max-time 2 "http://localhost:$CONSOLE_PORT/api/v1/auth/me" || true)"
  if [[ "$auth_route_code" == "404" ]]; then
    HUB_RESTART_REQUIRED=true
    log_warn "Hub auth route probe returned 404; restart required to refresh runtime binary"
  fi

  if [[ "$HUB_RESTART_REQUIRED" == "true" ]]; then
    hub_stop
  else
    log_skip "Hub already running (PID $(cat \"$PID_FILE\"))"
  fi
fi

if ! hub_is_running; then
  # Kill anything on the ports
  lsof -ti "tcp:$HUB_PORT" 2>/dev/null | xargs kill -TERM 2>/dev/null || true
  lsof -ti "tcp:$CONSOLE_PORT" 2>/dev/null | xargs kill -TERM 2>/dev/null || true
  sleep 1

  # Load credentials
  if [[ -f "$CONFIG_DIR/.db_credentials" ]]; then
    export DATABASE_URL=$(cat "$CONFIG_DIR/.db_credentials")
  fi
  export CERT_DIR="$CERT_DIR"
  export AA_POLICY_DIR="$POLICY_DIR"
  export AA_AUTH_ENC_KEY_FILE="$AUTH_KEY_FILE"
  export AA_CONFIG_DIR="$CONFIG_DIR"

  nohup "$HUB_BINARY" >> "$LOG_FILE" 2>&1 &
  echo $! > "$PID_FILE"
  sleep 2

  if hub_is_running; then
    log_ok "Hub started (PID $(cat "$PID_FILE"))"
  else
    log_error "Hub failed to start. Check: tail -20 $LOG_FILE"
    if [[ -r "$LOG_FILE" ]]; then
      echo "  Last log lines:"
      tail -n 20 "$LOG_FILE" | sed 's/^/    /'
    fi
    exit 1
  fi
else
  :
fi

# Step 7: Health check
log_step "7/7" "Health check"
HEALTH_OK=false
for _ in $(seq 1 30); do
  if curl -sf "http://localhost:$CONSOLE_PORT/api/v1/health" >/dev/null 2>&1; then
    HEALTH_OK=true
    break
  fi
  sleep 1
done
if [[ "$HEALTH_OK" == "true" ]]; then
  log_ok "Hub is healthy"
  curl -s "http://localhost:$CONSOLE_PORT/api/v1/health" | python3 -m json.tool 2>/dev/null || true
else
  log_error "Health check failed after 30s. Check: curl http://localhost:$CONSOLE_PORT/api/v1/health"
  exit 1
fi

# Success banner + summary
HUB_DEPLOY_SUCCEEDED=true
banner "Hub Server Deploy Complete"
echo ""
echo "  Hub Server API: https://localhost:$HUB_PORT (mTLS for Sentinels)"
echo "  Hub Console:   http://localhost:$CONSOLE_PORT"
echo "  Logs:          $LOG_FILE"
echo "  Certs:         $CERT_DIR"
echo "  Policy:        $POLICY_DIR/default.yaml"
if [[ "$SEED_AUTH" == "true" ]]; then
  echo ""
  echo "  Hub login seed:"
  echo "    Username:    $SEED_ADMIN_USER"
  echo "    Access token:$SEED_ADMIN_PASSWORD"
fi
echo ""
echo "  Deploy Sentinels:"
echo "    sudo AA_CENTRAL_URL=https://<this-server>:$HUB_PORT ./scripts/deploy_sentinel.sh"
echo ""
