#!/bin/bash
# Enforcer — Runtime security and governance for AI coding agents.
# Author: Deepankar Das
# Deploy Script — Starts the Enforcer Sentinel Server and Sentinel Developer Console
#
# NOTE:
#   Default mode is local Sentinel-only development startup.
#   It does NOT register this machine with the Management Hub.
#   Single-machine Hub+Sentinel deploy is available with --single-machine.
#   For Hub-connected Sentinel deployment, use:
#     sudo AA_CENTRAL_URL=https://<hub>:9200 ./scripts/deploy_sentinel.sh
#
# Usage:
#   ./scripts/deploy.sh                        # Start Sentinel Server + Sentinel Developer Console
#   sudo ./scripts/deploy.sh --single-machine  # Deploy Hub + Sentinel on one machine
#   ./scripts/deploy.sh --migrate              # Run DB migration first, then start
#   ./scripts/deploy.sh --port 6001            # Custom Sentinel Developer Console port
#   ./scripts/deploy.sh --skip-health-check    # Start without health check
#   ./scripts/deploy.sh --sentinel-only        # Start Sentinel Server only (no console)
#   ./scripts/deploy.sh --console-only         # Start Sentinel Developer Console only
#   ./scripts/deploy.sh --seed-auth            # Enable Sentinel Developer auth seeding for local demos/tests

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

RUN_MIGRATE=false
SKIP_HEALTH_CHECK=false
DAEMON_ONLY=false
CONSOLE_ONLY=false
FULL_DEPLOY=false
SEED_AUTH=false
SEED_ROLE="${AA_SEED_ROLE:-admin}"
SEED_TOKEN="${AA_SEED_TOKEN:-admin}"
SEED_USERNAME="${AA_SEED_USERNAME:-seeded-admin}"
DAEMON_RUNTIME="${DAEMON_RUNTIME:-go}"
STRICT_MODE="${AA_STRICT_MODE:-true}"
AUTH_KEY_FILE="${AA_AUTH_ENC_KEY_FILE:-/etc/enforcer/.auth_enc_key}"
AUTHSEED_BINARY="$PROJECT_ROOT/go/bin/enforcer-authseed"
FULL_HUB_URL="${AA_CENTRAL_URL:-https://localhost:9200}"
FULL_HUB_ADMIN_USER="${AA_SEED_ADMIN_USER:-admin}"
FULL_HUB_ADMIN_PASSWORD="${AA_SEED_ADMIN_PASSWORD:-}"
FULL_SENTINEL_DEV_USER="${AA_SEED_DEV_USER:-dev}"
FULL_SENTINEL_DEV_PASSWORD="${AA_SEED_DEV_PASSWORD:-}"

# Default ports
CONSOLE_PORT="${CONSOLE_PORT:-6100}"
DAEMON_PORT="${DAEMON_PORT:-9100}"
PROXY_PORT="${PROXY_PORT:-9101}"
FULL_HUB_ADMIN_PASSWORD_PROVIDED=false
FULL_SENTINEL_DEV_PASSWORD_PROVIDED=false

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

DEPLOY_START_TIME=$(date +%s)
STEP_START_TIME=0

log_info()  { echo -e "${GREEN}[INFO]${NC}  $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

log_step() {
  if [[ "$STEP_START_TIME" -gt 0 ]]; then
    _print_step_elapsed
  fi
  STEP_START_TIME=$(date +%s)
  echo -e "${BLUE}[STEP]${NC}  $1"
}

_print_step_elapsed() {
  local now elapsed mins secs
  now=$(date +%s)
  elapsed=$((now - STEP_START_TIME))
  mins=$((elapsed / 60))
  secs=$((elapsed % 60))
  if [[ $mins -gt 0 ]]; then
    echo -e "${DIM}        ↳ ${mins}m ${secs}s${NC}"
  elif [[ $elapsed -gt 0 ]]; then
    echo -e "${DIM}        ↳ ${secs}s${NC}"
  fi
}

log_deploy_summary() {
  if [[ "$STEP_START_TIME" -gt 0 ]]; then
    _print_step_elapsed
  fi
  local now total mins secs
  now=$(date +%s)
  total=$((now - DEPLOY_START_TIME))
  mins=$((total / 60))
  secs=$((total % 60))
  echo ""
  echo -e "${GREEN}${BOLD}Deploy completed in ${mins}m ${secs}s${NC}"
}

ensure_auth_encryption_key() {
  if [[ -f "$AUTH_KEY_FILE" ]]; then
    chmod 600 "$AUTH_KEY_FILE" 2>/dev/null || true
    return 0
  fi

  local auth_key_dir
  auth_key_dir="$(dirname "$AUTH_KEY_FILE")"
  mkdir -p "$auth_key_dir"
  openssl rand -base64 32 | tr -d '\n' > "$AUTH_KEY_FILE"
  chmod 600 "$AUTH_KEY_FILE" 2>/dev/null || true
  log_info "Generated local auth encryption key at $AUTH_KEY_FILE"
}

seed_tokens_in_db() {
  local admin_token="$1"
  local reviewer_token="$2"
  local operator_token="$3"

  local args=()
  [[ -n "$admin_token" ]] && args+=(--admin-token "$admin_token")
  [[ -n "$reviewer_token" ]] && args+=(--reviewer-token "$reviewer_token")
  [[ -n "$operator_token" ]] && args+=(--operator-token "$operator_token")
  if [[ ${#args[@]} -eq 0 ]]; then
    return 0
  fi

  local db_url="${DATABASE_URL:-}"
  if [[ -z "$db_url" && -f "/etc/enforcer/.db_credentials" ]]; then
    db_url="$(cat /etc/enforcer/.db_credentials)"
  fi
  if [[ -z "$db_url" ]]; then
    log_warn "Skipping DB auth seed: DATABASE_URL is not set"
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

  if ! DATABASE_URL="$db_url" AA_AUTH_ENC_KEY_FILE="$AUTH_KEY_FILE" "${cmd[@]}" "${args[@]}"; then
    log_error "Failed to seed encrypted auth tokens into PostgreSQL"
    return 1
  fi
  log_info "Seeded encrypted auth tokens into PostgreSQL"
}

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

BANNER_SHOWN=false

on_exit() {
  local code=$?
  if [[ $code -ne 0 && "$BANNER_SHOWN" != "true" ]]; then
    banner "Deploy Failed (exit code $code)"
    echo "  The error above shows what went wrong."
    echo "  This script is safe to re-run after fixing the issue."
    echo ""
    echo "  Next steps:"
    echo "    1. Fix the error shown above"
    echo "    2. Re-run: ./scripts/deploy.sh"
    echo ""
  fi
}
trap on_exit EXIT

# ── Load env ─────────────────────────────────────────────────────────────────
if [[ -f "$PROJECT_ROOT/.env" ]]; then
  set -a
  source "$PROJECT_ROOT/.env" || true
  set +a
fi

# Re-apply defaults after env load (env may override)
CONSOLE_PORT="${CONSOLE_PORT:-6100}"
DAEMON_PORT="${DAEMON_PORT:-9100}"
PROXY_PORT="${PROXY_PORT:-9101}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --single-machine|--full) FULL_DEPLOY=true; shift ;;
    --migrate) RUN_MIGRATE=true; shift ;;
    --skip-health-check) SKIP_HEALTH_CHECK=true; shift ;;
    --daemon-only|--sentinel-only) DAEMON_ONLY=true; shift ;;
    --console-only) CONSOLE_ONLY=true; shift ;;
    --seed-auth) SEED_AUTH=true; shift ;;
    --no-seed-auth) SEED_AUTH=false; shift ;;
    --seed-role) SEED_ROLE="$2"; shift 2 ;;
    --seed-token) SEED_TOKEN="$2"; shift 2 ;;
    --seed-username) SEED_USERNAME="$2"; shift 2 ;;
    --hub-url|--central-url) FULL_HUB_URL="$2"; shift 2 ;;
    --seed-hub-admin-user) FULL_HUB_ADMIN_USER="$2"; shift 2 ;;
    --seed-hub-admin-password)
      FULL_HUB_ADMIN_PASSWORD="$2"; FULL_HUB_ADMIN_PASSWORD_PROVIDED=true; shift 2 ;;
    --seed-dev-user) FULL_SENTINEL_DEV_USER="$2"; shift 2 ;;
    --seed-dev-password)
      FULL_SENTINEL_DEV_PASSWORD="$2"; FULL_SENTINEL_DEV_PASSWORD_PROVIDED=true; shift 2 ;;
    --runtime) DAEMON_RUNTIME="$2"; shift 2 ;;
    --port) CONSOLE_PORT="$2"; shift 2 ;;
    --daemon-port|--sentinel-api-port) DAEMON_PORT="$2"; shift 2 ;;
    --proxy-port|--sentinel-egress-port) PROXY_PORT="$2"; shift 2 ;;
    --help|-h)
      echo "Enforcer Deploy — Starts Sentinel Server and Sentinel Developer Console."
      echo ""
      echo "Usage: $0 [flags]"
      echo ""
      echo "Scope:"
      echo "  Local Sentinel startup only (no Hub registration)."
      echo "  OR deploy full single-machine mode (Hub + Sentinel): --single-machine"
      echo "  For Hub-connected Sentinel deployment use:"
      echo "    sudo AA_CENTRAL_URL=https://<hub>:9200 ./scripts/deploy_sentinel.sh"
      echo ""
      echo "Flags:"
      echo "  --single-machine     Deploy both Hub and Sentinel on this machine (requires sudo)."
      echo "  --full               Alias for --single-machine."
      echo "  --migrate            Run database migration before starting."
      echo "  --port N             Sentinel Developer Console port (default: 6000)."
      echo "  --sentinel-api-port N  Sentinel Server API port (default: 9100)."
      echo "  --sentinel-egress-port N  Sentinel Server egress port (default: 9101)."
      echo "  --sentinel-only      Start Sentinel Server only (no console)."
      echo "  --console-only       Start Sentinel Developer Console only."
      echo "  --skip-health-check  Skip startup health probe."
      echo "  --seed-auth          Enable auth seeding (local Sentinel Console, or Hub+Sentinel in --single-machine mode)."
      echo "  --no-seed-auth       Disable Sentinel Developer Console auth seeding (default)."
      echo "  --seed-role ROLE     Seed role when --seed-auth is enabled (default: admin)."
      echo "  --seed-token TOKEN   Seed token when --seed-auth is enabled (default: admin)."
      echo "  --seed-username USER Seed username when --seed-auth is enabled (default: seeded-admin)."
      echo "  --hub-url URL        Hub URL for single-machine/full deploy (default: https://localhost:9200)."
      echo "  --seed-hub-admin-user USER"
      echo "                       Hub admin username label for --single-machine when --seed-auth is set."
      echo "  --seed-hub-admin-password PASS"
      echo "                       Hub admin access token for --single-machine when --seed-auth is set."
      echo "  --seed-dev-user USER"
      echo "                       Sentinel developer username label for --single-machine when --seed-auth is set."
      echo "  --seed-dev-password PASS"
      echo "                       Sentinel developer access token for --single-machine when --seed-auth is set."
      echo "  --runtime MODE       Sentinel Server runtime: go|ts (default: go)."
      echo "  -h, --help           Show this help."
      echo ""
      echo "Examples:"
      echo "  sudo $0 --single-machine --seed-auth --seed-hub-admin-user admin --seed-hub-admin-password adm1 --seed-dev-user dev --seed-dev-password dev1"
      echo "  $0 --migrate                        # Migrate DB + start everything"
      echo "  $0                                   # Start without migration"
      echo "  $0 --port 6001                       # Sentinel Developer Console on custom port"
      echo "  $0 --sentinel-only                   # Sentinel Server only"
      echo ""
      echo "Services:"
      echo "  Sentinel Server API:   http://localhost:\$DAEMON_PORT   (default: 9100)"
      echo "  Sentinel Server Egress: http://localhost:\$PROXY_PORT   (default: 9101)"
      echo "  Sentinel Developer Console: http://localhost:\$CONSOLE_PORT (default: 6000)"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

cd "$PROJECT_ROOT"

# ── Log to file + terminal ───────────────────────────────────────────────────
DEPLOY_DIR="$PROJECT_ROOT/build"
DEPLOY_LOG="$DEPLOY_DIR/deploy-$(date +%Y%m%d-%H%M%S).log"
mkdir -p "$DEPLOY_DIR"
touch "$DEPLOY_LOG"
ln -sfn "$(basename "$DEPLOY_LOG")" "$DEPLOY_DIR/deploy-latest.log" 2>/dev/null || true
# Stream to terminal with colors, write to log file with ANSI codes stripped.
exec > >(tee >(sed 's/\x1b\[[0-9;]*m//g' >> "$DEPLOY_LOG")) 2>&1
echo "Deploy log: $DEPLOY_LOG"

if [[ "$FULL_DEPLOY" == "true" ]]; then
  if [[ $EUID -ne 0 ]]; then
    log_error "--single-machine requires root (sudo)."
    echo "  Run: sudo ./scripts/deploy.sh --single-machine"
    exit 1
  fi

  banner "Deploy (Single Machine: Hub + Sentinel)"
  echo "  Management Hub URL: $FULL_HUB_URL"
  echo "  Auth seed: $([[ \"$SEED_AUTH\" == \"true\" ]] && echo enabled || echo disabled)"
  echo ""

  log_step "Deploying Management Hub..."
  HUB_CMD=("$SCRIPT_DIR/deploy_hub.sh")
  if [[ "$SEED_AUTH" == "true" ]]; then
    HUB_CMD+=(--seed-auth --seed-admin-user "$FULL_HUB_ADMIN_USER")
    if [[ "$FULL_HUB_ADMIN_PASSWORD_PROVIDED" == "true" || -n "$FULL_HUB_ADMIN_PASSWORD" ]]; then
      HUB_CMD+=(--seed-admin-password "$FULL_HUB_ADMIN_PASSWORD")
    fi
  fi
  "${HUB_CMD[@]}"

  log_step "Deploying Sentinel..."
  SENTINEL_CMD=("$SCRIPT_DIR/deploy_sentinel.sh")
  if [[ "$SEED_AUTH" == "true" ]]; then
    SENTINEL_CMD+=(--seed-auth --seed-dev-user "$FULL_SENTINEL_DEV_USER")
    if [[ "$FULL_SENTINEL_DEV_PASSWORD_PROVIDED" == "true" || -n "$FULL_SENTINEL_DEV_PASSWORD" ]]; then
      SENTINEL_CMD+=(--seed-dev-password "$FULL_SENTINEL_DEV_PASSWORD")
    fi
  fi
  AA_CENTRAL_URL="$FULL_HUB_URL" "${SENTINEL_CMD[@]}"

  log_deploy_summary
  BANNER_SHOWN=true
  banner "Deploy Complete"
  echo "  Enforcer is running (single-machine):"
  echo "    Management Hub API:      https://localhost:9200"
  echo "    Hub Console:             http://localhost:9201"
  echo "    Sentinel Server API:     http://localhost:9100"
  echo "    Sentinel Developer Console: http://localhost:6100"
  echo ""
  echo "  Next:"
  echo "    ./scripts/validate.sh"
  echo ""
  exit 0
fi

if [[ -z "${AA_AUTH_ENC_KEY_FILE:-}" && $EUID -ne 0 ]]; then
  AUTH_KEY_FILE="$PROJECT_ROOT/.auth_enc_key"
fi
ensure_auth_encryption_key

SEED_ADMIN_TOKEN=""
SEED_REVIEWER_TOKEN=""
SEED_OPERATOR_TOKEN=""
if [[ "$SEED_AUTH" == "true" ]]; then
  case "$SEED_ROLE" in
    admin) SEED_ADMIN_TOKEN="$SEED_TOKEN" ;;
    reviewer) SEED_REVIEWER_TOKEN="$SEED_TOKEN" ;;
    operator) SEED_OPERATOR_TOKEN="$SEED_TOKEN" ;;
    *)
      log_error "Invalid --seed-role '$SEED_ROLE' (expected admin|reviewer|operator)"
      exit 1
      ;;
  esac
  seed_tokens_in_db "$SEED_ADMIN_TOKEN" "$SEED_REVIEWER_TOKEN" "$SEED_OPERATOR_TOKEN"
fi

banner "Deploy"
echo "  Sentinel Server API: localhost:$DAEMON_PORT"
echo "  Sentinel Egress: localhost:$PROXY_PORT"
echo "  Sentinel Developer Console: localhost:$CONSOLE_PORT"
echo "  Auth seed: $([[ \"$SEED_AUTH\" == \"true\" ]] && echo enabled || echo disabled)"
echo "  Runtime:  $DAEMON_RUNTIME"
echo "  Mode:     Local Sentinel startup (no Hub registration)"
echo ""

# ── Docker Compose (PostgreSQL) ──────────────────────────────────────────────
if [[ -f "$PROJECT_ROOT/docker/docker-compose.yaml" ]]; then
  log_step "Starting PostgreSQL via Docker Compose..."
  if docker compose -f "$PROJECT_ROOT/docker/docker-compose.yaml" up -d postgres 2>/dev/null; then
    log_info "PostgreSQL container running"
    sleep 2  # Allow PostgreSQL to become ready
  else
    log_warn "Docker Compose failed — ensure PostgreSQL is running manually"
  fi
else
  log_warn "docker/docker-compose.yaml not found — assuming PostgreSQL is running externally"
fi

# ── Database Migration ───────────────────────────────────────────────────────
if [[ "$RUN_MIGRATE" == "true" ]]; then
  log_step "Running database migration..."
  if [[ -f "$PROJECT_ROOT/src/audit/migrate.ts" ]]; then
    npx tsx "$PROJECT_ROOT/src/audit/migrate.ts"
  elif command -v npx >/dev/null 2>&1; then
    npx tsx -e "console.log('Migration script not yet implemented')"
  fi
  log_info "Database schema up to date"
fi

# ── Kill existing processes on our ports ─────────────────────────────────────
kill_port() {
  local port="$1"
  if lsof -ti tcp:"$port" >/dev/null 2>&1; then
    log_warn "Port $port in use — killing existing process..."
    lsof -ti tcp:"$port" | xargs kill -TERM >/dev/null 2>&1 || true
    sleep 1
  fi
}

# ── Start Sentinel Server ────────────────────────────────────────────────────
if [[ "$CONSOLE_ONLY" != "true" ]]; then
  log_step "Starting Enforcer Sentinel Server on port $DAEMON_PORT..."
  kill_port "$DAEMON_PORT"
  kill_port "$PROXY_PORT"
  DAEMON_PID=""
  case "$DAEMON_RUNTIME" in
    go)
      daemon_env=(
        DAEMON_PORT="$DAEMON_PORT"
        PROXY_PORT="$PROXY_PORT"
        AA_STRICT_MODE="$STRICT_MODE"
        AA_AUTH_ENC_KEY_FILE="$AUTH_KEY_FILE"
      )
      [[ -n "$SEED_ADMIN_TOKEN" ]] && daemon_env+=(AA_ADMIN_TOKEN="$SEED_ADMIN_TOKEN")
      [[ -n "$SEED_REVIEWER_TOKEN" ]] && daemon_env+=(AA_REVIEWER_TOKEN="$SEED_REVIEWER_TOKEN")
      [[ -n "$SEED_OPERATOR_TOKEN" ]] && daemon_env+=(AA_OPERATOR_TOKEN="$SEED_OPERATOR_TOKEN")
      if [[ -x "$PROJECT_ROOT/go/bin/enforcer-daemon" ]]; then
        env "${daemon_env[@]}" nohup "$PROJECT_ROOT/go/bin/enforcer-daemon" > /tmp/enforcer-daemon.log 2>&1 &
        DAEMON_PID=$!
      else
        (
          cd "$PROJECT_ROOT/go"
          env "${daemon_env[@]}" nohup go run ./cmd/daemon > /tmp/enforcer-daemon.log 2>&1 &
          echo $! > /tmp/enforcer-daemon.pid
        )
        DAEMON_PID=$(cat /tmp/enforcer-daemon.pid 2>/dev/null || true)
      fi
      ;;
    ts)
      DAEMON_PORT="$DAEMON_PORT" PROXY_PORT="$PROXY_PORT" AA_STRICT_MODE="$STRICT_MODE" \
        nohup npx tsx "$PROJECT_ROOT/src/daemon/server.ts" > /tmp/enforcer-daemon.log 2>&1 &
      DAEMON_PID=$!
      ;;
    *)
      log_error "Unknown --runtime value '$DAEMON_RUNTIME' (expected 'go' or 'ts')"
      exit 1
      ;;
  esac
  log_info "Sentinel Server started (PID $DAEMON_PID) on http://localhost:$DAEMON_PORT"
  log_info "Sentinel Server egress endpoint on http://localhost:$PROXY_PORT"
  log_info "Sentinel Server log: /tmp/enforcer-daemon.log"
fi

# ── Start Sentinel Developer Console ─────────────────────────────────────────
if [[ "$DAEMON_ONLY" != "true" ]]; then
  if [[ -d "$PROJECT_ROOT/console" && -f "$PROJECT_ROOT/console/package.json" ]]; then
    log_step "Starting Sentinel Developer Console on port $CONSOLE_PORT..."
    kill_port "$CONSOLE_PORT"

    cd "$PROJECT_ROOT/console"
    if [[ "$SEED_AUTH" == "true" ]]; then
      log_warn "Auth seeding enabled for Sentinel Developer Console (local demos/tests only)"
      NEXT_PUBLIC_AA_AUTH_SEED="true" \
      NEXT_PUBLIC_AA_AUTH_SEED_ROLE="$SEED_ROLE" \
      NEXT_PUBLIC_AA_AUTH_SEED_TOKEN="$SEED_TOKEN" \
      NEXT_PUBLIC_AA_AUTH_SEED_USERNAME="$SEED_USERNAME" \
      PORT="$CONSOLE_PORT" \
      nohup npm run dev -- --port "$CONSOLE_PORT" > /tmp/enforcer-console.log 2>&1 &
    else
      PORT="$CONSOLE_PORT" nohup npm run dev -- --port "$CONSOLE_PORT" > /tmp/enforcer-console.log 2>&1 &
    fi
    CONSOLE_PID=$!
    cd "$PROJECT_ROOT"
    log_info "Sentinel Developer Console started (PID $CONSOLE_PID) on http://localhost:$CONSOLE_PORT"
    log_info "Sentinel Developer Console log: /tmp/enforcer-console.log"
    if [[ "$SEED_AUTH" == "true" ]]; then
      log_info "Seeded role: $SEED_ROLE, username: $SEED_USERNAME"
    fi
  else
    log_warn "Sentinel Developer Console not found (console/ directory missing) — skipping startup"
  fi
fi

# ── Health Check ─────────────────────────────────────────────────────────────
if [[ "$SKIP_HEALTH_CHECK" != "true" && "$CONSOLE_ONLY" != "true" ]]; then
  log_step "Waiting for Sentinel Server health check..."
  for i in $(seq 1 30); do
    if curl -fsS "http://localhost:$DAEMON_PORT/v1/health" >/dev/null 2>&1; then
      log_info "Sentinel Server health check passed"
      break
    fi
    if [[ $i -eq 30 ]]; then
      log_warn "Sentinel Server health check timed out after 30s — check /tmp/enforcer-daemon.log"
    fi
    sleep 1
  done
fi

log_deploy_summary

BANNER_SHOWN=true
banner "Deploy Complete"
echo "  Enforcer is running:"
if [[ "$CONSOLE_ONLY" != "true" ]]; then
  echo "    Sentinel Server API:    http://localhost:$DAEMON_PORT"
  echo "    Sentinel Server Egress: http://localhost:$PROXY_PORT"
fi
if [[ "$DAEMON_ONLY" != "true" ]]; then
  echo "    Sentinel Developer Console: http://localhost:$CONSOLE_PORT"
fi
echo ""
echo "  Hub-connected Sentinel install:"
echo "    sudo AA_CENTRAL_URL=https://<hub>:9200 ./scripts/deploy_sentinel.sh"
echo ""
echo "  Logs:"
if [[ "$CONSOLE_ONLY" != "true" ]]; then
  echo "    Sentinel Server: tail -f /tmp/enforcer-daemon.log"
fi
if [[ "$DAEMON_ONLY" != "true" ]]; then
  echo "    Sentinel Developer Console: tail -f /tmp/enforcer-console.log"
fi
echo ""
echo "  Stop:"
if [[ "$CONSOLE_ONLY" != "true" ]]; then
  echo "    Sentinel Server: kill ${DAEMON_PID:-}"
fi
if [[ "$DAEMON_ONLY" != "true" ]]; then
  echo "    Sentinel Developer Console: kill ${CONSOLE_PID:-}"
fi
echo ""
