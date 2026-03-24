-- +goose Up

CREATE TABLE IF NOT EXISTS idp_settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS idp_users (
    id            SERIAL PRIMARY KEY,
    username      TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    display_name  TEXT NOT NULL DEFAULT '',
    active        BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS idp_groups (
    id          SERIAL PRIMARY KEY,
    name        TEXT UNIQUE NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    actions     TEXT[] NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS idp_user_groups (
    user_id  INT REFERENCES idp_users(id) ON DELETE CASCADE,
    group_id INT REFERENCES idp_groups(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, group_id)
);

CREATE TABLE IF NOT EXISTS idp_refresh_tokens (
    id         TEXT PRIMARY KEY,
    user_id    INT NOT NULL REFERENCES idp_users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_idp_refresh_tokens_user ON idp_refresh_tokens(user_id);

-- +goose Down
DROP TABLE IF EXISTS idp_refresh_tokens;
DROP TABLE IF EXISTS idp_user_groups;
DROP TABLE IF EXISTS idp_groups;
DROP TABLE IF EXISTS idp_users;
DROP TABLE IF EXISTS idp_settings;
