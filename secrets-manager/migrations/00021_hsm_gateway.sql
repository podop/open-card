-- +goose Up

CREATE TABLE IF NOT EXISTS sm_hsm_endpoints (
    id                   SERIAL PRIMARY KEY,
    name                 TEXT UNIQUE NOT NULL,
    model                TEXT NOT NULL,
    address              TEXT NOT NULL,
    port                 INT NOT NULL DEFAULT 1500,
    protocol             TEXT NOT NULL DEFAULT 'TCP' CHECK (protocol IN ('TCP','TLS','UDP')),
    header_length        INT NOT NULL DEFAULT 4,
    firmware_version     TEXT NOT NULL DEFAULT '',
    host_command_timeout INT NOT NULL DEFAULT 15,
    connection_timeout   INT NOT NULL DEFAULT 10,
    max_connections      INT NOT NULL DEFAULT 16,
    lmk_identifier       INT NOT NULL DEFAULT 0,
    config               JSONB NOT NULL DEFAULT '{}',
    weight               INT NOT NULL DEFAULT 100,
    priority             INT NOT NULL DEFAULT 0,
    status               TEXT NOT NULL DEFAULT 'offline' CHECK (status IN ('online','offline','degraded','draining')),
    capabilities         TEXT[] NOT NULL DEFAULT '{}',
    enabled              BOOLEAN NOT NULL DEFAULT true,
    last_health_check    TIMESTAMPTZ,
    last_health_ok       BOOLEAN NOT NULL DEFAULT false,
    error_message        TEXT,
    created_by           INT NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

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

CREATE TABLE IF NOT EXISTS sm_hsm_audit_log (
    id              BIGSERIAL PRIMARY KEY,
    session_id      UUID REFERENCES sm_hsm_sessions(id),
    endpoint_id     INT REFERENCES sm_hsm_endpoints(id),
    command_code    TEXT NOT NULL,
    api_operation   TEXT NOT NULL,
    request_size    INT NOT NULL DEFAULT 0,
    response_size   INT NOT NULL DEFAULT 0,
    response_code   TEXT,
    duration_ms     INT NOT NULL DEFAULT 0,
    error_message   TEXT,
    user_id         INT NOT NULL DEFAULT 0,
    ip_address      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_hsm_audit_time ON sm_hsm_audit_log(created_at);
CREATE INDEX idx_hsm_audit_endpoint ON sm_hsm_audit_log(endpoint_id);

CREATE TABLE IF NOT EXISTS sm_hsm_metrics (
    id              BIGSERIAL PRIMARY KEY,
    endpoint_id     INT NOT NULL REFERENCES sm_hsm_endpoints(id),
    period_start    TIMESTAMPTZ NOT NULL,
    period_end      TIMESTAMPTZ NOT NULL,
    total_requests  INT NOT NULL DEFAULT 0,
    success_count   INT NOT NULL DEFAULT 0,
    error_count     INT NOT NULL DEFAULT 0,
    avg_latency_ms  FLOAT NOT NULL DEFAULT 0,
    p99_latency_ms  FLOAT NOT NULL DEFAULT 0,
    commands        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_hsm_metrics_endpoint ON sm_hsm_metrics(endpoint_id);
CREATE INDEX idx_hsm_metrics_period ON sm_hsm_metrics(period_start);

-- +goose Down
DROP TABLE IF EXISTS sm_hsm_metrics;
DROP TABLE IF EXISTS sm_hsm_audit_log;
DROP TABLE IF EXISTS sm_hsm_sessions;
DROP TABLE IF EXISTS sm_hsm_endpoints;
