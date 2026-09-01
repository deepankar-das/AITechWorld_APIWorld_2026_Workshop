#!/bin/bash
# Syntra — The operating layer for in-house legal.
# Use with permission from the author.
# Author: Deepankar Das
# Deploy Script — Seeds data and starts the local Next.js dev server
#
# Usage:
#   ./scripts/deploy.sh --seed-full --force            # Reset + seed 16 users, ~955 docs into Postgres
#   ./scripts/deploy.sh --migrate                      # Run Drizzle schema migration
#   ./scripts/deploy.sh --migrate --seed-full --force  # Migrate + full seed
#   ./scripts/deploy.sh --seed-reset --force           # Truncate all Postgres tables
#   ./scripts/deploy.sh --skip-health-check            # Start without health check
#   ./scripts/deploy.sh --port 3001                    # Start on a custom port

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

RUN_MIGRATE=false
SEED_FULL=false
SEED_USERS=false
SEED_USERS_DEMO_4=false
SEED_RESET=false
FORCE=false
SKIP_HEALTH_CHECK=false
PORT="${PORT:-3000}"

load_env_file() {
  if [[ -f "$PROJECT_ROOT/.env" ]]; then
    set -a
    source "$PROJECT_ROOT/.env" || true
    set +a
  fi
}

uses_postgres() {
  [[ "${DATABASE_URL:-}" =~ ^postgres(ql)?:// ]]
}

postgres_host() {
  if [[ "${DATABASE_URL:-}" =~ ^postgres(ql)?://([^@/]+@)?([^:/?]+) ]]; then
    echo "${BASH_REMATCH[3]}"
    return
  fi
  echo ""
}

requires_docker() {
  local explicit host
  explicit="$(echo "${SYNTRA_REQUIRE_DOCKER:-}" | tr '[:upper:]' '[:lower:]')"
  if [[ "$explicit" == "1" || "$explicit" == "true" || "$explicit" == "yes" ]]; then
    return 0
  fi

  if ! uses_postgres; then
    return 1
  fi

  host="$(postgres_host)"
  case "$host" in
    postgres|db|database|pg)
      return 0
      ;;
  esac
  return 1
}

start_docker_if_needed() {
  if ! requires_docker; then
    echo "Docker check skipped (not required for current config)."
    return
  fi

  if ! command -v docker >/dev/null 2>&1; then
    echo "Docker is required but not installed. Run ./scripts/prepare.sh --install first."
    exit 1
  fi

  if docker info >/dev/null 2>&1; then
    echo "Docker daemon is running."
    return
  fi

  if [[ "$(uname)" != "Darwin" ]]; then
    echo "Docker is required but not running. Start Docker and retry."
    exit 1
  fi

  echo "Docker is required and not running. Starting Docker Desktop..."
  open -a Docker >/dev/null 2>&1 || true

  for _ in $(seq 1 90); do
    if docker info >/dev/null 2>&1; then
      echo "Docker daemon started."
      return
    fi
    sleep 2
  done

  echo "Docker did not become ready in time. Start Docker Desktop manually and retry."
  exit 1
}

ensure_auth_env() {
  local auth_secret
  export PORT
  export NEXTAUTH_URL="${NEXTAUTH_URL:-http://localhost:${PORT}}"

  auth_secret="${NEXTAUTH_SECRET:-${JWT_SECRET:-syntra-local-nextauth-secret-2026-change-before-prod}}"
  export NEXTAUTH_SECRET="$auth_secret"
  export JWT_SECRET="${JWT_SECRET:-$auth_secret}"
}

load_env_file
PORT="${PORT:-3000}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --migrate) RUN_MIGRATE=true; shift ;;
    --seed-full) SEED_FULL=true; shift ;;
    --seed-users) SEED_USERS=true; shift ;;
    --seed-users-demo-4) SEED_USERS_DEMO_4=true; shift ;;
    --seed-reset) SEED_RESET=true; shift ;;
    --force) FORCE=true; shift ;;
    --skip-health-check) SKIP_HEALTH_CHECK=true; shift ;;
    --port) PORT="$2"; shift 2 ;;
    --help|-h)
      echo "Syntra Deploy — Seeds data, runs migrations, and starts the local Next.js server."
      echo ""
      echo "Usage: $0 [flags]"
      echo ""
      echo "Flags:"
      echo "  --migrate          Run Drizzle schema migration (drizzle-kit push)."
      echo "                     Creates or updates all PostgreSQL tables to match the schema."
      echo ""
      echo "  --seed-full        Seed 16 users + ~955 documents with realistic workflows,"
      echo "                     AI review results, vendor risk reports, and audit trail."
      echo "                     Requires --force. Truncates all tables first."
      echo ""
      echo "  --seed-users       Seed 16 users + routing rules + counsel profiles + playbook"
      echo "                     only. No documents, no workflows — empty queues."
      echo "                     Requires --force. Truncates all tables first."
      echo "                     Best for manual workflow testing."
      echo ""
      echo "  --seed-users-demo-4  Seed only 4 workflow-demo accounts:"
      echo "                       alex.rivera, james.patel, sarah.kim, victoria.chen"
      echo "                       plus routing rules/counsel profiles/playbook."
      echo "                       No documents, no workflows — empty queues."
      echo "                       Requires --force. Truncates all tables first."
      echo ""
      echo "  --seed-reset       Truncate ALL data in all PostgreSQL tables."
      echo "                     Requires --force to confirm. Destructive."
      echo ""
      echo "  --force            Confirm destructive operations (--seed-full, --seed-reset)."
      echo ""
      echo "  --port N           Start on a custom port (default: 3000)."
      echo ""
      echo "  --skip-health-check  Skip the startup health probe."
      echo ""
      echo "  -h, --help         Show this help."
      echo ""
      echo "Examples:"
      echo "  # Fresh setup on a new machine (most common):"
      echo "  $0 --migrate --seed-full --force"
      echo ""
      echo "  # Users only — empty queues for manual workflow testing:"
      echo "  $0 --migrate --seed-users --force"
      echo ""
      echo "  # 4-user workflow demo profile (Alex/James/Sarah/Victoria):"
      echo "  $0 --migrate --seed-users-demo-4 --force"
      echo ""
      echo "  # Re-seed after schema or seed data changes:"
      echo "  $0 --migrate --seed-full --force"
      echo ""
      echo "  # Deploy new code without touching data:"
      echo "  $0"
      echo ""
      echo "  # Schema migration only (no seeding, no data loss):"
      echo "  $0 --migrate"
      echo ""
      echo "  # Nuke all data and start clean:"
      echo "  $0 --seed-reset --force"
      echo ""
      echo "  # Start on a different port:"
      echo "  $0 --port 3001"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

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

ensure_auth_env

cd "$PROJECT_ROOT"

# ── Log to file + terminal ───────────────────────────────────────────────────
DEPLOY_DIR="$PROJECT_ROOT/build"
DEPLOY_LOG="$DEPLOY_DIR/deploy-$(date +%Y%m%d-%H%M%S).log"
mkdir -p "$DEPLOY_DIR"
touch "$DEPLOY_LOG"
ln -sfn "$(basename "$DEPLOY_LOG")" "$DEPLOY_DIR/deploy-latest.log" 2>/dev/null || true
exec > >(tee -a "$DEPLOY_LOG") 2>&1
echo "Deploy log: $DEPLOY_LOG"

banner "Local Server Startup"
echo "  Port: $PORT"
echo ""

start_docker_if_needed

if [[ ! -d node_modules ]]; then
  BANNER_SHOWN=true
  banner "Deploy Failed"
  echo "  Dependencies missing."
  echo ""
  echo "  Next steps:"
  echo "    1. ./scripts/prepare.sh --install — install dependencies"
  echo "    2. Re-run: ./scripts/deploy.sh"
  echo ""
  exit 1
fi

# ── Database Migration ──────────────────────────────────────────────────────
if [[ "$RUN_MIGRATE" == "true" ]]; then
  log_step "Running Drizzle schema migration (push)..."
  npx drizzle-kit push
  log_info "Database schema up to date"
fi

# ── Seed Reset (truncate all Postgres tables) ──────────────────────────────
if [[ "$SEED_RESET" == "true" ]]; then
  if [[ "$FORCE" != "true" ]]; then
    echo ""
    log_warn "Seed reset will DELETE ALL DATA in the database."
    echo "  Re-run with --force to confirm."
    exit 1
  fi
  log_step "Resetting all data..."
  npx tsx scripts/seed-reset.ts
  log_info "All data reset"
fi

# ── Users-Only Seed (16 users, rules, profiles, playbook — no documents) ───
if [[ "$SEED_USERS" == "true" ]]; then
  if [[ "$FORCE" != "true" ]]; then
    echo ""
    log_warn "Users-only seed will reset all data and create 16 users with empty queues."
    echo "  Re-run with --force to confirm."
    exit 1
  fi
  log_step "Running users-only seed (16 users, empty queues)..."
  npx tsx scripts/seed-users.ts --reset
  log_info "Users-only seed complete — all queues empty"
fi

# ── 4-user Demo Seed (Alex, James, Sarah, Victoria — no documents) ─────────
if [[ "$SEED_USERS_DEMO_4" == "true" ]]; then
  if [[ "$FORCE" != "true" ]]; then
    echo ""
    log_warn "Demo seed will reset all data and create only 4 walkthrough users with empty queues."
    echo "  Re-run with --force to confirm."
    exit 1
  fi
  log_step "Running 4-user demo seed (Alex/James/Sarah/Victoria, empty queues)..."
  npx tsx scripts/seed-users.ts --reset --workflow-demo-4
  log_info "4-user demo seed complete — empty queues"
fi

# ── Full Seed (16 users, ~955 documents, realistic workflows) ──────────────
if [[ "$SEED_FULL" == "true" ]]; then
  if [[ "$FORCE" != "true" ]]; then
    echo ""
    log_warn "Full seed will populate the database with ~955 documents."
    echo "  Re-run with --force to confirm."
    exit 1
  fi
  log_step "Running full data seed (~955 documents, 16 users)..."
  npx tsx scripts/seed-full.ts --reset
  log_info "Full seed complete"
fi

# ── Default Playbook Sync (always) ──────────────────────────────────────────
log_step "Syncing default playbook to database..."
npx tsx scripts/seed-playbook.ts
log_info "Default playbook synced"

log_step "Starting Next.js dev server..."

if lsof -ti tcp:"$PORT" >/dev/null 2>&1; then
  log_warn "Port $PORT in use — killing existing process..."
  lsof -ti tcp:"$PORT" | xargs kill -TERM >/dev/null 2>&1 || true
  sleep 1
fi

nohup npm run dev -- --port "$PORT" >/tmp/syntra-next.log 2>&1 &
APP_PID=$!

log_info "Started Syntra (PID $APP_PID) on http://localhost:$PORT"
log_info "Log: /tmp/syntra-next.log"

if [[ "$SKIP_HEALTH_CHECK" != "true" ]]; then
  log_step "Waiting for health check..."
  for i in $(seq 1 40); do
    if curl -fsS "http://localhost:$PORT/health" >/dev/null 2>&1; then
      log_info "Health check passed"
      log_deploy_summary

      BANNER_SHOWN=true
      banner "Deploy Complete"
      echo "  Syntra running on http://localhost:$PORT"
      echo ""
      echo "  Next steps:"
      echo "    1. Open http://localhost:$PORT in your browser"
      echo "    2. Log: tail -f /tmp/syntra-next.log"
      echo "    3. Stop: kill $APP_PID"
      echo ""
      exit 0
    fi
    sleep 1
  done

  BANNER_SHOWN=true
  banner "Deploy Failed"
  echo "  Health check timed out after 40s."
  echo ""
  echo "  Next steps:"
  echo "    1. Inspect the log: cat /tmp/syntra-next.log"
  echo "    2. Check port $PORT: lsof -i tcp:$PORT"
  echo "    3. Re-run: ./scripts/deploy.sh"
  echo ""
  exit 1
else
  log_deploy_summary

  BANNER_SHOWN=true
  banner "Deploy Complete (Health Check Skipped)"
  echo "  Syntra running on http://localhost:$PORT"
  echo ""
  echo "  Next steps:"
  echo "    1. Open http://localhost:$PORT in your browser"
  echo "    2. Log: tail -f /tmp/syntra-next.log"
  echo "    3. Stop: kill $APP_PID"
  echo ""
fi
