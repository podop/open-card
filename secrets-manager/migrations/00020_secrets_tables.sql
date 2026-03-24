-- +goose Up

CREATE TABLE IF NOT EXISTS sm_security_providers (
    id              SERIAL PRIMARY KEY,
    name            TEXT UNIQUE NOT NULL,
    provider_type   TEXT NOT NULL CHECK (provider_type IN ('internal', 'hsm', 'ca')),
    config          JSONB NOT NULL DEFAULT '{}',
    active          BOOLEAN NOT NULL DEFAULT true,
    created_by      INT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sm_rotation_policies (
    id                  SERIAL PRIMARY KEY,
    name                TEXT UNIQUE NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    key_type            TEXT NOT NULL,
    rotation_interval   INTERVAL NOT NULL,
    notify_before       INTERVAL NOT NULL DEFAULT '30 days',
    auto_rotate         BOOLEAN NOT NULL DEFAULT false,
    is_template         BOOLEAN NOT NULL DEFAULT false,
    created_by          INT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sm_keys (
    id                  SERIAL PRIMARY KEY,
    uid                 UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    name                TEXT NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    key_type            TEXT NOT NULL CHECK (key_type IN (
        'RSA-1024','RSA-2048','RSA-3072','RSA-4096',
        'AES-128','AES-192','AES-256','3DES','X509'
    )),
    key_usage           TEXT NOT NULL DEFAULT 'general' CHECK (key_usage IN (
        'general','signing','encryption','tls','authentication'
    )),
    provider_id         INT NOT NULL REFERENCES sm_security_providers(id),
    public_key_pem      TEXT,
    private_key_enc     TEXT,
    certificate_pem     TEXT,
    hsm_key_handle      TEXT,
    fingerprint         TEXT NOT NULL,
    key_size_bits       INT NOT NULL,
    not_before          TIMESTAMPTZ,
    not_after           TIMESTAMPTZ,
    rotation_policy_id  INT REFERENCES sm_rotation_policies(id),
    last_rotated_at     TIMESTAMPTZ,
    next_rotation_at    TIMESTAMPTZ,
    version             INT NOT NULL DEFAULT 1,
    predecessor_id      INT REFERENCES sm_keys(id),
    owner_user_id       INT NOT NULL,
    active              BOOLEAN NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sm_keys_uid ON sm_keys(uid);
CREATE INDEX idx_sm_keys_type ON sm_keys(key_type);
CREATE INDEX idx_sm_keys_next_rotation ON sm_keys(next_rotation_at) WHERE active = true;

CREATE TABLE IF NOT EXISTS sm_key_access (
    id          SERIAL PRIMARY KEY,
    key_id      INT NOT NULL REFERENCES sm_keys(id) ON DELETE CASCADE,
    user_id     INT NOT NULL,
    permission  TEXT NOT NULL CHECK (permission IN ('use', 'manage')),
    granted_by  INT NOT NULL,
    granted_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (key_id, user_id)
);

CREATE TABLE IF NOT EXISTS sm_key_usage_log (
    id          BIGSERIAL PRIMARY KEY,
    key_id      INT NOT NULL REFERENCES sm_keys(id) ON DELETE CASCADE,
    user_id     INT NOT NULL,
    action      TEXT NOT NULL,
    ip_address  TEXT,
    user_agent  TEXT,
    details     JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sm_usage_log_key ON sm_key_usage_log(key_id);
CREATE INDEX idx_sm_usage_log_time ON sm_key_usage_log(created_at);

CREATE TABLE IF NOT EXISTS sm_rotation_history (
    id              BIGSERIAL PRIMARY KEY,
    key_id          INT NOT NULL REFERENCES sm_keys(id) ON DELETE CASCADE,
    old_version     INT NOT NULL,
    new_version     INT NOT NULL,
    rotated_by      INT,
    trigger         TEXT NOT NULL CHECK (trigger IN ('manual', 'scheduled', 'emergency')),
    status          TEXT NOT NULL CHECK (status IN ('success', 'failed')),
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS sm_rotation_history;
DROP TABLE IF EXISTS sm_key_usage_log;
DROP TABLE IF EXISTS sm_key_access;
DROP TABLE IF EXISTS sm_keys;
DROP TABLE IF EXISTS sm_rotation_policies;
DROP TABLE IF EXISTS sm_security_providers;
