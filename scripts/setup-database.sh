#!/usr/bin/env bash
# Author: Deepankar Das
#
# Enforcer — Restricted Database Setup
#
# Creates the enforcer database with a dedicated 'enforcer' user.
# The developer's OS user has NO access to this database.
# Must be run as root (via sudo) — the installer calls this automatically.
#
# Usage: sudo ./scripts/setup-database.sh
#
# What this does:
#   1. Generates a random 32-char password for the enforcer DB user
#   2. Creates the PostgreSQL user and database
#   3. Applies the schema (init.sql) with append-only grants
#   4. Revokes all access from the developer's OS user
#   5. Stores the DATABASE_URL in /etc/enforcer/.db_credentials (root:600)
#   6. Verifies the developer cannot connect

set -euo pipefail

# Platform detection (defines detect_platform, parse_env_flag, root_group, ...)
. "$(dirname "$0")/lib/platform.sh"
parse_env_flag "$@"
set -- "${REMAINING_ARGS[@]}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

DB_SETUP_SUCCEEDED=false
on_exit() {
  local code=$?
  if [[ $code -ne 0 && "$DB_SETUP_SUCCEEDED" != "true" ]]; then
    banner "Database Setup Failed"
  fi
}
trap on_exit EXIT

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

# Must be root
if [[ $EUID -ne 0 ]]; then
    echo -e "${RED}[ERROR]${NC} This script must be run as root (sudo)."
    exit 1
fi

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CONFIG_DIR="/etc/enforcer"
CREDS_FILE="$CONFIG_DIR/.db_credentials"
INIT_SQL="$PROJECT_ROOT/docker/init.sql"

# Detect the developer's OS user (the one who called sudo)
DEV_USER="${SUDO_USER:-$(logname 2>/dev/null || echo '')}"
if [[ -z "$DEV_USER" || "$DEV_USER" == "root" ]]; then
    echo -e "${YELLOW}[WARN]${NC} Could not detect developer username. Skipping per-user revoke."
fi

banner "Database Setup Start"
echo ""
echo "  Developer user: ${DEV_USER:-unknown}"
echo "  Config dir:     $CONFIG_DIR"
echo ""

# ── Idempotency check ──────────────────────────────────────────────────────
CREDS_FILE="$CONFIG_DIR/.db_credentials"
if [[ -f "$CREDS_FILE" ]]; then
    echo -e "${GREEN}[SKIP]${NC} Database already set up ($CREDS_FILE exists)"
    echo -e "${GREEN}[SKIP]${NC} To re-run: sudo rm $CREDS_FILE && sudo ./scripts/setup-database.sh"
    exit 0
fi

# ── 1. Generate random password ─────────────────────────────────────────────
DB_PASSWORD=$(openssl rand -base64 32 | tr -d '/+=' | head -c 32)
echo -e "${GREEN}[OK]${NC} Generated random database password (32 chars)"

# ── 2. Create config directory ──────────────────────────────────────────────
mkdir -p "$CONFIG_DIR"
chmod 700 "$CONFIG_DIR"
chown root:wheel "$CONFIG_DIR" 2>/dev/null || chown root:root "$CONFIG_DIR"

# ── 3. Determine PostgreSQL superuser and how to run psql ───────────────────
# On macOS Homebrew, PostgreSQL runs as the developer's OS user (no 'postgres' OS user).
# On Linux, PostgreSQL typically runs as 'postgres'.
# We try multiple approaches to find a working psql connection.

# Pick the OS user that can run psql. macOS Homebrew installs PostgreSQL under
# the developer's account; Linux ships a dedicated 'postgres' OS user. Try the
# platform-preferred candidate first, then fall back through the others — but
# only after id(1) confirms the account exists and a real SELECT 1 succeeds.
pick_pg_run_user() {
    local candidates=()
    case "$(detect_platform)" in
        macos)
            candidates=("$DEV_USER" postgres root)
            ;;
        *)
            candidates=(postgres "$DEV_USER" root)
            ;;
    esac
    local candidate
    for candidate in "${candidates[@]}"; do
        [[ -z "$candidate" ]] && continue
        if id "$candidate" >/dev/null 2>&1 && \
           sudo -u "$candidate" psql -d postgres -c 'SELECT 1' >/dev/null 2>&1; then
            echo "$candidate"
            return 0
        fi
    done
    return 1
}

# Tentative pick used only for service-start commands (e.g. `brew services
# restart` below needs a real OS user). Re-validated via pick_pg_run_user once
# PostgreSQL is confirmed accepting connections.
case "$(detect_platform)" in
    macos) PG_RUN_USER="$DEV_USER" ;;
    *)     PG_RUN_USER="postgres" ;;
esac

# All commands connect to the 'postgres' default database first (the user's
# default database may not exist).  On macOS Homebrew the PG_RUN_USER is the
# developer who installed PostgreSQL.  On Linux it is typically 'postgres'.
run_psql() {
    sudo -u "$PG_RUN_USER" psql -d postgres -c "$1" 2>&1
    return $?
}

run_psql_db() {
    local db="$1"
    shift
    sudo -u "$PG_RUN_USER" psql -d "$db" -c "$1" 2>&1
    return $?
}

run_psql_file() {
    local db="$1"
    local file="$2"
    sudo -u "$PG_RUN_USER" psql -d "$db" -f "$file" 2>&1
    return $?
}

run_psql_query() {
    # tab-separated, unaligned output for machine-readable checks
    local sql="$1"
    sudo -u "$PG_RUN_USER" psql -d postgres -tA -c "$sql" 2>/dev/null
    return $?
}

db_exists() {
    sudo -u "$PG_RUN_USER" psql -d postgres -lqt 2>/dev/null | grep -qw "enforcer"
}

# ── 3a. Install PostgreSQL if not present ───────────────────────────────────
if ! command -v psql >/dev/null 2>&1; then
    echo -e "${YELLOW}[WARN]${NC} PostgreSQL not installed. Installing..."
    if command -v brew >/dev/null 2>&1; then
        sudo -u "$PG_RUN_USER" brew install postgresql@16 2>/dev/null \
            || sudo -u "$PG_RUN_USER" brew install postgresql@15 2>/dev/null \
            || sudo -u "$PG_RUN_USER" brew install postgresql 2>/dev/null \
            || true
    elif command -v apt-get >/dev/null 2>&1; then
        apt-get update -qq && apt-get install -y -qq postgresql >/dev/null 2>&1 || true
    fi
    if ! command -v psql >/dev/null 2>&1; then
        echo -e "${RED}[ERROR]${NC} Could not install PostgreSQL."
        exit 1
    fi
    echo -e "${GREEN}[OK]${NC} PostgreSQL installed"
fi

# ── 3b. Start PostgreSQL if not running ────────────────────────────────────
if ! pg_isready -q >/dev/null 2>&1; then
    echo -e "${YELLOW}[WARN]${NC} PostgreSQL not running. Starting..."
    if command -v brew >/dev/null 2>&1; then
        sudo -u "$PG_RUN_USER" brew services restart postgresql@16 2>/dev/null \
            || sudo -u "$PG_RUN_USER" brew services restart postgresql@15 2>/dev/null \
            || sudo -u "$PG_RUN_USER" brew services restart postgresql 2>/dev/null \
            || true
    elif command -v systemctl >/dev/null 2>&1; then
        systemctl restart postgresql 2>/dev/null || true
    fi

    # Wait for pg_isready (up to 15 seconds)
    echo -e "${YELLOW}[WAIT]${NC} Waiting for PostgreSQL to accept connections..."
    for i in 1 2 3 4 5; do
        if pg_isready -q >/dev/null 2>&1; then
            break
        fi
        sleep 3
        echo -e "${YELLOW}[WAIT]${NC} Retrying ($i/5)..."
    done

    if ! pg_isready -q >/dev/null 2>&1; then
        echo -e "${RED}[ERROR]${NC} PostgreSQL failed to start."
        echo "  Try manually: sudo -u $PG_RUN_USER brew services restart postgresql@15"
        exit 1
    fi
    echo -e "${GREEN}[OK]${NC} PostgreSQL started"
else
    echo -e "${GREEN}[OK]${NC} PostgreSQL already running"
fi

# ── 3c. Pick the OS user that can actually run psql ───────────────────────
if PICKED_USER=$(pick_pg_run_user); then
    PG_RUN_USER="$PICKED_USER"
    echo -e "${GREEN}[OK]${NC} PostgreSQL admin user: $PG_RUN_USER (platform: $(detect_platform))"
else
    echo -e "${RED}[ERROR]${NC} pg_isready says OK but no OS user can run psql."
    echo "  Tried: postgres, $DEV_USER, root"
    echo "  Manual check:  sudo -u postgres psql -c 'SELECT 1'"
    echo "  Force a user:  sudo ./scripts/setup-database.sh --env local_macos|local_ubuntu"
    exit 1
fi

# ── 4. Create the enforcer database user ──────────────────────────────────
# Create the role if it doesn't exist, or update its password if it does
# (handles re-deployment after a partial uninstall that left the role behind).
if [[ "$(run_psql_query "SELECT 1 FROM pg_roles WHERE rolname = 'enforcer' LIMIT 1;")" == "1" ]]; then
    run_psql "ALTER ROLE enforcer WITH LOGIN PASSWORD '$DB_PASSWORD';" >/dev/null 2>&1
    echo -e "${GREEN}[OK]${NC} Database user 'enforcer' exists — password updated"
else
    run_psql "CREATE ROLE enforcer WITH LOGIN PASSWORD '$DB_PASSWORD';" >/dev/null 2>&1
    echo -e "${GREEN}[OK]${NC} Database user 'enforcer' created"
fi

# ── 5. Create the database ─────────────────────────────────────────────────
if db_exists; then
    echo -e "${GREEN}[OK]${NC} Database 'enforcer' already exists"
else
    sudo -u "$PG_RUN_USER" createdb enforcer 2>/dev/null || true
    echo -e "${GREEN}[OK]${NC} Database 'enforcer' created"
fi

# ── 6. Apply schema with restricted grants ─────────────────────────────────
INIT_SQL_TEMP=$(mktemp /tmp/enforcer-init-XXXXXXXX)
chmod 644 "$INIT_SQL_TEMP"
sed "s/PLACEHOLDER_REPLACED_BY_INSTALLER/$DB_PASSWORD/g" "$INIT_SQL" > "$INIT_SQL_TEMP"
export PGOPTIONS="-c enforcer.developer_user=$DEV_USER"
if SCHEMA_OUTPUT=$(run_psql_file "enforcer" "$INIT_SQL_TEMP" 2>&1); then
    echo -e "${GREEN}[OK]${NC} Schema applied"
else
    echo -e "${RED}[ERROR]${NC} Schema apply failed:"
    echo "$SCHEMA_OUTPUT"
    rm -f "$INIT_SQL_TEMP"
    exit 1
fi
rm -f "$INIT_SQL_TEMP"

# ── 7. Verify and enforce grants ──────────────────────────────────────────
# The init.sql REVOKE/GRANT sequence can silently fail on macOS Homebrew
# where the developer user is the PostgreSQL superuser and table owner.
# Re-apply grants explicitly to guarantee the service user has access.
GRANT_SQL="
GRANT INSERT ON audit_events TO enforcer;
GRANT SELECT ON audit_events TO enforcer;
GRANT INSERT ON hub_policy_revisions TO enforcer;
GRANT SELECT ON hub_policy_revisions TO enforcer;
GRANT INSERT ON hub_client_snapshots TO enforcer;
GRANT SELECT ON hub_client_snapshots TO enforcer;
GRANT INSERT ON hub_enforcement_states TO enforcer;
GRANT SELECT ON hub_enforcement_states TO enforcer;
GRANT INSERT, UPDATE ON auth_secrets TO enforcer;
GRANT SELECT ON auth_secrets TO enforcer;
GRANT USAGE, SELECT ON SEQUENCE hub_policy_revisions_id_seq TO enforcer;
GRANT USAGE, SELECT ON SEQUENCE hub_client_snapshots_id_seq TO enforcer;
GRANT USAGE, SELECT ON SEQUENCE hub_enforcement_states_id_seq TO enforcer;
"
if GRANT_OUTPUT=$(run_psql_db "enforcer" "$GRANT_SQL" 2>&1); then
    echo -e "${GREEN}[OK]${NC} Database grants applied (INSERT + SELECT for enforcer on all tables)"
else
    echo -e "${RED}[ERROR]${NC} Failed to apply database grants:"
    echo "$GRANT_OUTPUT"
    echo "  Fix manually: psql -d enforcer and run the GRANT statements from docker/init.sql"
    exit 1
fi

# Verify grants actually work by testing INSERT + SELECT with the service user
if VERIFY_INSERT=$(PGPASSWORD="$DB_PASSWORD" psql -U enforcer -d enforcer -tA -c "INSERT INTO audit_events (id, event) VALUES (gen_random_uuid(), '{\"_verify\": true}'::jsonb) RETURNING id" 2>&1); then
    if VERIFY_SELECT=$(PGPASSWORD="$DB_PASSWORD" psql -U enforcer -d enforcer -tA -c "SELECT count(*) FROM audit_events WHERE event->>'_verify' = 'true'" 2>&1) && [[ "$VERIFY_SELECT" -ge 1 ]] 2>/dev/null; then
        echo -e "${GREEN}[OK]${NC} INSERT + SELECT verified for enforcer user"
    else
        echo -e "${RED}[ERROR]${NC} enforcer user can INSERT but SELECT returned: $VERIFY_SELECT"
        exit 1
    fi
    # Clean up verification row
    run_psql_db "enforcer" "DELETE FROM audit_events WHERE event->>'_verify' = 'true'" >/dev/null 2>&1
else
    echo -e "${RED}[ERROR]${NC} enforcer user cannot INSERT audit events:"
    echo "  $VERIFY_INSERT"
    exit 1
fi

# ── 8. Revoke developer's OS user access ────────────────────────────────────
if [[ -n "$DEV_USER" && "$DEV_USER" != "root" ]]; then
    if [[ "$(run_psql_query "SELECT 1 FROM pg_roles WHERE rolname = '$DEV_USER' LIMIT 1;")" == "1" ]]; then
        run_psql_db "enforcer" "REVOKE ALL ON DATABASE enforcer FROM \"$DEV_USER\";" >/dev/null 2>&1 || true
        run_psql_db "enforcer" "REVOKE ALL ON ALL TABLES IN SCHEMA public FROM \"$DEV_USER\";" >/dev/null 2>&1 || true
        echo -e "${GREEN}[OK]${NC} Revoked all database access from developer user '$DEV_USER'"
    else
        echo -e "${YELLOW}[WARN]${NC} PostgreSQL role '$DEV_USER' does not exist — per-user revoke skipped"
    fi
fi

# ── 9. Store credentials securely ───────────────────────────────────────────
DATABASE_URL="postgresql://enforcer:${DB_PASSWORD}@localhost:5432/enforcer?sslmode=prefer"
echo "$DATABASE_URL" > "$CREDS_FILE"
chmod 600 "$CREDS_FILE"
chown root:wheel "$CREDS_FILE" 2>/dev/null || chown root:root "$CREDS_FILE"
echo -e "${GREEN}[OK]${NC} Credentials stored in $CREDS_FILE (root:600 — developer cannot read)"

# ── 10. Verify developer cannot connect ─────────────────────────────────────
echo ""
echo "── Verification ──────────────────────────────────────────────────"
DEV_ACCESS_SUMMARY="unknown"
if [[ -n "$DEV_USER" && "$DEV_USER" != "root" ]]; then
    DEV_IS_SUPERUSER="false"
    if [[ "$(run_psql_query "SELECT rolsuper::int FROM pg_roles WHERE rolname = '$DEV_USER' LIMIT 1;")" == "1" ]]; then
        DEV_IS_SUPERUSER="true"
    fi

    if sudo -u "$DEV_USER" psql -d enforcer -c 'SELECT 1' >/dev/null 2>&1; then
        if [[ "$DEV_IS_SUPERUSER" == "true" ]]; then
            echo -e "${YELLOW}[WARN]${NC} Developer '$DEV_USER' CAN connect because their PostgreSQL role is SUPERUSER."
            echo "  Superusers bypass CONNECT revokes. Use a dedicated non-superuser postgres admin role on Hub hosts."
            DEV_ACCESS_SUMMARY="superuser_can_connect"
        else
            echo -e "${RED}[FAIL]${NC} Developer '$DEV_USER' CAN still connect to enforcer!"
            echo "  Manual fix: REVOKE CONNECT ON DATABASE enforcer FROM \"$DEV_USER\";"
            DEV_ACCESS_SUMMARY="can_connect"
        fi
    else
        echo -e "${GREEN}[PASS]${NC} Developer '$DEV_USER' CANNOT connect to enforcer"
        DEV_ACCESS_SUMMARY="blocked"
    fi
fi

# Verify enforcer user CAN connect
if PGPASSWORD="$DB_PASSWORD" psql -U enforcer -d enforcer -c "SELECT 1" >/dev/null 2>&1; then
    echo -e "${GREEN}[PASS]${NC} Service user 'enforcer' CAN connect"
else
    echo -e "${YELLOW}[WARN]${NC} Service user connection test inconclusive (pg_hba.conf may need md5/scram-sha-256)"
fi

# Verify DELETE is denied for enforcer user (INSERT + SELECT already verified in step 7)
if PGPASSWORD="$DB_PASSWORD" psql -U enforcer -d enforcer -c "DELETE FROM audit_events WHERE 1=0" >/dev/null 2>&1; then
    echo -e "${RED}[FAIL]${NC} Service user CAN delete — grants are incorrect!"
else
    echo -e "${GREEN}[PASS]${NC} Service user CANNOT delete audit events (append-only enforced)"
fi

echo ""
echo "── Summary ─────────────────────────────────────────────────────"
echo "  Database:     enforcer"
echo "  Service user: enforcer (INSERT + SELECT only)"
case "$DEV_ACCESS_SUMMARY" in
  blocked)
    echo "  Developer:    ${DEV_USER:-unknown} (NO access)"
    ;;
  superuser_can_connect)
    echo "  Developer:    ${DEV_USER:-unknown} (CAN connect as PostgreSQL superuser)"
    ;;
  can_connect)
    echo "  Developer:    ${DEV_USER:-unknown} (CAN connect — fix required)"
    ;;
  *)
    echo "  Developer:    ${DEV_USER:-unknown} (access check unavailable)"
    ;;
esac
echo "  Credentials:  $CREDS_FILE (root:600)"
echo "  DATABASE_URL: Set in Sentinel Server LaunchDaemon plist environment"
echo ""
DB_SETUP_SUCCEEDED=true
banner "Database Setup Complete"
