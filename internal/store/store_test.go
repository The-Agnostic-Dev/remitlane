package store_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/ledger"
	"github.com/remitlane/remitlane/internal/money"
	"github.com/remitlane/remitlane/internal/store"
	"github.com/remitlane/remitlane/internal/testutil"
)

func partner(t *testing.T, st *store.Store, id string) {
	t.Helper()
	if _, err := st.Q.CreatePartner(context.Background(), db.CreatePartnerParams{
		ID: id, Slug: id, Name: id, Country: "NG", ContactEmail: id + "@example.com", DailyLimitUsdMinor: 1000000,
	}); err != nil {
		t.Fatal(err)
	}
}

func addPayout(t *testing.T, st *store.Store, partnerID, ref, status string, at time.Time, usd int64) db.Payout {
	t.Helper()
	ctx := context.Background()
	p, err := st.Q.CreatePayout(ctx, db.CreatePayoutParams{
		ID: "po_" + ref, PartnerID: partnerID, Reference: ref, Status: status, Channel: "bank", Provider: "swiftpay",
		SourceCurrency: "USD", SourceAmountMinor: usd, FeeMinor: 100, DestCurrency: "NGN", DestAmountMinor: usd * 1500,
		UsdAmountMinor: usd, Rate: "1500.00000000", RecipientName: "Ada Obi", RecipientCountry: "NG",
		BankCode: "058", AccountNumber: "0123456789",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Pool.Exec(ctx, `UPDATE payouts SET created_at = $2 WHERE id = $1`, p.ID, at); err != nil {
		t.Fatal(err)
	}
	p.CreatedAt = at
	return p
}

func TestBalancesFollowLedger(t *testing.T) {
	st := testutil.Store(t)
	ctx := context.Background()
	partner(t, st, "prt_a")
	err := st.InTx(ctx, func(q *db.Queries, tx pgx.Tx) error {
		if _, err := ledger.Post(ctx, q, "funding", "", "", ledger.Fund("prt_a", money.New(100000, money.USD))...); err != nil {
			return err
		}
		lp := ledger.Payout{ID: "po_x", PartnerID: "prt_a", Provider: "swiftpay", Currency: money.USD, Amount: 2500, Fee: 150}
		_, err := ledger.Post(ctx, q, "payout_hold", "", "", ledger.Hold(lp)...)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	bs, err := st.PartnerBalances(ctx, "prt_a")
	if err != nil {
		t.Fatal(err)
	}
	if len(bs) != 1 || bs[0].Available != 97350 || bs[0].Held != 2650 {
		t.Fatalf("balances %+v", bs)
	}
}

func TestLedgerRefusesUnbalanced(t *testing.T) {
	st := testutil.Store(t)
	ctx := context.Background()
	partner(t, st, "prt_a")
	err := st.InTx(ctx, func(q *db.Queries, _ pgx.Tx) error {
		_, err := ledger.Post(ctx, q, "oops", "", "", ledger.Entry{Account: ledger.Available("prt_a", money.USD), Minor: 10})
		return err
	})
	if err == nil {
		t.Fatal("unbalanced transaction was accepted")
	}
}

func TestListPartnerPayoutsPages(t *testing.T) {
	st := testutil.Store(t)
	ctx := context.Background()
	partner(t, st, "prt_a")
	base := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	for i := range 5 {
		addPayout(t, st, "prt_a", fmt.Sprintf("r%d", i), "completed", base.Add(time.Duration(i)*time.Minute), 1000)
	}
	page, more, err := st.ListPartnerPayouts(ctx, store.PartnerPayoutFilter{PartnerID: "prt_a", Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 3 || !more || page[0].Reference != "r4" {
		t.Fatalf("first page %d more=%v first=%s", len(page), more, page[0].Reference)
	}
	page, more, err = st.ListPartnerPayouts(ctx, store.PartnerPayoutFilter{PartnerID: "prt_a", Limit: 3, Before: page[2].CreatedAt})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || more || page[1].Reference != "r0" {
		t.Fatalf("second page %d more=%v", len(page), more)
	}
}

func TestConsoleFilters(t *testing.T) {
	st := testutil.Store(t)
	ctx := context.Background()
	partner(t, st, "prt_a")
	partner(t, st, "prt_b")
	now := time.Now()
	addPayout(t, st, "prt_a", "a1", "completed", now, 1000)
	addPayout(t, st, "prt_a", "a2", "failed", now, 1000)
	addPayout(t, st, "prt_b", "b1", "failed", now, 1000)
	rows, total, err := st.ListConsolePayouts(ctx, store.ConsoleFilter{Status: "failed", Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(rows) != 2 {
		t.Fatalf("failed filter: total %d rows %d", total, len(rows))
	}
	_, total, err = st.ListConsolePayouts(ctx, store.ConsoleFilter{PartnerID: "prt_a", Search: "A1", Page: 1})
	if err != nil || total != 1 {
		t.Fatalf("search: total %d err %v", total, err)
	}
}

func TestUsedUSDSkipsFailed(t *testing.T) {
	st := testutil.Store(t)
	ctx := context.Background()
	partner(t, st, "prt_a")
	at := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	addPayout(t, st, "prt_a", "u1", "completed", at, 1500)
	addPayout(t, st, "prt_a", "u2", "processing", at, 500)
	addPayout(t, st, "prt_a", "u3", "failed", at, 9000)
	used, err := st.UsedUSD(ctx, "prt_a", at.Add(-time.Hour), at.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if used != 2000 {
		t.Errorf("used %d, want 2000", used)
	}
}

func TestRequeueFailedForEndpoint(t *testing.T) {
	st := testutil.Store(t)
	ctx := context.Background()
	partner(t, st, "prt_a")
	ep, err := st.Q.CreateEndpoint(ctx, db.CreateEndpointParams{PartnerID: "prt_a", Url: "http://example.invalid/hook", Secret: "s"})
	if err != nil {
		t.Fatal(err)
	}
	for i, status := range []string{"failed", "failed", "delivered"} {
		ev := fmt.Sprintf("evt_%d", i)
		if err := st.Q.CreateEvent(ctx, db.CreateEventParams{ID: ev, PartnerID: "prt_a", Type: "payout.completed", Payload: []byte(`{}`)}); err != nil {
			t.Fatal(err)
		}
		id, err := st.Q.CreateDelivery(ctx, db.CreateDeliveryParams{EventID: ev, EndpointID: ep.ID})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.Pool.Exec(ctx, `UPDATE webhook_deliveries SET status = $2 WHERE id = $1`, id, status); err != nil {
			t.Fatal(err)
		}
	}
	failed, err := st.ListFailedDeliveries(ctx, "prt_a", 10)
	if err != nil || len(failed) != 2 {
		t.Fatalf("failed list %d %v", len(failed), err)
	}
	ids, err := st.RequeueFailed(ctx, ep.ID)
	if err != nil || len(ids) != 2 {
		t.Fatalf("requeued %v %v", ids, err)
	}
	if failed, _ = st.ListFailedDeliveries(ctx, "", 10); len(failed) != 0 {
		t.Errorf("still failed: %d", len(failed))
	}
}
