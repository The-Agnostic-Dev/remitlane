-- +goose Up
-- The built-in fake partner receiver (/_dev/receiver). Not used in production.
CREATE TABLE receiver_config (
    slug          TEXT PRIMARY KEY,
    respond_with  INTEGER NOT NULL DEFAULT 200,
    note          TEXT NOT NULL DEFAULT ''
);

CREATE TABLE receiver_hits (
    id            BIGSERIAL PRIMARY KEY,
    slug          TEXT NOT NULL,
    event_id      TEXT NOT NULL DEFAULT '',
    event_type    TEXT NOT NULL DEFAULT '',
    payout_id     TEXT NOT NULL DEFAULT '',
    signature     TEXT NOT NULL DEFAULT '',
    responded     INTEGER NOT NULL,
    body          TEXT NOT NULL,
    received_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX receiver_hits_slug_idx ON receiver_hits (slug, id DESC);

-- +goose Down
DROP TABLE receiver_hits;
DROP TABLE receiver_config;
