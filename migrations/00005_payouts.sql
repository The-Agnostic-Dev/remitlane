-- +goose Up
CREATE TABLE payouts (
    id                  TEXT PRIMARY KEY,
    partner_id          TEXT NOT NULL REFERENCES partners(id),
    quote_id            TEXT REFERENCES quotes(id),
    reference           TEXT NOT NULL,
    status              TEXT NOT NULL,
    channel             TEXT NOT NULL,
    provider            TEXT NOT NULL,
    provider_ref        TEXT NOT NULL DEFAULT '',
    source_currency     CHAR(3) NOT NULL,
    source_amount_minor BIGINT NOT NULL,
    fee_minor           BIGINT NOT NULL,
    dest_currency       CHAR(3) NOT NULL,
    dest_amount_minor   BIGINT NOT NULL,
    usd_amount_minor    BIGINT NOT NULL,
    rate                NUMERIC(20,8) NOT NULL,
    recipient_name      TEXT NOT NULL,
    recipient_country   CHAR(2) NOT NULL,
    bank_code           TEXT NOT NULL DEFAULT '',
    account_number      TEXT NOT NULL DEFAULT '',
    msisdn              TEXT NOT NULL DEFAULT '',
    narration           TEXT NOT NULL DEFAULT '',
    failure_code        TEXT NOT NULL DEFAULT '',
    failure_message     TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    submitted_at        TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    UNIQUE (partner_id, reference)
);

CREATE INDEX payouts_partner_created_idx ON payouts (partner_id, created_at DESC);
CREATE INDEX payouts_status_idx ON payouts (status);

CREATE TABLE payout_events (
    id          BIGSERIAL PRIMARY KEY,
    payout_id   TEXT NOT NULL REFERENCES payouts(id),
    status      TEXT NOT NULL,
    source      TEXT NOT NULL,
    detail      TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX payout_events_payout_idx ON payout_events (payout_id, id);

-- +goose Down
DROP TABLE payout_events;
DROP TABLE payouts;
