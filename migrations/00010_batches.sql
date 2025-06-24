-- +goose Up
CREATE TABLE payout_batches (
    id          TEXT PRIMARY KEY,
    partner_id  TEXT NOT NULL REFERENCES partners(id),
    reference   TEXT NOT NULL,
    item_count  INTEGER NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (partner_id, reference)
);

ALTER TABLE payouts ADD COLUMN batch_id TEXT REFERENCES payout_batches(id);
CREATE INDEX payouts_batch_idx ON payouts (batch_id) WHERE batch_id IS NOT NULL;

-- +goose Down
ALTER TABLE payouts DROP COLUMN batch_id;
DROP TABLE payout_batches;
