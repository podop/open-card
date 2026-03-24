-- +goose Up
ALTER TABLE test_scenarios ALTER COLUMN flow_type TYPE VARCHAR(30);
ALTER TABLE scenario_runs  ALTER COLUMN flow_type TYPE VARCHAR(30);

-- +goose Down
ALTER TABLE test_scenarios ALTER COLUMN flow_type TYPE VARCHAR(20);
ALTER TABLE scenario_runs  ALTER COLUMN flow_type TYPE VARCHAR(20);
