// Package storetest holds helpers for tests that need Postgres. They run when
// DATABASE_URL points at a disposable database and are skipped otherwise.
package storetest

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/remitlane/remitlane/internal/migrate"
	"github.com/remitlane/remitlane/internal/store"
)

// Logger discards output.
func Logger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// Store connects to DATABASE_URL, migrates and empties every table.
func Store(t *testing.T) *store.Store {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping database test")
	}
	ctx := context.Background()
	st, err := store.Open(ctx, url, 4)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(st.Close)
	if err := migrate.Up(ctx, st.Pool, Logger()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := st.Pool.Exec(ctx, `TRUNCATE alert_log, receiver_hits, receiver_config, recon_runs, outbox_messages,
		provider_events, webhook_attempts, webhook_deliveries, webhook_events, webhook_endpoints,
		idempotency_keys, payout_events, ledger_entries, ledger_txns, ledger_accounts, payouts,
		payout_batches, quotes, console_sessions, ops_users, api_keys, partners RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return st
}
