-- +goose Up
-- +goose StatementBegin

-- Add network identifier to card_ranges so each BIN range knows its card scheme.
ALTER TABLE card_ranges
    ADD COLUMN network VARCHAR(20) NOT NULL DEFAULT 'unknown';

-- Add network to transactions for audit / reporting (copied from card_range at auth time).
ALTER TABLE transactions
    ADD COLUMN network VARCHAR(20) NOT NULL DEFAULT 'unknown';

CREATE INDEX idx_card_ranges_network ON card_ranges (network);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_card_ranges_network;
ALTER TABLE transactions DROP COLUMN IF EXISTS network;
ALTER TABLE card_ranges  DROP COLUMN IF EXISTS network;

-- +goose StatementEnd