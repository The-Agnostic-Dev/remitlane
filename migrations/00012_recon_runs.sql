-- +goose Up
CREATE TABLE recon_runs (
    id            BIGSERIAL PRIMARY KEY,
    business_date DATE NOT NULL,
    payout_count  INTEGER NOT NULL,
    usd_total_minor BIGINT NOT NULL,
    triggered_by  TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX recon_runs_date_idx ON recon_runs (business_date DESC);

-- +goose Down
DROP TABLE recon_runs;
