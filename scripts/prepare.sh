#!/bin/bash
# Enforcer — Runtime security and governance for AI coding agents.
# Author: Deepankar Das
# Preparation Script — Validates and installs prerequisites for build/deploy
#
# Usage:
#   ./scripts/prepare.sh                    # Install missing tools + validate
#   ./scripts/prepare.sh --check-only       # Validate only (no installs)
#   ./scripts/prepare.sh --skip-security    # Skip npm security audit

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Platform detection — gives us detect_platform, root_group, etc.
. "$SCRIPT_DIR/lib/platform.sh"

AUTO_INSTALL=true
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
    echo "       (run without --check-only to auto-install missing tools)"
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
# Stream to terminal with colors, write to log file with ANSI codes stripped.
exec > >(tee >(sed 's/\x1b\[[0-9;]*m//g' >> "$PREPARE_LOG")) 2>&1
echo "Prepare log: $PREPARE_LOG"

if [[ -f "$PROJECT_ROOT/.env" ]]; then
  set -a
  source "$PROJECT_ROOT/.env" || true
  set +a
fi

# Strip --env flag first; remaining args are processed below.
parse_env_flag "$@"
set -- "${REMAINING_ARGS[@]}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --check-only|--dry-run) AUTO_INSTALL=false; shift ;;
    --skip-security) SKIP_SECURITY=true; shift ;;
    --help|-h)
      echo "Usage: $0 [--check-only] [--skip-security] [--env local_macos|local_ubuntu]"
      echo ""
      echo "Options:"
      echo "  --check-only     Validate only — report missing tools without installing"
      echo "  --skip-security  Skip npm security audit"
      echo "  --env <name>     Force platform path (auto-detected by default)"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

if [[ "$(detect_platform)" == "unsupported" ]]; then
  log_error "This script targets macOS and Linux. Use --env local_macos or --env local_ubuntu to force a path."
  exit 1
fi

# ── Homebrew (macOS only) ────────────────────────────────────────────────────

check_homebrew() {
  if [[ "$(detect_platform)" != "macos" ]]; then
    return
  fi

  log_step "Homebrew"
  if command -v brew >/dev/null 2>&1; then
    log_info "Homebrew $(brew --version | head -1 | awk '{print $2}')"
    return
  fi

  if [[ "$AUTO_INSTALL" == "true" ]]; then
    log_warn "Installing Homebrew..."
    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
  else
    log_error "Homebrew missing. Run without --check-only to auto-install, or install from https://brew.sh"
  fi
}

# ── Go ───────────────────────────────────────────────────────────────────────

# Compare two semver-ish strings: returns 0 if $1 >= $2, 1 otherwise.
version_ge() {
  [[ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | tail -n1)" == "$1" ]]
}

required_go_version() {
  # Read major.minor from go/go.mod (line like: "go 1.26.2"). Go's auto-toolchain
  # mechanism (>=1.21) fetches the exact patch version on demand, so the
  # installed Go only needs to match major.minor.
  if [[ -f "$PROJECT_ROOT/go/go.mod" ]]; then
    awk '/^go [0-9]+\.[0-9]+/ {split($2, v, "."); print v[1]"."v[2]".0"; exit}' "$PROJECT_ROOT/go/go.mod"
  else
    echo "1.26.0"
  fi
}

check_go() {
  log_step "Go toolchain"

  local required current
  required="$(required_go_version)"

  if command -v go >/dev/null 2>&1; then
    current="$(go version | awk '{print $3}' | sed 's/^go//')"
    if version_ge "$current" "$required"; then
      log_info "Go $current (>= required $required)"
      return
    fi
    log_warn "Go $current is older than required $required — upgrading..."
  fi

  if [[ "$AUTO_INSTALL" != "true" ]]; then
    log_error "Go >= $required not installed. Run without --check-only to auto-install."
    return
  fi

  case "$(detect_platform)" in
    macos)
      brew install go
      ;;
    linux)
      local arch tarball url
      case "$(uname -m)" in
        x86_64)  arch="amd64" ;;
        aarch64) arch="arm64" ;;
        *) log_error "Unsupported arch: $(uname -m)"; return ;;
      esac
      tarball="go${required}.linux-${arch}.tar.gz"
      url="https://go.dev/dl/${tarball}"
      log_warn "Downloading $url ..."
      local tmp
      tmp="$(mktemp -d)"
      if ! curl -fsSL -o "$tmp/$tarball" "$url"; then
        log_error "Download failed: $url (try a newer/older patch version in go.mod)"
        rm -rf "$tmp"
        return
      fi
      sudo rm -rf /usr/local/go
      sudo tar -C /usr/local -xzf "$tmp/$tarball"
      rm -rf "$tmp"
      # Ensure /usr/local/go/bin is on PATH for this session and future shells
      export PATH="/usr/local/go/bin:$PATH"
      if ! grep -q '/usr/local/go/bin' "$HOME/.profile" 2>/dev/null; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> "$HOME/.profile"
      fi
      ;;
    *)
      log_error "Unsupported platform for Go install"
      return
      ;;
  esac

  if command -v go >/dev/null 2>&1; then
    log_info "Go $(go version | awk '{print $3}' | sed 's/^go//') installed"
  else
    log_error "Go install did not complete (PATH may need refresh — open a new shell)"
  fi
}

# ── Node.js ──────────────────────────────────────────────────────────────────

check_node() {
  log_step "Node.js + npm"

  # Next.js 15 requires Node >= 20.9.0
  local required_node="20.9.0"

  install_node() {
    case "$(detect_platform)" in
      macos)
        brew install node@22
        brew link node@22 --force --overwrite || true
        ;;
      linux)
        curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
        sudo apt-get install -y nodejs
        ;;
      *)
        log_error "Unsupported platform for Node.js install"
        return 1
        ;;
    esac
  }

  local current=""
  if command -v node >/dev/null 2>&1; then
    current="$(node -v | sed 's/^v//')"
  fi

  if [[ -z "$current" ]]; then
    if [[ "$AUTO_INSTALL" == "true" ]]; then
      log_warn "Node.js not found — installing Node 22..."
      install_node
    else
      log_error "Node.js not found."
      return
    fi
  elif ! version_ge "$current" "$required_node"; then
    if [[ "$AUTO_INSTALL" == "true" ]]; then
      log_warn "Node $current is older than required $required_node — upgrading to Node 22..."
      install_node
    else
      log_error "Node $current is older than required $required_node (Next.js needs >= 20.9)"
      return
    fi
  fi

  if command -v node >/dev/null 2>&1; then
    current="$(node -v | sed 's/^v//')"
    if version_ge "$current" "$required_node"; then
      log_info "Node v$current (>= required $required_node)"
    else
      log_error "Node v$current still below required $required_node after install attempt"
    fi
  fi

  if command -v npm >/dev/null 2>&1; then
    log_info "npm $(npm -v)"
  else
    log_error "npm not found"
  fi
}

# ── .env ─────────────────────────────────────────────────────────────────────

check_dotenv() {
  log_step ".env"

  if [[ ! -f "$PROJECT_ROOT/.env" ]]; then
    if [[ "$AUTO_INSTALL" == "true" ]]; then
      local db_user
      db_user="$(whoami)"
      cat > "$PROJECT_ROOT/.env" <<ENVEOF
# Enforcer — Environment Configuration
# Sentinel Server
DAEMON_PORT=9100
PROXY_PORT=9101
CONSOLE_PORT=6000

# Database (audit store)
DATABASE_URL=postgresql://${db_user}@localhost:5432/enforcer

# Node
NODE_ENV=development
ENVEOF
      log_info "Created .env with default ports (Sentinel Server API: 9100, Sentinel Server egress: 9101, Sentinel Developer Console: 6000)"
    else
      log_warn ".env missing (optional for local dev)"
      return
    fi
  fi

  if grep -q '^DAEMON_PORT=' "$PROJECT_ROOT/.env"; then
    log_info "DAEMON_PORT set"
  else
    log_warn "DAEMON_PORT not set (will use default 9100)"
  fi

  if grep -q '^DATABASE_URL=' "$PROJECT_ROOT/.env"; then
    log_info "DATABASE_URL set"
  else
    log_warn "DATABASE_URL not set"
  fi
}

# ── PostgreSQL ───────────────────────────────────────────────────────────────

check_postgres() {
  log_step "PostgreSQL"

  if ! command -v psql >/dev/null 2>&1; then
    if [[ "$AUTO_INSTALL" == "true" ]]; then
      log_warn "Installing postgresql@16..."
      case "$(detect_platform)" in
        macos)
          brew install postgresql@16
          brew link postgresql@16 --force --overwrite || true
          ;;
        linux)
          sudo apt-get install -y postgresql-16 || log_warn "PostgreSQL 16 not available, trying default..."
          sudo apt-get install -y postgresql || true
          ;;
      esac
    else
      log_error "psql not found. Run without --check-only to auto-install, or install PostgreSQL manually."
      return
    fi
  fi

  if command -v psql >/dev/null 2>&1; then
    log_info "psql available"
  fi

  if pg_isready -q >/dev/null 2>&1; then
    log_info "PostgreSQL running"
  else
    if [[ "$(detect_platform)" == "linux" ]]; then
      log_warn "PostgreSQL not running — starting via systemctl..."
      sudo systemctl start postgresql 2>/dev/null || true
      sudo systemctl enable postgresql 2>/dev/null || true
    fi
    if ! pg_isready -q >/dev/null 2>&1; then
      log_error "PostgreSQL not running (macOS: brew services start postgresql@16; Linux: sudo systemctl start postgresql)"
      return
    fi
    log_info "PostgreSQL started"
  fi

  local db_name="enforcer"
  local creds_file="/etc/enforcer/.db_credentials"

  # On Linux, the hardened install path is setup-database.sh (creates a
  # restricted enforcer role, schema with append-only grants, root-owned
  # credentials file). Don't try peer-auth createdb — it doesn't work on
  # Ubuntu's system PostgreSQL where the dev user is not a Postgres role.
  if [[ "$(detect_platform)" == "linux" ]]; then
    if [[ -f "$creds_file" ]]; then
      log_info "Database '$db_name' provisioned (credentials at $creds_file)"
    elif [[ "$AUTO_INSTALL" == "true" ]]; then
      log_warn "Provisioning database via setup-database.sh (sudo required)..."
      if sudo "$SCRIPT_DIR/setup-database.sh"; then
        log_info "Database provisioned"
      else
        log_error "setup-database.sh failed — see output above"
      fi
    else
      log_error "Database not provisioned. Run: sudo $SCRIPT_DIR/setup-database.sh"
    fi
    return
  fi

  # macOS Homebrew path: peer-auth createdb works because the dev user is
  # the PostgreSQL superuser.
  if psql -U "$(whoami)" -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='$db_name'" 2>/dev/null | grep -q 1; then
    log_info "Database '$db_name' exists"
  else
    if [[ "$AUTO_INSTALL" == "true" ]]; then
      createdb "$db_name" 2>/dev/null && log_info "Created database '$db_name'" || log_error "Could not create database '$db_name'"
    else
      log_error "Database '$db_name' does not exist (run without --check-only to auto-create)"
    fi
  fi

  # Run schema migration if init.sql exists
  if [[ -f "$PROJECT_ROOT/docker/init.sql" ]]; then
    if psql -d "$db_name" -c "SELECT 1 FROM audit_events LIMIT 0" >/dev/null 2>&1; then
      log_info "Schema already applied (audit_events table exists)"
    else
      if [[ "$AUTO_INSTALL" == "true" ]]; then
        psql -d "$db_name" -f "$PROJECT_ROOT/docker/init.sql" >/dev/null 2>&1 && log_info "Schema migration applied" || log_error "Schema migration failed"
      else
        log_error "Schema not applied (run without --check-only to auto-apply)"
      fi
    fi
  fi
}

# ── Docker ───────────────────────────────────────────────────────────────────

check_docker() {
  log_step "Docker"

  if ! command -v docker >/dev/null 2>&1; then
    if [[ "$AUTO_INSTALL" == "true" ]]; then
      log_warn "Installing Docker..."
      case "$(detect_platform)" in
        macos) brew install --cask docker ;;
        linux) curl -fsSL https://get.docker.com | sh ;;
      esac
    else
      log_error "Docker not found. Run without --check-only to auto-install."
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

# ── npm dependencies ─────────────────────────────────────────────────────────

check_dependencies() {
  log_step "npm dependencies"
  cd "$PROJECT_ROOT"

  if [[ ! -f package.json ]]; then
    log_warn "package.json not found (project not yet scaffolded)"
    return
  fi

  if [[ ! -d node_modules ]]; then
    if [[ "$AUTO_INSTALL" == "true" ]]; then
      log_warn "Installing npm dependencies..."
      npm install --include=dev --no-audit --no-fund >/dev/null 2>&1
      log_info "Dependencies installed"
    else
      log_warn "node_modules missing (run npm install)"
    fi
  else
    log_info "Dependencies installed"
  fi
}

# ── Native module ABI (Linux only) ───────────────────────────────────────────
#
# better-sqlite3 ships a precompiled .node binary tied to a specific
# NODE_MODULE_VERSION. On Ubuntu, apt's nodejs (Node 18) commonly gets
# installed before prepare.sh upgrades to Node 22+ via NodeSource, leaving
# stale native bindings. `npm rebuild` recompiles them against the current
# Node. macOS users typically don't switch Node versions mid-project, so this
# check is gated to Linux.
check_native_modules() {
  if [[ "$(detect_platform)" != "linux" ]]; then
    return
  fi

  log_step "Native modules ABI (Linux)"
  cd "$PROJECT_ROOT"

  if [[ ! -d node_modules/better-sqlite3 ]]; then
    log_info "No native modules to verify"
    return
  fi

  # Instantiate Database to force loading the native .node binding (better-sqlite3
  # lazy-loads it). If the ABI matches, exit 0; otherwise this throws
  # "NODE_MODULE_VERSION X requires Y" or "Module did not self-register".
  if node -e "const D=require('better-sqlite3'); new D(':memory:').close();" >/dev/null 2>&1; then
    log_info "Native modules ABI matches Node $(node -v)"
    return
  fi

  if [[ "$AUTO_INSTALL" != "true" ]]; then
    log_error "Native module ABI mismatch (Node version changed since install). Run without --check-only to rebuild."
    return
  fi

  log_warn "Native modules built against a different Node ABI — running npm rebuild..."
  if npm rebuild >/dev/null 2>&1; then
    log_info "Native modules rebuilt against $(node -v)"
  else
    log_error "npm rebuild failed — try: rm -rf node_modules && npm install"
  fi
}

# ── Console dependencies (Next.js + shadcn/ui) ──────────────────────────────

check_console_dependencies() {
  log_step "Console dependencies (Next.js + shadcn/ui)"
  cd "$PROJECT_ROOT"

  if [[ ! -d console || ! -f console/package.json ]]; then
    log_warn "Console not found (console/ directory missing — skip if not yet scaffolded)"
    return
  fi

  cd "$PROJECT_ROOT/console"

  if [[ ! -d node_modules ]]; then
    if [[ "$AUTO_INSTALL" == "true" ]]; then
      log_warn "Installing console npm dependencies..."
      npm install --include=dev --no-audit --no-fund >/dev/null 2>&1
      log_info "Console dependencies installed"
    else
      log_warn "Console node_modules missing (run npm install in console/)"
    fi
  else
    log_info "Console dependencies installed"
  fi

  cd "$PROJECT_ROOT"
}

# ── Playwright (E2E testing) ─────────────────────────────────────────────────

check_playwright() {
  log_step "Playwright (E2E testing)"
  cd "$PROJECT_ROOT"

  if [[ ! -f package.json ]]; then
    log_warn "Skipping Playwright (project not scaffolded)"
    return
  fi

  # Check if @playwright/test is in devDependencies
  if ! node -e "require.resolve('@playwright/test')" >/dev/null 2>&1; then
    if [[ "$AUTO_INSTALL" == "true" ]]; then
      log_warn "Installing Playwright..."
      npm install --save-dev @playwright/test >/dev/null 2>&1
      log_info "@playwright/test installed"
    else
      log_warn "@playwright/test not installed (run without --check-only to auto-apply)"
      return
    fi
  else
    log_info "@playwright/test installed"
  fi

  # Check if Chromium browser binary is available
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
    # Always install Chromium if missing
    log_warn "Chromium browser not found — installing..."
    if npx playwright install chromium >/dev/null 2>&1; then
      log_info "Playwright Chromium installed"
    else
      log_error "Playwright Chromium install failed. Run manually: npx playwright install chromium"
    fi
  fi
}

# ── TypeScript check ─────────────────────────────────────────────────────────

check_typecheck() {
  log_step "TypeScript"
  cd "$PROJECT_ROOT"
  if [[ -f tsconfig.json && -d node_modules ]]; then
    npx tsc --noEmit >/dev/null 2>&1 && log_info "Typecheck passed" || log_warn "Typecheck has issues"
  else
    log_warn "Skipping typecheck (tsconfig.json or dependencies missing)"
  fi
}

# ── npm audit ────────────────────────────────────────────────────────────────

check_security() {
  if [[ "$SKIP_SECURITY" == "true" ]]; then
    log_step "Security"
    log_info "Skipped npm audit"
    return
  fi

  log_step "Security"
  cd "$PROJECT_ROOT"
  if [[ -d node_modules ]]; then
    if npm audit --omit=dev >/dev/null 2>&1; then
      log_info "No production audit findings"
    else
      log_warn "npm audit findings detected — run 'npm audit --omit=dev' for details"
    fi
  else
    log_warn "Skipping npm audit (dependencies missing)"
  fi
}

# ── Run all checks ───────────────────────────────────────────────────────────

banner "Preparation Check"

check_homebrew
check_go
check_node
check_dotenv
check_postgres
check_docker
check_dependencies
check_native_modules
check_console_dependencies
check_playwright
check_typecheck
check_security

BANNER_SHOWN=true
if [[ $ERRORS -eq 0 ]]; then
  banner "Preparation Complete"
  echo -e "  ${GREEN}All checks passed!${NC} ($WARNINGS warnings)"
  echo ""
  echo "  Next steps:"
  echo "    1. ./scripts/build.sh                        — typecheck + test + build"
  echo "    2. ./scripts/deploy.sh --migrate             — migrate + start on port 6000"
  echo ""
else
  banner "Preparation Failed"
  echo -e "  ${RED}$ERRORS check(s) failed${NC} ($WARNINGS warnings)"
  echo ""
  echo "  Fix the issues above before running build or deploy scripts."
  if [[ "$AUTO_INSTALL" == "false" ]]; then
    echo "  Tip: Remove --check-only to auto-install missing tools."
  fi
  echo ""
  exit 1
fi
