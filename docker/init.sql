-- Enforcer — PostgreSQL Initialization (Strict Mode)
-- Author: Deepankar Das
--
-- Creates a dedicated 'enforcer' database user with restricted grants.
-- The developer's OS user has NO access to this database.
-- Audit events are append-only: INSERT + SELECT only, no UPDATE or DELETE.
--
-- This script must be run as the PostgreSQL superuser (postgres) by the
-- installation script (enforcer_deploy.sh) under sudo.

-- ============================================================================
-- 1. Create restricted database user (if not exists)
-- ============================================================================
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'enforcer') THEN
        CREATE ROLE enforcer WITH LOGIN PASSWORD 'PLACEHOLDER_REPLACED_BY_INSTALLER';
    END IF;
END
$$;

-- ============================================================================
-- 2. Revoke all public access to the database
-- ============================================================================
REVOKE ALL ON DATABASE enforcer FROM PUBLIC;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;

-- Grant connect only to the enforcer user
GRANT CONNECT ON DATABASE enforcer TO enforcer;
GRANT USAGE ON SCHEMA public TO enforcer;

-- ============================================================================
-- 3. Create the audit_events table (append-only)
-- ============================================================================
CREATE TABLE IF NOT EXISTS audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event JSONB NOT NULL,
    session_id TEXT GENERATED ALWAYS AS (event->>'session_id') STORED,
    action_type TEXT GENERATED ALWAYS AS (event->'action'->>'type') STORED,
    decision TEXT GENERATED ALWAYS AS (event->'policy_detail'->>'decision') STORED,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================================
-- 3b. Create Management Hub state tables (append-only revisions/snapshots)
-- ============================================================================
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

-- ============================================================================
-- 4. Indexes for common query patterns (TDD Section 9.4)
-- ============================================================================
CREATE INDEX IF NOT EXISTS idx_audit_session ON audit_events (session_id);
CREATE INDEX IF NOT EXISTS idx_audit_action_type ON audit_events (action_type);
CREATE INDEX IF NOT EXISTS idx_audit_decision ON audit_events (decision);
CREATE INDEX IF NOT EXISTS idx_audit_created_at ON audit_events (created_at);
CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_events ((event->'actor'->>'user_id'));
CREATE INDEX IF NOT EXISTS idx_audit_event_gin ON audit_events USING GIN (event);
CREATE INDEX IF NOT EXISTS idx_hub_policy_created_at ON hub_policy_revisions (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_hub_policy_hash ON hub_policy_revisions (hash);
CREATE INDEX IF NOT EXISTS idx_hub_client_id_created ON hub_client_snapshots (client_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_hub_client_last_heartbeat ON hub_client_snapshots (last_heartbeat DESC);
CREATE INDEX IF NOT EXISTS idx_hub_enforcement_changed_at ON hub_enforcement_states (changed_at DESC);
CREATE INDEX IF NOT EXISTS idx_auth_secrets_updated_at ON auth_secrets (updated_at DESC);

-- ============================================================================
-- 5. Enforce append-only at the DATABASE LEVEL (not just application level)
-- ============================================================================

-- Revoke everything from the enforcer user first
REVOKE ALL ON audit_events FROM enforcer;

-- Grant INSERT only (daemon writes new events)
GRANT INSERT ON audit_events TO enforcer;

-- Grant SELECT only (daemon reads for queries/export)
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

-- NO UPDATE — events are immutable after insertion
-- NO DELETE — events cannot be removed
-- NO TRUNCATE — table cannot be cleared

-- Explicitly deny the developer's OS user (if they somehow get a psql session)
-- The REVOKE ALL FROM PUBLIC above already handles this, but be explicit:
DO $$
DECLARE
    dev_user TEXT;
BEGIN
    -- Get the OS user who invoked sudo (the developer)
    dev_user := current_setting('enforcer.developer_user', true);
    IF dev_user IS NOT NULL AND dev_user != '' AND dev_user != 'postgres' THEN
        EXECUTE format('REVOKE ALL ON DATABASE enforcer FROM %I', dev_user);
        EXECUTE format('REVOKE ALL ON ALL TABLES IN SCHEMA public FROM %I', dev_user);
    END IF;
EXCEPTION WHEN OTHERS THEN
    -- If the setting doesn't exist or the user doesn't exist, skip silently
    NULL;
END
$$;

-- ============================================================================
-- 6. Prevent the enforcer user from creating new tables or modifying schema
-- ============================================================================
REVOKE CREATE ON SCHEMA public FROM enforcer;

-- ============================================================================
-- 7. Metadata
-- ============================================================================
COMMENT ON TABLE audit_events IS 'Append-only audit event store. INSERT + SELECT only. No UPDATE, DELETE, or TRUNCATE. Developer has no access.';
COMMENT ON DATABASE enforcer IS 'Enforcer audit database. Restricted access: only the enforcer service user can connect. Developer OS user is denied.';
