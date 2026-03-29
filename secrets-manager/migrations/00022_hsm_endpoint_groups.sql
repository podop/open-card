-- +goose Up

-- Create endpoint groups table
CREATE TABLE IF NOT EXISTS sm_hsm_endpoint_groups (
    id          SERIAL PRIMARY KEY,
    name        TEXT UNIQUE NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    strategy    TEXT NOT NULL DEFAULT 'round-robin' CHECK (strategy IN ('round-robin','capability','weighted')),
    enabled     BOOLEAN NOT NULL DEFAULT true,
    created_by  INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Add group_id FK to endpoints
ALTER TABLE sm_hsm_endpoints ADD COLUMN group_id INT REFERENCES sm_hsm_endpoint_groups(id) ON DELETE SET NULL;
CREATE INDEX idx_hsm_endpoints_group ON sm_hsm_endpoints(group_id);

-- Replace session_id with group_id in audit log
ALTER TABLE sm_hsm_audit_log DROP CONSTRAINT IF EXISTS sm_hsm_audit_log_session_id_fkey;
ALTER TABLE sm_hsm_audit_log RENAME COLUMN session_id TO group_id;
ALTER TABLE sm_hsm_audit_log ALTER COLUMN group_id TYPE INT USING NULL;
ALTER TABLE sm_hsm_audit_log ADD CONSTRAINT sm_hsm_audit_log_group_id_fkey FOREIGN KEY (group_id) REFERENCES sm_hsm_endpoint_groups(id);

-- Drop sessions table
DROP INDEX IF EXISTS idx_hsm_sessions_status;
DROP TABLE IF EXISTS sm_hsm_sessions;

-- +goose Down

-- Recreate sessions table
CREATE TABLE IF NOT EXISTS sm_hsm_sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    endpoint_id     INT REFERENCES sm_hsm_endpoints(id),
    client_label    TEXT NOT NULL DEFAULT '',
    strategy        TEXT NOT NULL DEFAULT 'round-robin' CHECK (strategy IN ('round-robin','capability','weighted','sticky')),
    status          TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','idle','closed')),
    last_ping       TIMESTAMPTZ NOT NULL DEFAULT now(),
    ttl_seconds     INT NOT NULL DEFAULT 300,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at       TIMESTAMPTZ
);
CREATE INDEX idx_hsm_sessions_status ON sm_hsm_sessions(status) WHERE status = 'active';

-- Revert audit log
ALTER TABLE sm_hsm_audit_log DROP CONSTRAINT IF EXISTS sm_hsm_audit_log_group_id_fkey;
ALTER TABLE sm_hsm_audit_log RENAME COLUMN group_id TO session_id;
ALTER TABLE sm_hsm_audit_log ALTER COLUMN session_id TYPE UUID USING NULL;
ALTER TABLE sm_hsm_audit_log ADD CONSTRAINT sm_hsm_audit_log_session_id_fkey FOREIGN KEY (session_id) REFERENCES sm_hsm_sessions(id);

-- Remove group_id from endpoints
DROP INDEX IF EXISTS idx_hsm_endpoints_group;
ALTER TABLE sm_hsm_endpoints DROP COLUMN IF EXISTS group_id;

-- Drop endpoint groups table
DROP TABLE IF EXISTS sm_hsm_endpoint_groups;