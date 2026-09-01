#!/bin/bash
# Enforcer — Package for Distribution
#
# Builds compiled binaries and packages them into distribution tarballs.
# No source code (.ts files) is included — only compiled .js and configs.
#
# Output:
#   dist/enforcer-central-<platform>.tar.gz    — Hub Server package
#   dist/enforcer-client-<platform>.tar.gz     — Sentinel Server package
#   dist/enforcer_deploy.sh                    — Installer script
#
# Usage:
#   ./scripts/package.sh                     # Build + package
#   ./scripts/package.sh --encrypt KEY       # Build + package + encrypt with key
#   ./scripts/package.sh --skip-build        # Package only (skip TypeScript compilation)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

ENCRYPT_KEY=""
SKIP_BUILD=false
PLATFORM="$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m)"
VERSION=$(node -pe "require('$PROJECT_ROOT/package.json').version" 2>/dev/null || echo "0.1.0")
DIST_DIR="$PROJECT_ROOT/dist"
STAGING_DIR="$DIST_DIR/staging"

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
    --encrypt) ENCRYPT_KEY="$2"; shift 2 ;;
    --skip-build) SKIP_BUILD=true; shift ;;
    --help|-h)
      echo "Usage: $0 [--encrypt KEY] [--skip-build]"
      echo ""
      echo "Builds and packages Enforcer for distribution."
echo "Output: dist/enforcer-{central,client}-${PLATFORM}.tar.gz (Hub/Sentinel packages)"
      exit 0
      ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

banner "Package Start"
echo -e "${CYAN}${BOLD}Packaging Enforcer v${VERSION} for ${PLATFORM}${NC}"
echo ""

# ── Build ────────────────────────────────────────────────────────────────────

if [[ "$SKIP_BUILD" != "true" ]]; then
  echo -e "${CYAN}[BUILD]${NC} Compiling TypeScript..."
  cd "$PROJECT_ROOT"

  # TypeScript check
  npx tsc --noEmit
  echo -e "${GREEN}[OK]${NC} TypeScript: zero errors"

  # Compile to dist/compiled/
  rm -rf "$DIST_DIR/compiled"
  npx tsc -p tsconfig.build.json --outDir "$DIST_DIR/compiled"
  echo -e "${GREEN}[OK]${NC} Compiled to $DIST_DIR/compiled/"
else
  echo -e "${DIM}[SKIP]${NC} Build skipped"
fi

# ── Stage Central Server Package ─────────────────────────────────────────────

echo -e "${CYAN}[STAGE]${NC} Staging Hub Server package..."

CENTRAL_STAGE="$STAGING_DIR/enforcer-central"
rm -rf "$CENTRAL_STAGE"
mkdir -p "$CENTRAL_STAGE"/{bin,config,scripts,lib}

# Compiled JS (no TypeScript source)
cp -r "$DIST_DIR/compiled/src/central" "$CENTRAL_STAGE/lib/"
cp -r "$DIST_DIR/compiled/src/policy" "$CENTRAL_STAGE/lib/"
cp -r "$DIST_DIR/compiled/src/audit" "$CENTRAL_STAGE/lib/"
cp -r "$DIST_DIR/compiled/src/daemon" "$CENTRAL_STAGE/lib/"
cp -r "$DIST_DIR/compiled/types" "$CENTRAL_STAGE/lib/"

# Node modules (production only)
cp "$PROJECT_ROOT/package.json" "$CENTRAL_STAGE/"
cd "$CENTRAL_STAGE"
npm install --omit=dev --no-audit --no-fund 2>/dev/null || cp -r "$PROJECT_ROOT/node_modules" "$CENTRAL_STAGE/"
cd "$PROJECT_ROOT"

# Config templates
cp -r "$PROJECT_ROOT/policies" "$CENTRAL_STAGE/config/"
cp "$PROJECT_ROOT/docker/init.sql" "$CENTRAL_STAGE/config/"

# Scripts
cp "$PROJECT_ROOT/scripts/generate-certs.sh" "$CENTRAL_STAGE/scripts/"
cp "$PROJECT_ROOT/scripts/enforcer_deploy.sh" "$CENTRAL_STAGE/scripts/"
chmod +x "$CENTRAL_STAGE/scripts/"*

# Entry point wrapper
cat > "$CENTRAL_STAGE/bin/enforcer-central" << 'WRAPPER'
#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="$(dirname "$SCRIPT_DIR")"
cd "$APP_DIR"
exec node lib/central/server.js "$@"
WRAPPER
chmod +x "$CENTRAL_STAGE/bin/enforcer-central"

# Version file
echo "{\"version\":\"$VERSION\",\"platform\":\"$PLATFORM\",\"built\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"type\":\"central\"}" > "$CENTRAL_STAGE/version.json"

echo -e "${GREEN}[OK]${NC} Hub Server staged"

# ── Stage Client Agent Package ───────────────────────────────────────────────

echo -e "${CYAN}[STAGE]${NC} Staging Sentinel Server package..."

CLIENT_STAGE="$STAGING_DIR/enforcer-client"
rm -rf "$CLIENT_STAGE"
mkdir -p "$CLIENT_STAGE"/{bin,config,scripts,lib}

# Compiled JS (no TypeScript source)
cp -r "$DIST_DIR/compiled/src/daemon" "$CLIENT_STAGE/lib/"
cp -r "$DIST_DIR/compiled/src/enforcement" "$CLIENT_STAGE/lib/"
cp -r "$DIST_DIR/compiled/src/policy" "$CLIENT_STAGE/lib/"
cp -r "$DIST_DIR/compiled/src/approval" "$CLIENT_STAGE/lib/"
cp -r "$DIST_DIR/compiled/src/audit" "$CLIENT_STAGE/lib/"
cp -r "$DIST_DIR/compiled/src/client" "$CLIENT_STAGE/lib/"
cp -r "$DIST_DIR/compiled/types" "$CLIENT_STAGE/lib/"

# Node modules
cp "$PROJECT_ROOT/package.json" "$CLIENT_STAGE/"
cd "$CLIENT_STAGE"
npm install --omit=dev --no-audit --no-fund 2>/dev/null || cp -r "$PROJECT_ROOT/node_modules" "$CLIENT_STAGE/"
cd "$PROJECT_ROOT"

# Config
cp -r "$PROJECT_ROOT/policies" "$CLIENT_STAGE/config/"

# Scripts
cp "$PROJECT_ROOT/scripts/enforcer_deploy.sh" "$CLIENT_STAGE/scripts/"
chmod +x "$CLIENT_STAGE/scripts/"*

# Entry point wrappers
cat > "$CLIENT_STAGE/bin/enforcer-client" << 'WRAPPER'
#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="$(dirname "$SCRIPT_DIR")"
cd "$APP_DIR"
exec node lib/daemon/server.js "$@"
WRAPPER
chmod +x "$CLIENT_STAGE/bin/enforcer-client"

cat > "$CLIENT_STAGE/bin/enforcer-hook" << 'WRAPPER'
#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="$(dirname "$SCRIPT_DIR")"
cd "$APP_DIR"
exec node lib/enforcement/hook-handler.js "$@"
WRAPPER
chmod +x "$CLIENT_STAGE/bin/enforcer-hook"

# Version file
echo "{\"version\":\"$VERSION\",\"platform\":\"$PLATFORM\",\"built\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"type\":\"client\"}" > "$CLIENT_STAGE/version.json"

echo -e "${GREEN}[OK]${NC} Sentinel Server staged"

# ── Create Tarballs ──────────────────────────────────────────────────────────

echo -e "${CYAN}[PACKAGE]${NC} Creating distribution tarballs..."

CENTRAL_TAR="$DIST_DIR/enforcer-central-${VERSION}-${PLATFORM}.tar.gz"
CLIENT_TAR="$DIST_DIR/enforcer-client-${VERSION}-${PLATFORM}.tar.gz"

cd "$STAGING_DIR"
tar czf "$CENTRAL_TAR" -C "$STAGING_DIR" enforcer-central
tar czf "$CLIENT_TAR" -C "$STAGING_DIR" enforcer-client

echo -e "${GREEN}[OK]${NC} Central: $CENTRAL_TAR ($(du -h "$CENTRAL_TAR" | awk '{print $1}'))"
echo -e "${GREEN}[OK]${NC} Client:  $CLIENT_TAR ($(du -h "$CLIENT_TAR" | awk '{print $1}'))"

# ── Encrypt (optional) ──────────────────────────────────────────────────────

if [[ -n "$ENCRYPT_KEY" ]]; then
  echo -e "${CYAN}[ENCRYPT]${NC} Encrypting tarballs with AES-256-CBC..."

  openssl enc -aes-256-cbc -salt -pbkdf2 -in "$CENTRAL_TAR" -out "${CENTRAL_TAR}.enc" -pass "pass:${ENCRYPT_KEY}"
  openssl enc -aes-256-cbc -salt -pbkdf2 -in "$CLIENT_TAR" -out "${CLIENT_TAR}.enc" -pass "pass:${ENCRYPT_KEY}"

  echo -e "${GREEN}[OK]${NC} Encrypted: ${CENTRAL_TAR}.enc"
  echo -e "${GREEN}[OK]${NC} Encrypted: ${CLIENT_TAR}.enc"
  echo ""
  echo -e "${YELLOW}[NOTE]${NC} To decrypt: openssl enc -d -aes-256-cbc -pbkdf2 -in file.tar.gz.enc -out file.tar.gz -pass pass:KEY"
fi

# ── Copy installer script ───────────────────────────────────────────────────

cp "$PROJECT_ROOT/scripts/enforcer_deploy.sh" "$DIST_DIR/enforcer_deploy.sh"
chmod +x "$DIST_DIR/enforcer_deploy.sh"

# ── Cleanup staging ─────────────────────────────────────────────────────────

rm -rf "$STAGING_DIR"

# ── Summary ──────────────────────────────────────────────────────────────────

echo ""
echo -e "${GREEN}${BOLD}Packaging complete.${NC}"
echo ""
echo "  Distribution files:"
echo "    $CENTRAL_TAR"
echo "    $CLIENT_TAR"
echo "    $DIST_DIR/enforcer_deploy.sh"
if [[ -n "$ENCRYPT_KEY" ]]; then
  echo "    ${CENTRAL_TAR}.enc (encrypted)"
  echo "    ${CLIENT_TAR}.enc (encrypted)"
fi
echo ""
echo "  Deployment:"
echo "    1. Copy the tarball + deploy script to the target machine"
echo "    2. Untar: tar xzf enforcer-{central|client}-*.tar.gz"
echo "    3. Install: sudo ./enforcer_deploy.sh {hub|sentinel}"
echo ""
echo "  No source code (.ts) is included — only compiled JavaScript."
echo ""
banner "Package Complete"
