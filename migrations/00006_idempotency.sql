-- +goose Up
CREATE TABLE idempotency_keys (
    partner_id      TEXT NOT NULL REFERENCES partners(id),
    key             TEXT NOT NULL,
    request_hash    TEXT NOT NULL,
    response_status INTEGER NOT NULL,
    response_body   BYTEA NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (partner_id, key)
);

-- +goose Down
DROP TABLE idempotency_keys;
