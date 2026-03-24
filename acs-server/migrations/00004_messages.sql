-- +goose Up
CREATE TABLE messages (
    id                 BIGSERIAL    PRIMARY KEY,
    acs_transaction_id UUID         NOT NULL,
    message_type       VARCHAR(10)  NOT NULL,  -- AReq, ARes, CReq, CRes, RReq, RRes, Erro
    direction          VARCHAR(10)  NOT NULL,  -- inbound | outbound
    content            JSONB        NOT NULL,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_messages_acs_tx ON messages (acs_transaction_id);

-- +goose Down
DROP TABLE messages;