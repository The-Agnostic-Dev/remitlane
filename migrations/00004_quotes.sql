-- +goose Up
CREATE TABLE quotes (
    id                  TEXT PRIMARY KEY,
    partner_id          TEXT NOT NULL REFERENCES partners(id),
    source_currency     CHAR(3) NOT NULL,
    dest_currency       CHAR(3) NOT NULL,
    channel             TEXT NOT NULL,
    source_amount_minor BIGINT NOT NULL,
    dest_amount_minor   BIGINT NOT NULL,
    fee_minor           BIGINT NOT NULL,
    rate                NUMERIC(20,8) NOT NULL,
    mid_rate            NUMERIC(20,8) NOT NULL,
    spread_bps          INTEGER NOT NULL,
    expires_at          TIMESTAMPTZ NOT NULL,
    used_at             TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX quotes_partner_idx ON quotes (partner_id, created_at DESC);

-- +goose Down
DROP TABLE quotes;
