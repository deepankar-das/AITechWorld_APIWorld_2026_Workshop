#!/bin/bash
# Syntra — The operating layer for in-house legal.
# Use with permission from the author.
# Author: Deepankar Das
# Preparation Script — Validates and installs prerequisites for build/deploy
#
# Usage:
#   ./scripts/prepare.sh                    # Check all prerequisites
#   ./scripts/prepare.sh --install          # Auto-install missing tools
#   ./scripts/prepare.sh --skip-security    # Skip npm security audit

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

AUTO_INSTALL=false
SKIP_SECURITY=false

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

ERRORS=0
WARNINGS=0

log_info()  { echo -e "${GREEN}[OK]${NC}    $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; WARNINGS=$((WARNINGS + 1)); }
log_error() { echo -e "${RED}[FAIL]${NC}  $1"; ERRORS=$((ERRORS + 1)); }
log_step()  { echo -e "${BLUE}[CHECK]${NC} $1"; }

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
    banner "Preparation Failed (exit code $code)"
    echo "  The error above shows what went wrong."
    echo "  This script is safe to re-run after fixing the issue."
    echo ""
    echo "  Next steps:"
    echo "    1. Fix the error shown above"
    echo "    2. Re-run: ./scripts/prepare.sh"
    echo "       (or with --install to auto-install missing tools)"
    echo ""
  fi
}
trap on_exit EXIT

# ── Log to file + terminal ───────────────────────────────────────────────────
PREPARE_DIR="$PROJECT_ROOT/build"
PREPARE_LOG="$PREPARE_DIR/prepare-$(date +%Y%m%d-%H%M%S).log"
mkdir -p "$PREPARE_DIR"
touch "$PREPARE_LOG"
ln -sfn "$(basename "$PREPARE_LOG")" "$PREPARE_DIR/prepare-latest.log" 2>/dev/null || true
exec > >(tee -a "$PREPARE_LOG") 2>&1
echo "Prepare log: $PREPARE_LOG"

if [[ -f "$PROJECT_ROOT/.env" ]]; then
  set -a
  source "$PROJECT_ROOT/.env" || true
  set +a
fi

uses_postgres() {
  [[ "${DATABASE_URL:-}" =~ ^postgres(ql)?:// ]]
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --install) AUTO_INSTALL=true; shift ;;
    --skip-security) SKIP_SECURITY=true; shift ;;
    --help|-h)
      echo "Usage: $0 [--install] [--skip-security]"
      echo ""
      echo "Options:"
      echo "  --install        Auto-install missing tools via Homebrew"
      echo "  --skip-security  Skip npm security audit"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

if [[ "$(uname)" != "Darwin" ]]; then
  log_error "This script currently targets macOS."
  exit 1
fi

check_homebrew() {
  log_step "Homebrew"
  if command -v brew >/dev/null 2>&1; then
    log_info "Homebrew $(brew --version | head -1 | awk '{print $2}')"
    return
  fi

  if [[ "$AUTO_INSTALL" == "true" ]]; then
    log_warn "Installing Homebrew..."
    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
  else
    log_error "Homebrew missing. Run with --install or install from https://brew.sh"
  fi
}

check_node() {
  log_step "Node.js + npm"

  if ! command -v node >/dev/null 2>&1; then
    if [[ "$AUTO_INSTALL" == "true" ]]; then
      log_warn "Installing Node.js 22..."
      brew install node@22
      brew link node@22 --force --overwrite || true
    else
      log_error "Node.js not found."
      return
    fi
  fi

  if command -v node >/dev/null 2>&1; then
    log_info "Node $(node -v)"
  fi

  if command -v npm >/dev/null 2>&1; then
    log_info "npm $(npm -v)"
  else
    log_error "npm not found"
  fi
}

check_dotenv() {
  log_step ".env"

  if [[ ! -f "$PROJECT_ROOT/.env" ]]; then
    if [[ "$AUTO_INSTALL" == "true" ]]; then
      local secret
      secret=$(openssl rand -hex 24)
      local db_user
      db_user="$(whoami)"
      cat > "$PROJECT_ROOT/.env" <<ENVEOF
JWT_SECRET=$secret
NEXTAUTH_SECRET=$secret
NEXTAUTH_URL=http://localhost:3000
DATABASE_URL=postgresql://${db_user}@localhost:5432/syntra
PORT=3000
NODE_ENV=development

# AI API Keys — add your keys here to enable real AI reviews
# Without these, the system falls back to rule-based extraction
ANTHROPIC_API_KEY=
OPENAI_API_KEY=
ENVEOF
      log_info "Created .env (DATABASE_URL uses local PostgreSQL as user '$db_user')"
    else
      log_warn ".env missing (optional for local dev)"
      return
    fi
  fi

  local jwt_secret nextauth_url
  jwt_secret="$(grep -E '^JWT_SECRET=' "$PROJECT_ROOT/.env" | head -1 | cut -d= -f2- || true)"
  nextauth_url="http://localhost:${PORT:-3000}"

  if [[ -z "$jwt_secret" ]]; then
    if [[ "$AUTO_INSTALL" == "true" ]]; then
      jwt_secret="$(openssl rand -hex 24)"
      printf '\nJWT_SECRET=%s\n' "$jwt_secret" >> "$PROJECT_ROOT/.env"
      log_info "JWT_SECRET set"
    else
      log_warn "JWT_SECRET not set"
    fi
  else
    log_info "JWT_SECRET set"
  fi

  if grep -q '^NEXTAUTH_SECRET=' "$PROJECT_ROOT/.env"; then
    log_info "NEXTAUTH_SECRET set"
  else
    if [[ "$AUTO_INSTALL" == "true" ]]; then
      printf 'NEXTAUTH_SECRET=%s\n' "$jwt_secret" >> "$PROJECT_ROOT/.env"
      log_info "NEXTAUTH_SECRET set"
    else
      log_warn "NEXTAUTH_SECRET not set"
    fi
  fi

  if grep -q '^NEXTAUTH_URL=' "$PROJECT_ROOT/.env"; then
    log_info "NEXTAUTH_URL set"
  else
    if [[ "$AUTO_INSTALL" == "true" ]]; then
      printf 'NEXTAUTH_URL=%s\n' "$nextauth_url" >> "$PROJECT_ROOT/.env"
      log_info "NEXTAUTH_URL set"
    else
      log_warn "NEXTAUTH_URL not set"
    fi
  fi

  if grep -q '^DATABASE_URL=' "$PROJECT_ROOT/.env"; then
    log_info "DATABASE_URL set"
  else
    log_warn "DATABASE_URL not set"
  fi

  # AI API keys (optional but recommended)
  local anthropic_key openai_key
  anthropic_key="$(grep -E '^ANTHROPIC_API_KEY=' "$PROJECT_ROOT/.env" | cut -d= -f2- || true)"
  openai_key="$(grep -E '^OPENAI_API_KEY=' "$PROJECT_ROOT/.env" | cut -d= -f2- || true)"
  if [[ -n "$anthropic_key" && "$anthropic_key" != "" ]]; then
    log_info "ANTHROPIC_API_KEY set (AI clause extraction + review enabled)"
  else
    log_warn "ANTHROPIC_API_KEY not set (AI falls back to rule-based extraction)"
  fi
  if [[ -n "$openai_key" && "$openai_key" != "" ]]; then
    log_info "OPENAI_API_KEY set (embeddings + knowledge layer enabled)"
  else
    log_warn "OPENAI_API_KEY not set (embeddings use zero vectors — knowledge layer disabled)"
  fi
}

check_postgres() {
  if ! uses_postgres; then
    log_step "PostgreSQL"
    log_info "Skipping PostgreSQL checks (DATABASE_URL is non-PostgreSQL)"
    return
  fi

  log_step "PostgreSQL"

  if ! command -v psql >/dev/null 2>&1; then
    if [[ "$AUTO_INSTALL" == "true" ]]; then
      log_warn "Installing postgresql@16..."
      brew install postgresql@16
      brew link postgresql@16 --force --overwrite || true
    else
      log_error "psql not found for PostgreSQL DATABASE_URL"
      return
    fi
  fi

  if pg_isready -q >/dev/null 2>&1; then
    log_info "PostgreSQL running"
  else
    log_warn "PostgreSQL not running (start with: brew services start postgresql@16)"
    return
  fi

  # Create the syntra database if it doesn't exist
  local db_name="syntra"
  if psql -U "$(whoami)" -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='$db_name'" 2>/dev/null | grep -q 1; then
    log_info "Database '$db_name' exists"
  else
    if [[ "$AUTO_INSTALL" == "true" ]]; then
      createdb "$db_name" 2>/dev/null && log_info "Created database '$db_name'" || log_warn "Could not create database '$db_name'"
    else
      log_warn "Database '$db_name' does not exist (run with --install to create)"
    fi
  fi

  # Create the syntra_test database for unit/integration tests
  local test_db="syntra_test"
  if psql -U "$(whoami)" -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='$test_db'" 2>/dev/null | grep -q 1; then
    log_info "Database '$test_db' exists"
  else
    if [[ "$AUTO_INSTALL" == "true" ]]; then
      createdb "$test_db" 2>/dev/null && log_info "Created database '$test_db'" || log_warn "Could not create database '$test_db'"
    else
      log_warn "Database '$test_db' does not exist (run with --install to create)"
    fi
  fi
}

check_docker() {
  log_step "Docker"

  if ! command -v docker >/dev/null 2>&1; then
    if [[ "$AUTO_INSTALL" == "true" ]]; then
      log_warn "Installing Docker Desktop..."
      brew install --cask docker
    else
      log_error "Docker not found. Run with --install to install Docker Desktop."
      return
    fi
  fi

  if command -v docker >/dev/null 2>&1; then
    log_info "Docker $(docker --version | awk '{print $3}' | tr -d ',')"
  else
    log_error "Docker install did not complete successfully"
    return
  fi

  if docker info >/dev/null 2>&1; then
    log_info "Docker daemon running"
  else
    log_warn "Docker daemon not running (start Docker Desktop)"
  fi

  if docker compose version >/dev/null 2>&1; then
    log_info "Docker Compose available"
  else
    log_warn "Docker Compose plugin not available"
  fi
}

check_dependencies() {
  log_step "npm dependencies"
  cd "$PROJECT_ROOT"

  if [[ ! -f package.json ]]; then
    log_error "package.json not found"
    return
  fi

  has_node_module() {
    local module_path="$1"
    [[ -d "node_modules/$module_path" ]]
  }

  install_node_dependencies() {
    log_warn "Installing/repairing npm dependencies (including dev packages)..."
    npm install --include=dev --no-audit --no-fund >/dev/null 2>&1
  }

  if [[ ! -d node_modules ]]; then
    install_node_dependencies
    log_info "Dependencies installed"
  else
    # If lockfile and installed modules drift, npm ls exits non-zero.
    if ! npm ls --depth=0 >/dev/null 2>&1; then
      install_node_dependencies
    fi

    local missing_modules=()
    has_node_module "next" || missing_modules+=("next")
    has_node_module "typescript" || missing_modules+=("typescript")
    has_node_module "@playwright/test" || missing_modules+=("@playwright/test")
    if [[ ${#missing_modules[@]} -gt 0 ]]; then
      install_node_dependencies
    fi

    missing_modules=()
    has_node_module "next" || missing_modules+=("next")
    has_node_module "typescript" || missing_modules+=("typescript")
    has_node_module "@playwright/test" || missing_modules+=("@playwright/test")
    if [[ ${#missing_modules[@]} -gt 0 ]]; then
      log_error "Dependencies still missing after install: ${missing_modules[*]}"
      return
    fi

    log_info "Dependencies installed"
  fi
}

check_playwright() {
  log_step "Playwright (E2E testing)"
  cd "$PROJECT_ROOT"

  if [[ ! -d node_modules/@playwright/test ]]; then
    log_warn "Playwright not installed (run npm install first)"
    return
  fi

  # Check if Chromium browser binary is available.
  # Dry-run output includes multiple "Install location" entries (chromium, ffmpeg, headless shell),
  # so filter explicitly for the real Chromium path.
  local dry_run_output browser_path
  dry_run_output="$(npx playwright install --dry-run chromium 2>&1 || true)"
  browser_path="$(
    printf '%s\n' "$dry_run_output" \
      | sed -n 's/^[[:space:]]*Install location:[[:space:]]*//p' \
      | grep '/chromium-' \
      | head -n 1
  )"

  if [[ -n "$browser_path" && -d "$browser_path" ]]; then
    log_info "Playwright Chromium available"
  else
    # Chromium is a dev dependency — always install it if missing.
    log_warn "Chromium browser not found — installing..."
    if npx playwright install chromium; then
      log_info "Playwright Chromium installed"
    else
      log_error "Playwright Chromium install failed. Run manually: npx playwright install chromium"
    fi
  fi
}

check_typecheck() {
  log_step "TypeScript"
  cd "$PROJECT_ROOT"
  if [[ -d node_modules ]]; then
    npm run -s typecheck >/dev/null 2>&1 && log_info "Typecheck passed" || log_warn "Typecheck has issues"
  else
    log_warn "Skipping typecheck (dependencies missing)"
  fi
}

check_security() {
  if [[ "$SKIP_SECURITY" == "true" ]]; then
    log_step "Security"
    log_info "Skipped npm audit"
    return
  fi

  log_step "Security"
  cd "$PROJECT_ROOT"
  if [[ -d node_modules ]]; then
    local audit_output audit_exit
    if audit_output=$(npm audit --omit=dev 2>&1); then
      audit_exit=0
    else
      audit_exit=$?
    fi

    if [[ $audit_exit -eq 0 ]]; then
      log_info "No production audit findings"
    else
      if echo "$audit_output" | grep -Eq '[0-9]+ (critical|high|moderate|low)'; then
        log_warn "npm audit findings detected; attempting automatic remediation..."
        npm audit fix --omit=dev >/dev/null 2>&1 || true

        local post_output post_exit
        if post_output=$(npm audit --omit=dev 2>&1); then
          post_exit=0
        else
          post_exit=$?
        fi

        if [[ $post_exit -eq 0 ]]; then
          log_info "npm audit fix resolved production findings"
        else
          local summary critical high moderate low
          summary="$(echo "$post_output" | grep -E '[0-9]+ vulnerabilit' | tail -1 || true)"

          critical="$(echo "$summary" | grep -oE '[0-9]+ critical' | awk '{print $1}' || true)"
          high="$(echo "$summary" | grep -oE '[0-9]+ high' | awk '{print $1}' || true)"
          moderate="$(echo "$summary" | grep -oE '[0-9]+ moderate' | awk '{print $1}' || true)"
          low="$(echo "$summary" | grep -oE '[0-9]+ low' | awk '{print $1}' || true)"

          critical="${critical:-0}"
          high="${high:-0}"
          moderate="${moderate:-0}"
          low="${low:-0}"

          # Fallback for output variants such as "4 moderate vulnerabilities found"
          if [[ "$critical" -eq 0 && "$high" -eq 0 && "$moderate" -eq 0 && "$low" -eq 0 ]]; then
            local total_vulns severity_label
            total_vulns="$(echo "$post_output" | grep -oE '[0-9]+ (critical|high|moderate|low)' | head -1 | awk '{print $1}' || true)"
            severity_label="$(echo "$post_output" | grep -oE '(critical|high|moderate|low)' | head -1 || true)"
            total_vulns="${total_vulns:-0}"
            case "$severity_label" in
              critical) critical="$total_vulns" ;;
              high) high="$total_vulns" ;;
              moderate) moderate="$total_vulns" ;;
              low) low="$total_vulns" ;;
            esac
          fi

          if [[ "$critical" -gt 0 || "$high" -gt 0 ]]; then
            log_error "npm audit: ${critical} critical, ${high} high, ${moderate} moderate, ${low} low"
          else
            log_warn "npm audit: ${moderate} moderate, ${low} low (no critical/high)"
          fi

          local affected
          affected="$(echo "$post_output" | grep -E '^[a-z@]' | grep -v 'npm audit' | head -5 || true)"
          if [[ -n "$affected" ]]; then
            echo -e "         Affected packages:"
            echo "$affected" | while IFS= read -r line; do
              echo -e "           $line"
            done
          fi

          if echo "$post_output" | grep -q 'fix available via'; then
            echo -e "         ${YELLOW}Remediation:${NC} npm audit fix (or npm audit fix --force for breaking changes)"
          else
            echo -e "         ${YELLOW}Remediation:${NC} Run 'npm audit --omit=dev' for details. Upstream fixes may be pending."
          fi
        fi
      else
        log_warn "npm audit failed unexpectedly; run 'npm audit --omit=dev' manually."
      fi
    fi
  else
    log_warn "Skipping npm audit (dependencies missing)"
  fi
}

banner "Preparation Check"

check_homebrew
check_node
check_dotenv
check_postgres
check_docker
check_dependencies
check_typecheck
check_playwright
check_security

BANNER_SHOWN=true
if [[ $ERRORS -eq 0 ]]; then
  banner "Preparation Complete"
  echo -e "  ${GREEN}All checks passed!${NC} ($WARNINGS warnings)"
  echo ""
  echo "  Next steps:"
  echo "    1. ./scripts/build.sh                        — typecheck + test + build"
  echo "    2. ./scripts/deploy.sh --migrate --seed-full --force  — migrate + seed + start"
  echo "       (or: ./scripts/deploy.sh for deploy without seeding)"
  echo ""
else
  banner "Preparation Failed"
  echo -e "  ${RED}$ERRORS check(s) failed${NC} ($WARNINGS warnings)"
  echo ""
  echo "  Fix the issues above before running build or deploy scripts."
  if [[ "$AUTO_INSTALL" == "false" ]]; then
    echo "  Tip: Run with --install to auto-install missing tools."
  fi
  echo ""
  exit 1
fi
