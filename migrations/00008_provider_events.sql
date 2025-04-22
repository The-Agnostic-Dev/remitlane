-- +goose Up
-- Every inbound provider webhook, kept for support and replay.
CREATE TABLE provider_events (
    id            BIGSERIAL PRIMARY KEY,
    provider      TEXT NOT NULL,
    reference     TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT '',
    signature_ok  BOOLEAN NOT NULL,
    outcome       TEXT NOT NULL DEFAULT '',
    body          TEXT NOT NULL,
    received_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX provider_events_received_idx ON provider_events (provider, received_at DESC);

-- +goose Down
DROP TABLE provider_events;
