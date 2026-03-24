-- +goose Up
ALTER TABLE messages ALTER COLUMN acs_transaction_id DROP NOT NULL;

-- +goose Down
UPDATE messages SET acs_transaction_id = '00000000-0000-0000-0000-000000000000' WHERE acs_transaction_id IS NULL;
ALTER TABLE messages ALTER COLUMN acs_transaction_id SET NOT NULL;
