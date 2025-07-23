-- +goose Up
-- Messages sent through MAIL_PROVIDER=outbox land here instead of a real
-- mail service. Shown at /_dev/outbox.
CREATE TABLE outbox_messages (
    id          BIGSERIAL PRIMARY KEY,
    channel     TEXT NOT NULL DEFAULT 'email',
    recipient   TEXT NOT NULL,
    subject     TEXT NOT NULL,
    body        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE outbox_messages;
