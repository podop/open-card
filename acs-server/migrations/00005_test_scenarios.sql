-- +goose Up

CREATE TABLE test_scenarios (
    id          BIGSERIAL PRIMARY KEY,
    name        VARCHAR(100)  NOT NULL,
    description TEXT          NOT NULL DEFAULT '',
    flow_type   VARCHAR(20)   NOT NULL, -- frictionless, challenge_otp, challenge_oob, decoupled
    device_channel  CHAR(2)   NOT NULL DEFAULT '02', -- 01=App, 02=Browser, 03=3RI
    network     VARCHAR(20)   NOT NULL DEFAULT 'visa',
    card_number VARCHAR(19)   NOT NULL DEFAULT '4000000000001000',
    merchant_name   VARCHAR(40)  NOT NULL DEFAULT 'Test Merchant',
    merchant_country CHAR(3)  NOT NULL DEFAULT '840',
    purchase_amount  VARCHAR(20) NOT NULL DEFAULT '1000',
    purchase_currency VARCHAR(3) NOT NULL DEFAULT '840',
    challenge_action VARCHAR(20) NOT NULL DEFAULT 'submit_otp', -- submit_otp, cancel, timeout, oob_approve, oob_deny, oob_fallback
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE TABLE scenario_runs (
    id          BIGSERIAL PRIMARY KEY,
    scenario_id BIGINT        NOT NULL REFERENCES test_scenarios(id) ON DELETE CASCADE,
    scenario_name VARCHAR(100) NOT NULL DEFAULT '',
    flow_type   VARCHAR(20)   NOT NULL,
    acs_transaction_id UUID,
    status      VARCHAR(20)   NOT NULL DEFAULT 'pending', -- pending, running, completed, failed
    steps       JSONB         NOT NULL DEFAULT '[]',
    error_msg   TEXT          NOT NULL DEFAULT '',
    duration_ms INTEGER       NOT NULL DEFAULT 0,
    started_at  TIMESTAMPTZ   NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_scenario_runs_scenario_id ON scenario_runs(scenario_id);
CREATE INDEX idx_scenario_runs_status ON scenario_runs(status);

-- +goose Down
DROP TABLE IF EXISTS scenario_runs;
DROP TABLE IF EXISTS test_scenarios;