-- +goose Up
CREATE TABLE alert_log (
    id          BIGSERIAL PRIMARY KEY,
    partner_id  TEXT NOT NULL REFERENCES partners(id),
    kind        TEXT NOT NULL,
    day         DATE NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (partner_id, kind, day)
);

-- +goose Down
DROP TABLE alert_log;
