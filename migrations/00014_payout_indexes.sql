-- +goose Up
CREATE INDEX payouts_created_idx ON payouts (created_at DESC);
CREATE INDEX payouts_provider_ref_idx ON payouts (provider, provider_ref) WHERE provider_ref <> '';

-- +goose Down
DROP INDEX payouts_provider_ref_idx;
DROP INDEX payouts_created_idx;
