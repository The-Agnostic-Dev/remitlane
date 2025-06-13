// Package seed resets the database to the demo data set. It runs on every
// start when SEED_ON_START=true (the default outside production), so the
// sandbox always opens with the same partners, balances and a month of
// payout history relative to today.
package seed

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/remitlane/remitlane/internal/clock"
	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/ledger"
	"github.com/remitlane/remitlane/internal/money"
	"github.com/remitlane/remitlane/internal/partners"
	"github.com/remitlane/remitlane/internal/rates"
	"github.com/remitlane/remitlane/internal/store"
)

// Password is the demo password for every console user.
const Password = "remitlane2026"

// Partner is a seeded partner and its sandbox API key.
type Partner struct {
	ID, Slug, Name, Country, Email, Key string
	LimitUSD                            int64
	Funding                             []money.Amount
	Receiver                            int32
	ReceiverNote                        string
}

// Partners are the demo partners. Keys are sandbox keys and are documented
// in the README.
var Partners = []Partner{
	{
		ID: "prt_kobopay", Slug: "kobopay", Name: "Kobopay", Country: "NG", Email: "payments@kobopay.ng",
		Key: "rl_sandbox_kobopay_4f9a2c71d8e3b605", LimitUSD: 250000,
		Funding:  []money.Amount{money.New(50000000, money.USD), money.New(5000000000, money.NGN)},
		Receiver: 200, ReceiverNote: "Kobopay staging receiver",
	},
	{
		ID: "prt_dukamarket", Slug: "duka-market", Name: "Duka Market", Country: "KE", Email: "finance@dukamarket.co.ke",
		Key: "rl_sandbox_duka_9b31e6a0c4d27f18", LimitUSD: 30000,
		Funding:  []money.Amount{money.New(15000000, money.USD)},
		Receiver: 200, ReceiverNote: "Duka Market staging receiver",
	},
	{
		ID: "prt_accrafreight", Slug: "accra-freight", Name: "Accra Freight", Country: "GH", Email: "it@accrafreight.com.gh",
		Key: "rl_sandbox_accra_27c8d5f1e09ab346", LimitUSD: 20000,
		Funding:  []money.Amount{money.New(9000000, money.USD)},
		Receiver: 200, ReceiverNote: "Accra Freight staging receiver",
	},
}

// Users are the console logins.
var Users = []struct{ Email, Name, Role string }{
	{"ops@remitlane.com", "Wanjiru Kamau", "ops"},
	{"finance@remitlane.com", "Kofi Mensah", "finance"},
}

// Options controls a seed run.
type Options struct {
	BaseURL string
	Clock   *clock.Clock
	Quoter  *rates.Quoter
	Feed    rates.Feed
	Hash    func(string) (string, error)
	Log     *slog.Logger
}

const truncate = `TRUNCATE alert_log, receiver_hits, receiver_config, recon_runs, outbox_messages,
	provider_events, webhook_attempts, webhook_deliveries, webhook_events, webhook_endpoints,
	idempotency_keys, payout_events, ledger_entries, ledger_txns, ledger_accounts, payouts,
	payout_batches, quotes, console_sessions, ops_users, api_keys, partners RESTART IDENTITY CASCADE`

// Run wipes and reseeds the database.
func Run(ctx context.Context, s *store.Store, o Options) error {
	start := time.Now()
	hash, err := o.Hash(Password)
	if err != nil {
		return err
	}
	g := newGenerator(o)
	err = s.InTx(ctx, func(q *db.Queries, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, truncate); err != nil {
			return fmt.Errorf("truncate: %w", err)
		}
		for _, u := range Users {
			if err := q.CreateOpsUser(ctx, db.CreateOpsUserParams{Lower: u.Email, Name: u.Name, Role: u.Role, PasswordHash: hash}); err != nil {
				return err
			}
		}
		opened := o.Clock.Now().AddDate(0, 0, -45)
		for _, p := range Partners {
			if err := seedPartner(ctx, q, tx, p, o.BaseURL, opened); err != nil {
				return fmt.Errorf("partner %s: %w", p.Slug, err)
			}
		}
		if err := g.history(ctx, q, tx); err != nil {
			return fmt.Errorf("history: %w", err)
		}
		return g.webhooks(ctx, q, tx)
	})
	if err != nil {
		return err
	}
	o.Log.Info("seeded demo data", "payouts", g.count, "took", time.Since(start).Round(time.Millisecond))
	return nil
}

func seedPartner(ctx context.Context, q *db.Queries, tx pgx.Tx, p Partner, baseURL string, opened time.Time) error {
	if _, err := q.CreatePartner(ctx, db.CreatePartnerParams{
		ID: p.ID, Slug: p.Slug, Name: p.Name, Country: p.Country, ContactEmail: p.Email, DailyLimitUsdMinor: p.LimitUSD * 100,
	}); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE partners SET created_at = $2 WHERE id = $1`, p.ID, opened); err != nil {
		return err
	}
	if err := q.CreateAPIKey(ctx, db.CreateAPIKeyParams{
		PartnerID: p.ID, Prefix: partners.Prefix(p.Key), KeyHash: partners.HashKey(p.Key), Label: "Sandbox", Environment: "sandbox",
	}); err != nil {
		return err
	}
	if _, err := q.CreateEndpoint(ctx, db.CreateEndpointParams{
		PartnerID: p.ID, Url: baseURL + "/_dev/receiver/" + p.Slug, Secret: "whsec_sandbox_" + p.Slug,
	}); err != nil {
		return err
	}
	if err := q.UpsertReceiverConfig(ctx, db.UpsertReceiverConfigParams{Slug: p.Slug, RespondWith: p.Receiver, Note: p.ReceiverNote}); err != nil {
		return err
	}
	for _, f := range p.Funding {
		id, err := ledger.Post(ctx, q, "funding", "", "Opening balance", ledger.Fund(p.ID, f)...)
		if err != nil {
			return err
		}
		if err := backdateTxn(ctx, tx, id, opened); err != nil {
			return err
		}
	}
	return nil
}

func backdateTxn(ctx context.Context, tx pgx.Tx, id int64, at time.Time) error {
	if _, err := tx.Exec(ctx, `UPDATE ledger_txns SET created_at = $2 WHERE id = $1`, id, at); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE ledger_entries SET created_at = $2 WHERE txn_id = $1`, id, at)
	return err
}
