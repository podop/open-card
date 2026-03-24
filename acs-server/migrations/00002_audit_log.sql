-- +goose Up
-- +goose StatementBegin

-- ─────────────────────────────────────────────────────────────────
-- audit_log: immutable record of authentication decisions
-- ─────────────────────────────────────────────────────────────────
CREATE TABLE audit_log (
    id                  BIGSERIAL    PRIMARY KEY,
    acs_transaction_id  UUID         NOT NULL,
    ds_transaction_id   UUID         NOT NULL,
    event_type          VARCHAR(30)  NOT NULL, -- 'auth_frictionless' | 'auth_challenge_start'
                                               -- | 'challenge_success' | 'challenge_fail'
                                               -- | 'auth_unavailable' | 'method_collected'
    trans_status        CHAR(1),
    card_bin            VARCHAR(8),
    merchant_country    CHAR(3),
    device_channel      CHAR(2),
    spec_version        VARCHAR(10),
    detail              JSONB,                 -- arbitrary extra context
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_log_acs_trans_id ON audit_log (acs_transaction_id);
CREATE INDEX idx_audit_log_created_at  ON audit_log (created_at DESC);

-- ─────────────────────────────────────────────────────────────────
-- Scheduled cleanup: expire stale pending sessions (run via pg_cron
-- or an external job; this just adds the helper function).
-- ─────────────────────────────────────────────────────────────────
CREATE OR REPLACE FUNCTION expire_stale_sessions()
RETURNS INTEGER AS $$
DECLARE
    rows_updated INTEGER;
BEGIN
    UPDATE acs_sessions
    SET    status = 'expired'
    WHERE  status    = 'pending'
      AND  expires_at < NOW();
    GET DIAGNOSTICS rows_updated = ROW_COUNT;
    RETURN rows_updated;
END;
$$ LANGUAGE plpgsql;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS expire_stale_sessions();
DROP TABLE  IF EXISTS audit_log;
-- +goose StatementEnd
