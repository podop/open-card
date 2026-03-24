-- +goose Up
-- +goose StatementBegin

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ─────────────────────────────────────────────────────────────────
-- card_ranges: BIN ranges registered for 3DS authentication
-- ─────────────────────────────────────────────────────────────────
CREATE TABLE card_ranges (
    id                BIGSERIAL PRIMARY KEY,
    start_range       VARCHAR(19)  NOT NULL,
    end_range         VARCHAR(19)  NOT NULL,
    acs_url_method    TEXT         NOT NULL DEFAULT '',
    acs_url_challenge TEXT         NOT NULL DEFAULT '',
    min_protocol      VARCHAR(10)  NOT NULL DEFAULT '2.2.0',
    max_protocol      VARCHAR(10)  NOT NULL DEFAULT '2.3.1',
    action_ind        CHAR(1)      NOT NULL DEFAULT 'A', -- A=Add M=Modify D=Delete
    active            BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_card_range UNIQUE (start_range, end_range)
);

CREATE INDEX idx_card_ranges_active ON card_ranges (active, start_range, end_range);

-- ─────────────────────────────────────────────────────────────────
-- transactions: core 3DS authentication lifecycle
-- ─────────────────────────────────────────────────────────────────
CREATE TABLE transactions (
    id                      UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    acs_transaction_id      UUID         NOT NULL UNIQUE,
    ds_transaction_id       UUID         NOT NULL,
    server_transaction_id   UUID         NOT NULL,
    sdk_transaction_id      UUID,

    spec_version            VARCHAR(10)  NOT NULL,
    message_category        CHAR(2)      NOT NULL,  -- '01'=PA '02'=NPA
    device_channel          CHAR(2)      NOT NULL,  -- '01'=App '02'=Browser '03'=3RI

    -- Card data (PAN stored AES-256-GCM encrypted, hex-encoded)
    acct_number             TEXT         NOT NULL,
    card_bin                VARCHAR(8)   NOT NULL,

    -- Merchant
    merchant_name           VARCHAR(40)  NOT NULL,
    merchant_country        CHAR(3)      NOT NULL,
    merchant_id             VARCHAR(35)  NOT NULL DEFAULT '',

    -- Purchase (nullable for NPA)
    purchase_amount         VARCHAR(48),
    purchase_currency       CHAR(3),
    purchase_exponent       SMALLINT,

    -- Authentication outcome
    trans_status            CHAR(1)      NOT NULL DEFAULT 'U',
    auth_value              TEXT,        -- CAVV / Authentication Value (base64)
    eci                     CHAR(2),

    -- Challenge state
    challenge_attempts      SMALLINT     NOT NULL DEFAULT 0,
    otp_hash                TEXT         NOT NULL DEFAULT '',
    otp_expiry              TIMESTAMPTZ,

    -- Timestamps
    created_at              TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    completed_at            TIMESTAMPTZ
);

CREATE INDEX idx_transactions_acs_trans_id    ON transactions (acs_transaction_id);
CREATE INDEX idx_transactions_server_trans_id ON transactions (server_transaction_id);
CREATE INDEX idx_transactions_ds_trans_id     ON transactions (ds_transaction_id);
CREATE INDEX idx_transactions_created_at      ON transactions (created_at DESC);

-- ─────────────────────────────────────────────────────────────────
-- acs_sessions: 3DS Method and challenge session state
-- ─────────────────────────────────────────────────────────────────
CREATE TABLE acs_sessions (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id  UUID         NOT NULL,   -- references server_transaction_id or acs_transaction_id
    session_type    VARCHAR(10)  NOT NULL,   -- 'method' | 'challenge'
    status          VARCHAR(10)  NOT NULL DEFAULT 'pending',
    notif_url       TEXT         NOT NULL DEFAULT '',
    device_data     BYTEA,                   -- encrypted JSON blob of browser/device fingerprint
    expires_at      TIMESTAMPTZ  NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_acs_sessions_transaction_id ON acs_sessions (transaction_id, session_type, status);
CREATE INDEX idx_acs_sessions_expires_at     ON acs_sessions (expires_at);

-- ─────────────────────────────────────────────────────────────────
-- Automatic updated_at trigger helper
-- ─────────────────────────────────────────────────────────────────
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_transactions_updated_at
    BEFORE UPDATE ON transactions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_transactions_updated_at ON transactions;
DROP FUNCTION IF EXISTS set_updated_at();
DROP TABLE IF EXISTS acs_sessions;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS card_ranges;
-- +goose StatementEnd
