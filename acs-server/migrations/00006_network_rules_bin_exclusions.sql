-- +goose Up
-- +goose StatementBegin

-- ─────────────────────────────────────────────────────────────────
-- network_rules: per-network default flow-control settings
-- Applied to every transaction on that network unless a BIN exclusion overrides.
-- ─────────────────────────────────────────────────────────────────
CREATE TABLE network_rules (
    id                      BIGSERIAL    PRIMARY KEY,
    network                 VARCHAR(20)  NOT NULL UNIQUE,   -- visa | mastercard | amex | discover | unknown
    allow_frictionless      BOOLEAN      NOT NULL DEFAULT TRUE,
    allow_otp               BOOLEAN      NOT NULL DEFAULT TRUE,
    allow_oob               BOOLEAN      NOT NULL DEFAULT TRUE,
    default_challenge_channel VARCHAR(10) NOT NULL DEFAULT 'otp', -- 'otp' | 'oob'
    created_at              TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TRIGGER trg_network_rules_updated_at
    BEFORE UPDATE ON network_rules
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Seed default rules for known networks
INSERT INTO network_rules (network, allow_frictionless, allow_otp, allow_oob, default_challenge_channel) VALUES
    ('visa',       TRUE,  TRUE, TRUE,  'otp'),
    ('mastercard', TRUE,  TRUE, TRUE,  'otp'),
    ('amex',       FALSE, TRUE, FALSE, 'otp'),
    ('discover',   TRUE,  TRUE, TRUE,  'otp'),
    ('unknown',    FALSE, TRUE, FALSE, 'otp');

-- ─────────────────────────────────────────────────────────────────
-- bin_exclusions: per-BIN range overrides that bypass the network default.
-- When a card's BIN falls in an exclusion range, the exclusion settings
-- are used instead of the network_rules defaults.
-- ─────────────────────────────────────────────────────────────────
CREATE TABLE bin_exclusions (
    id                      BIGSERIAL    PRIMARY KEY,
    network                 VARCHAR(20)  NOT NULL,
    start_range             VARCHAR(19)  NOT NULL,
    end_range               VARCHAR(19)  NOT NULL,
    description             TEXT         NOT NULL DEFAULT '',
    allow_frictionless      BOOLEAN      NOT NULL DEFAULT TRUE,
    allow_otp               BOOLEAN      NOT NULL DEFAULT TRUE,
    allow_oob               BOOLEAN      NOT NULL DEFAULT TRUE,
    default_challenge_channel VARCHAR(10) NOT NULL DEFAULT 'otp',
    active                  BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at              TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_bin_exclusion UNIQUE (network, start_range, end_range)
);

CREATE INDEX idx_bin_exclusions_lookup ON bin_exclusions (network, active, start_range, end_range);

CREATE TRIGGER trg_bin_exclusions_updated_at
    BEFORE UPDATE ON bin_exclusions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_bin_exclusions_updated_at ON bin_exclusions;
DROP TABLE IF EXISTS bin_exclusions;
DROP TRIGGER IF EXISTS trg_network_rules_updated_at ON network_rules;
DROP TABLE IF EXISTS network_rules;
-- +goose StatementEnd