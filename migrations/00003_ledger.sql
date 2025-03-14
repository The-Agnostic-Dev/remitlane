-- +goose Up
-- Double-entry ledger. Every transaction's entries sum to zero per currency.
-- Amounts are major units with four decimal places (see docs/adr/0002-ledger.md).
CREATE TABLE ledger_accounts (
    id          BIGSERIAL PRIMARY KEY,
    code        TEXT NOT NULL UNIQUE,
    partner_id  TEXT REFERENCES partners(id),
    kind        TEXT NOT NULL,
    currency    CHAR(3) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ledger_txns (
    id          BIGSERIAL PRIMARY KEY,
    kind        TEXT NOT NULL,
    payout_id   TEXT,
    memo        TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ledger_entries (
    id          BIGSERIAL PRIMARY KEY,
    txn_id      BIGINT NOT NULL REFERENCES ledger_txns(id),
    account_id  BIGINT NOT NULL REFERENCES ledger_accounts(id),
    amount      NUMERIC(20,4) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ledger_entries_account_idx ON ledger_entries (account_id);
CREATE INDEX ledger_entries_txn_idx ON ledger_entries (txn_id);
CREATE INDEX ledger_txns_payout_idx ON ledger_txns (payout_id);

-- +goose Down
DROP TABLE ledger_entries;
DROP TABLE ledger_txns;
DROP TABLE ledger_accounts;
