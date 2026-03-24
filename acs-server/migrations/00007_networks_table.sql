-- +goose Up
-- +goose StatementBegin

-- ─────────────────────────────────────────────────────────────────
-- networks: master table for all card networks / payment schemes.
-- Previously hardcoded as Go constants and JS arrays — now DB-driven.
-- ─────────────────────────────────────────────────────────────────
CREATE TABLE networks (
    id              BIGSERIAL    PRIMARY KEY,
    name            VARCHAR(40)  NOT NULL UNIQUE, -- slug: visa, mastercard, amex, etc.
    display_name    VARCHAR(80)  NOT NULL,        -- human-readable: Visa, Mastercard, etc.
    active          BOOLEAN      NOT NULL DEFAULT TRUE,
    color           VARCHAR(10)  NOT NULL DEFAULT '#555d70',  -- brand color hex for UI charts
    eci_full_auth   VARCHAR(4)   NOT NULL DEFAULT '05',       -- ECI for fully authenticated
    eci_attempt     VARCHAR(4)   NOT NULL DEFAULT '06',       -- ECI for attempt processing
    eci_not_auth    VARCHAR(4)   NOT NULL DEFAULT '07',       -- ECI for not authenticated
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TRIGGER trg_networks_updated_at
    BEFORE UPDATE ON networks
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Seed with existing networks
INSERT INTO networks (name, display_name, active, color, eci_full_auth, eci_attempt, eci_not_auth) VALUES
    ('visa',       'Visa',       TRUE, '#1A1F71', '05', '06', '07'),
    ('mastercard', 'Mastercard', TRUE, '#EB001B', '02', '01', '00'),
    ('amex',       'Amex',       TRUE, '#006FCF', '05', '06', '07'),
    ('discover',   'Discover',   TRUE, '#FF6600', '05', '06', '07'),
    ('unknown',    'Unknown',    TRUE, '#555d70', '05', '06', '07');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_networks_updated_at ON networks;
DROP TABLE IF EXISTS networks;
-- +goose StatementEnd