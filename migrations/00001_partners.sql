-- +goose Up
CREATE TABLE partners (
    id          TEXT PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    country     CHAR(2) NOT NULL,
    contact_email TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE api_keys (
    id           BIGSERIAL PRIMARY KEY,
    partner_id   TEXT NOT NULL REFERENCES partners(id) ON DELETE CASCADE,
    prefix       TEXT NOT NULL,
    key_hash     TEXT NOT NULL UNIQUE,
    label        TEXT NOT NULL DEFAULT '',
    environment  TEXT NOT NULL DEFAULT 'sandbox',
    last_used_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX api_keys_partner_idx ON api_keys (partner_id);

-- +goose Down
DROP TABLE api_keys;
DROP TABLE partners;
