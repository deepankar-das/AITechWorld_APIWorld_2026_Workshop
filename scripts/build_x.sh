#!/bin/bash
# Syntra — The operating layer for in-house legal.
# Use with permission from the author.
# Author: Deepankar Das
# Build Script — TypeScript check + tests + Next.js production build
#
# Usage:
#   ./scripts/build.sh                # Full build (typecheck + test + build)
#   ./scripts/build.sh --skip-checks  # Skip typecheck and tests

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

SKIP_CHECKS=false
BUILD_DIR=""
BUILD_LOG=""

load_env_file() {
  if [[ -f "$PROJECT_ROOT/.env" ]]; then
    set -a
    source "$PROJECT_ROOT/.env" || true
    set +a
  fi
}

has_node_module() {
  local module_path="$1"
  node -e "require.resolve('$module_path')" >/dev/null 2>&1
}

ensure_check_dependencies() {
  if [[ "$SKIP_CHECKS" == "true" ]]; then
    return
  fi

  local missing=()
  has_node_module "typescript" || missing+=("typescript")
  has_node_module "vitest/package.json" || missing+=("vitest")

  if [[ ${#missing[@]} -eq 0 ]]; then
    return
  fi

  log_warn "Missing dev dependencies for checks: ${missing[*]}"
  log_step "Repairing dependencies (npm install --include=dev)..."
  npm install --include=dev --no-audit --no-fund

  missing=()
  has_node_module "typescript" || missing+=("typescript")
  has_node_module "vitest/package.json" || missing+=("vitest")

  if [[ ${#missing[@]} -gt 0 ]]; then
    BANNER_SHOWN=true
    banner "Build Failed"
    echo "  Required dev dependencies are still missing: ${missing[*]}"
    echo ""
    echo "  Next steps:"
    echo "    1. ./scripts/prepare.sh --install"
    echo "    2. Re-run: ./scripts/build.sh"
    echo ""
    exit 1
  fi

  log_info "Dev dependency repair complete"
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
  local app_port auth_secret
  app_port="${PORT:-3000}"
  export PORT="$app_port"
  export NEXTAUTH_URL="${NEXTAUTH_URL:-http://localhost:${app_port}}"

  auth_secret="${NEXTAUTH_SECRET:-${JWT_SECRET:-syntra-local-nextauth-secret-2026-change-before-prod}}"
  export NEXTAUTH_SECRET="$auth_secret"
  export JWT_SECRET="${JWT_SECRET:-$auth_secret}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-checks) SKIP_CHECKS=true; shift ;;
    --help|-h)
      echo "Usage: $0 [--skip-checks]"
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

BUILD_START_TIME=$(date +%s)
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

log_build_summary() {
  if [[ "$STEP_START_TIME" -gt 0 ]]; then
    _print_step_elapsed
  fi
  local now total mins secs
  now=$(date +%s)
  total=$((now - BUILD_START_TIME))
  mins=$((total / 60))
  secs=$((total % 60))
  echo ""
  echo -e "${GREEN}${BOLD}Build completed in ${mins}m ${secs}s${NC}"
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
    banner "Build Failed (exit code $code)"
    echo "  The error above shows what went wrong."
    echo "  This script is safe to re-run after fixing the issue."
    echo ""
    echo "  Next steps:"
    echo "    1. Fix the error shown above"
    echo "    2. Re-run: ./scripts/build.sh"
    echo ""
  fi
}
trap on_exit EXIT

cd "$PROJECT_ROOT"

BUILD_DIR="$PROJECT_ROOT/build"
BUILD_LOG="$BUILD_DIR/build-$(date +%Y%m%d-%H%M%S).log"
mkdir -p "$BUILD_DIR"
touch "$BUILD_LOG"
ln -sfn "$(basename "$BUILD_LOG")" "$BUILD_DIR/latest.log" 2>/dev/null || true
exec > >(tee -a "$BUILD_LOG") 2>&1
echo "Build log: $BUILD_LOG"

banner "Build"

load_env_file
ensure_auth_env
start_docker_if_needed

if [[ ! -f package.json ]]; then
  BANNER_SHOWN=true
  banner "Build Failed"
  echo "  package.json not found."
  echo ""
  echo "  Next steps:"
  echo "    1. Ensure you are in the Syntra project root"
  echo ""
  exit 1
fi

if [[ ! -d node_modules ]]; then
  BANNER_SHOWN=true
  banner "Build Failed"
  echo "  Dependencies missing."
  echo ""
  echo "  Next steps:"
  echo "    1. ./scripts/prepare.sh --install — install dependencies"
  echo "    2. Re-run: ./scripts/build.sh"
  echo ""
  exit 1
fi

ensure_check_dependencies

if [[ "$SKIP_CHECKS" != "true" ]]; then
  log_step "Running TypeScript check..."
  npm run typecheck
  log_info "TypeScript: zero errors"
fi

# Tests run after deployment against the real database, not during build.
# Use: ./scripts/deploy.sh --test

log_step "Building Next.js production bundle..."
NODE_ENV=production npm run build
log_info "Build complete"

log_step "Generating OpenAPI spec..."
npm run openapi:write
log_info "OpenAPI spec written"

log_build_summary

BANNER_SHOWN=true
banner "Build Complete"
echo "  Next steps:"
echo "    1. ./scripts/deploy.sh --migrate --seed-full --force"
echo "    2. ./scripts/deploy.sh --migrate   — start with migrations"
echo ""
