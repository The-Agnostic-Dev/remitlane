-- +goose Up
ALTER TABLE partners ADD COLUMN daily_limit_usd_minor BIGINT NOT NULL DEFAULT 2500000;

-- +goose Down
ALTER TABLE partners DROP COLUMN daily_limit_usd_minor;
