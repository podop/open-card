-- +goose Up
-- +goose StatementBegin

-- ─────────────────────────────────────────────────────────────────
-- mfa_providers: managed MFA provider registry (internal + external)
-- ─────────────────────────────────────────────────────────────────
CREATE TABLE mfa_providers (
    id              BIGSERIAL    PRIMARY KEY,
    name            VARCHAR(50)  NOT NULL UNIQUE,
    display_name    VARCHAR(100) NOT NULL,
    type            VARCHAR(20)  NOT NULL,  -- 'internal' | 'external'
    factor_type     VARCHAR(30)  NOT NULL,  -- 'card_pin' | 'authenticator' | 'duo' etc.
    active          BOOLEAN      NOT NULL DEFAULT FALSE,
    config          JSONB        NOT NULL DEFAULT '{}',
    description     TEXT         NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TRIGGER trg_mfa_providers_updated_at
    BEFORE UPDATE ON mfa_providers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Seed built-in Card PIN provider
INSERT INTO mfa_providers (name, display_name, type, factor_type, active, description) VALUES
    ('card_pin', 'Card PIN Verification', 'internal', 'card_pin', FALSE, 'Built-in second factor: cardholder enters their card PIN code');

-- Add second factor columns to network_rules
ALTER TABLE network_rules ADD COLUMN second_factor_mode VARCHAR(20) NOT NULL DEFAULT 'none';
ALTER TABLE network_rules ADD COLUMN mfa_provider_id BIGINT REFERENCES mfa_providers(id);

-- Add second factor columns to bin_exclusions
ALTER TABLE bin_exclusions ADD COLUMN second_factor_mode VARCHAR(20) NOT NULL DEFAULT 'none';
ALTER TABLE bin_exclusions ADD COLUMN mfa_provider_id BIGINT REFERENCES mfa_providers(id);

-- Add second factor columns to transactions
ALTER TABLE transactions ADD COLUMN second_factor_required BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE transactions ADD COLUMN pin_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE transactions ADD COLUMN pin_expiry TIMESTAMPTZ;
ALTER TABLE transactions ADD COLUMN pin_attempts SMALLINT NOT NULL DEFAULT 0;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE transactions DROP COLUMN IF EXISTS pin_attempts;
ALTER TABLE transactions DROP COLUMN IF EXISTS pin_expiry;
ALTER TABLE transactions DROP COLUMN IF EXISTS pin_hash;
ALTER TABLE transactions DROP COLUMN IF EXISTS second_factor_required;
ALTER TABLE bin_exclusions DROP COLUMN IF EXISTS mfa_provider_id;
ALTER TABLE bin_exclusions DROP COLUMN IF EXISTS second_factor_mode;
ALTER TABLE network_rules DROP COLUMN IF EXISTS mfa_provider_id;
ALTER TABLE network_rules DROP COLUMN IF EXISTS second_factor_mode;
DROP TRIGGER IF EXISTS trg_mfa_providers_updated_at ON mfa_providers;
DROP TABLE IF EXISTS mfa_providers;
-- +goose StatementEnd
