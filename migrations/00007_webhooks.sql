-- +goose Up
CREATE TABLE webhook_endpoints (
    id          BIGSERIAL PRIMARY KEY,
    partner_id  TEXT NOT NULL REFERENCES partners(id),
    url         TEXT NOT NULL,
    secret      TEXT NOT NULL,
    active      BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE webhook_events (
    id          TEXT PRIMARY KEY,
    partner_id  TEXT NOT NULL REFERENCES partners(id),
    type        TEXT NOT NULL,
    payout_id   TEXT,
    payload     JSONB NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE webhook_deliveries (
    id               BIGSERIAL PRIMARY KEY,
    event_id         TEXT NOT NULL REFERENCES webhook_events(id),
    endpoint_id      BIGINT NOT NULL REFERENCES webhook_endpoints(id),
    status           TEXT NOT NULL DEFAULT 'queued',
    attempts         INTEGER NOT NULL DEFAULT 0,
    last_status_code INTEGER NOT NULL DEFAULT 0,
    last_error       TEXT NOT NULL DEFAULT '',
    last_attempt_at  TIMESTAMPTZ,
    delivered_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX webhook_deliveries_status_idx ON webhook_deliveries (status, endpoint_id);

CREATE TABLE webhook_attempts (
    id          BIGSERIAL PRIMARY KEY,
    delivery_id BIGINT NOT NULL REFERENCES webhook_deliveries(id),
    status_code INTEGER NOT NULL DEFAULT 0,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    error       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX webhook_attempts_delivery_idx ON webhook_attempts (delivery_id);

-- +goose Down
DROP TABLE webhook_attempts;
DROP TABLE webhook_deliveries;
DROP TABLE webhook_events;
DROP TABLE webhook_endpoints;
