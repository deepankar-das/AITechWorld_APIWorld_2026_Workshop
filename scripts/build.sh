#!/bin/bash
# Enforcer — Runtime security and governance for AI coding agents.
# Author: Deepankar Das
# Build Script — TypeScript check + tests + production build
#
# Usage:
#   ./scripts/build.sh                # Full build (typecheck + test + build Sentinel Server + build Sentinel Developer Console)
#   ./scripts/build.sh --skip-checks  # Skip typecheck and tests

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Platform detection — provides detect_platform, parse_env_flag.
# Honors --env local_macos | local_ubuntu (or AA_PLATFORM_OVERRIDE env var).
. "$SCRIPT_DIR/lib/platform.sh"

SKIP_CHECKS=false

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

# Strip --env first; exported so any child process inherits the override.
parse_env_flag "$@"
set -- "${REMAINING_ARGS[@]}"
export AA_PLATFORM_OVERRIDE

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-checks) SKIP_CHECKS=true; shift ;;
    --help|-h)
      echo "Usage: $0 [--skip-checks] [--env local_macos|local_ubuntu]"
      echo ""
      echo "Options:"
      echo "  --skip-checks    Skip typecheck and vitest"
      echo "  --env <name>     Force platform path (auto-detected by default)"
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
BUILD_DIR="$PROJECT_ROOT/build"
BUILD_LOG="$BUILD_DIR/build-$(date +%Y%m%d-%H%M%S).log"
mkdir -p "$BUILD_DIR"
touch "$BUILD_LOG"
ln -sfn "$(basename "$BUILD_LOG")" "$BUILD_DIR/latest.log" 2>/dev/null || true
# Stream to terminal with colors, write to log file with ANSI codes stripped.
exec > >(tee >(sed 's/\x1b\[[0-9;]*m//g' >> "$BUILD_LOG")) 2>&1
echo "Build log: $BUILD_LOG"

# ── Load env ─────────────────────────────────────────────────────────────────
if [[ -f "$PROJECT_ROOT/.env" ]]; then
  set -a
  source "$PROJECT_ROOT/.env" || true
  set +a
fi

banner "Build"

# ── Validate prerequisites ───────────────────────────────────────────────────
if [[ ! -f package.json ]]; then
  BANNER_SHOWN=true
  banner "Build Failed"
  echo "  package.json not found."
  echo ""
  echo "  Next steps:"
  echo "    1. Run Phase 0 scaffolding first (see implementation plan)"
  echo "    2. Or run: ./scripts/prepare.sh"
  echo ""
  exit 1
fi

if [[ ! -d node_modules ]]; then
  BANNER_SHOWN=true
  banner "Build Failed"
  echo "  Dependencies missing."
  echo ""
  echo "  Next steps:"
  echo "    1. ./scripts/prepare.sh — install dependencies"
  echo "    2. Re-run: ./scripts/build.sh"
  echo ""
  exit 1
fi

# ── TypeScript check ─────────────────────────────────────────────────────────
if [[ "$SKIP_CHECKS" != "true" ]]; then
  log_step "Running TypeScript check..."
  npx tsc --noEmit
  log_info "TypeScript: zero errors"
fi

# ── Native modules ABI check (Linux only) ───────────────────────────────────
# better-sqlite3 ships a precompiled .node binding tied to NODE_MODULE_VERSION.
# On Ubuntu, the apt nodejs (18) → NodeSource 22+ upgrade leaves stale bindings
# that crash vitest. Rebuild before tests if the ABI doesn't match. Gated to
# Linux (detect_platform respects --env local_ubuntu | local_macos).
if [[ "$(detect_platform)" == "linux" && -d node_modules/better-sqlite3 ]]; then
  if ! node -e "const D=require('better-sqlite3'); new D(':memory:').close();" >/dev/null 2>&1; then
    log_step "Rebuilding native modules for Node $(node -v) (Linux)..."
    npm rebuild >/dev/null 2>&1 || { log_error "npm rebuild failed — try: rm -rf node_modules && npm install"; exit 1; }
    log_info "Native modules rebuilt"
  fi
fi

# ── Unit tests ───────────────────────────────────────────────────────────────
if [[ "$SKIP_CHECKS" != "true" ]]; then
  log_step "Running Vitest unit tests..."
  npx vitest run
  log_info "All tests passed"
fi

# ── Build Sentinel Server ─────────────────────────────────────────────────────────────
log_step "Building Sentinel Server (TypeScript → dist/)..."
npx tsc -p tsconfig.build.json 2>/dev/null || npx tsc --outDir dist
log_info "Sentinel Server build complete"

# ── Build Sentinel Developer Console ─────────────────────────────────────────────────────
if [[ -d "$PROJECT_ROOT/console" && -f "$PROJECT_ROOT/console/package.json" ]]; then
  log_step "Building Sentinel Developer Console (Next.js)..."
  cd "$PROJECT_ROOT/console"
  # Build both console variants (Hub + Sentinel) as separate static exports.
  # Force production mode even if repo-level .env sets NODE_ENV=development.
  NODE_ENV=production npm run build:all
  cd "$PROJECT_ROOT"
  log_info "Console build complete"
else
  log_warn "Console not found (console/ directory missing) — skipping console build"
fi

# ── Build Go binaries ───────────────────────────────────────────────────────
if [[ -f "$PROJECT_ROOT/go/Makefile" ]]; then
  log_step "Building Go binaries (5 statically compiled binaries)..."
  cd "$PROJECT_ROOT/go"
  make build-go
  cd "$PROJECT_ROOT"
  log_info "Go build complete → go/bin/"
else
  log_warn "Go directory not found (go/Makefile missing) — skipping Go build"
fi

log_build_summary

BANNER_SHOWN=true
banner "Build Complete"
echo "  Next steps:"
echo "    1. ./scripts/deploy.sh --migrate             — start Sentinel Server + Sentinel Developer Console on port 6000"
echo "    2. ./scripts/run_tests.sh                    — run tests with dashboard on port 7000"
echo ""
