package payouts_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/remitlane/remitlane/internal/clock"
	"github.com/remitlane/remitlane/internal/config"
	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/domain"
	"github.com/remitlane/remitlane/internal/ledger"
	"github.com/remitlane/remitlane/internal/limits"
	"github.com/remitlane/remitlane/internal/money"
	"github.com/remitlane/remitlane/internal/outbound"
	"github.com/remitlane/remitlane/internal/payouts"
	"github.com/remitlane/remitlane/internal/providers"
	"github.com/remitlane/remitlane/internal/quotes"
	"github.com/remitlane/remitlane/internal/rates"
	"github.com/remitlane/remitlane/internal/recipients"
	"github.com/remitlane/remitlane/internal/store"
	"github.com/remitlane/remitlane/internal/testutil"
)

type fakeQueue struct{ dispatched []string }

func (f *fakeQueue) Dispatch(_ context.Context, id string) error {
	f.dispatched = append(f.dispatched, id)
	return nil
}
func (f *fakeQueue) Deliver(context.Context, int64, bool) error       { return nil }
func (f *fakeQueue) Recon(context.Context, string, string) error      { return nil }
func (f *fakeQueue) LowBalance(context.Context, string, string) error { return nil }

type fakeProvider struct {
	result providers.Result
	err    error
}

func (f *fakeProvider) Name() string { return providers.Swiftpay }
func (f *fakeProvider) Send(context.Context, providers.TransferRequest) (providers.Result, error) {
	return f.result, f.err
}
func (f *fakeProvider) Status(context.Context, string) (providers.Result, error) {
	return f.result, f.err
}

type fixture struct {
	st      *store.Store
	svc     *payouts.Service
	quotes  *quotes.Service
	queue   *fakeQueue
	prov    *fakeProvider
	partner db.Partner
}

func setup(t *testing.T) fixture {
	t.Helper()
	st := testutil.Store(t)
	ctx := context.Background()
	loc, _ := time.LoadLocation("Africa/Lagos")
	clk := clock.Fixed(time.Now(), loc)
	feed, err := rates.NewFixedFeed(config.RatesConfig{USDNGN: "1540.00", USDKES: "129.37", USDGHS: "15.82", USDUGX: "3698.40"})
	if err != nil {
		t.Fatal(err)
	}
	quoter := rates.NewQuoter(feed, clk, 15*time.Minute)
	q := &fakeQueue{}
	prov := &fakeProvider{result: providers.Result{ProviderRef: "SWP2_T1", RawStatus: "PROCESSING", Status: domain.Processing}}
	p, err := st.Q.CreatePartner(ctx, db.CreatePartnerParams{
		ID: "prt_t", Slug: "t", Name: "Test Partner", Country: "NG", ContactEmail: "t@example.com", DailyLimitUsdMinor: 10000000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.InTx(ctx, func(q *db.Queries, _ pgx.Tx) error {
		_, err := ledger.Post(ctx, q, "funding", "", "", ledger.Fund(p.ID, money.New(100000, money.USD))...)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	svc := payouts.New(payouts.Deps{
		Store: st, Quoter: quoter, Feed: feed, Limits: limits.New(st, clk),
		Events: outbound.New(st, q, nil, testutil.Logger(), 5), Queue: q,
		Providers: providers.Registry{providers.Swiftpay: prov}, Clock: clk, Log: testutil.Logger(), MaxBulk: 10,
	})
	return fixture{st: st, svc: svc, quotes: quotes.New(st, quoter, clk), queue: q, prov: prov, partner: p}
}

func (f fixture) create(t *testing.T, ref string, usd int64) db.Payout {
	t.Helper()
	ctx := context.Background()
	qt, err := f.quotes.Create(ctx, f.partner.ID, rates.Request{From: money.USD, To: money.NGN, Channel: rates.Bank, AmountMinor: usd, AmountType: rates.BySource})
	if err != nil {
		t.Fatal(err)
	}
	p, err := f.svc.Create(ctx, f.partner, payouts.CreateRequest{
		QuoteID: qt.ID, Reference: ref,
		Recipient: recipients.Recipient{Name: "Ada Obi", Country: "NG", BankCode: "058", AccountNumber: "0123456789"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func available(t *testing.T, f fixture) int64 {
	t.Helper()
	bs, err := f.st.PartnerBalances(context.Background(), f.partner.ID)
	if err != nil || len(bs) == 0 {
		t.Fatalf("balances %v %v", bs, err)
	}
	return bs[0].Available
}

func TestCreateHoldsFundsAndQueuesDispatch(t *testing.T) {
	f := setup(t)
	p := f.create(t, "INV-1", 5000)
	if p.Status != string(domain.Pending) || p.DestAmountMinor != 7584500 {
		t.Fatalf("created %+v", p)
	}
	if got := available(t, f); got != 100000-5000-100 {
		t.Errorf("available %d", got)
	}
	if len(f.queue.dispatched) != 1 || f.queue.dispatched[0] != p.ID {
		t.Errorf("dispatched %v", f.queue.dispatched)
	}
}

func TestQuoteCanOnlyBeUsedOnce(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	qt, err := f.quotes.Create(ctx, f.partner.ID, rates.Request{From: money.USD, To: money.NGN, Channel: rates.Bank, AmountMinor: 5000})
	if err != nil {
		t.Fatal(err)
	}
	req := payouts.CreateRequest{QuoteID: qt.ID, Reference: "INV-2",
		Recipient: recipients.Recipient{Name: "Ada Obi", Country: "NG", BankCode: "058", AccountNumber: "0123456789"}}
	if _, err := f.svc.Create(ctx, f.partner, req); err != nil {
		t.Fatal(err)
	}
	req.Reference = "INV-3"
	_, err = f.svc.Create(ctx, f.partner, req)
	var pe *payouts.Error
	if !errors.As(err, &pe) || pe.Code != payouts.CodeQuoteUsed {
		t.Fatalf("want quote_already_used, got %v", err)
	}
}

func TestDispatchThenComplete(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	p := f.create(t, "INV-4", 5000)
	if err := f.svc.Dispatch(ctx, p.ID, false); err != nil {
		t.Fatal(err)
	}
	got, _ := f.st.Q.GetPayout(ctx, p.ID)
	if got.Status != string(domain.Processing) || got.ProviderRef != "SWP2_T1" {
		t.Fatalf("after dispatch %s %q", got.Status, got.ProviderRef)
	}
	if _, err := f.svc.Apply(ctx, p.ID, payouts.Update{Status: domain.Completed, Source: "test"}); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.Apply(ctx, p.ID, payouts.Update{Status: domain.Failed, Source: "test"})
	if !errors.Is(err, payouts.ErrIgnored) {
		t.Fatalf("failed after completed: %v", err)
	}
	if got := available(t, f); got != 100000-5100 {
		t.Errorf("available %d", got)
	}
}

func TestRejectedTransferReleasesFunds(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	f.prov.err = &providers.RejectedError{Provider: "swiftpay", Code: "INVALID_ACCOUNT", Message: "no such account"}
	p := f.create(t, "INV-5", 5000)
	if err := f.svc.Dispatch(ctx, p.ID, false); err != nil {
		t.Fatal(err)
	}
	got, _ := f.st.Q.GetPayout(ctx, p.ID)
	if got.Status != string(domain.Failed) || got.FailureCode != "INVALID_ACCOUNT" {
		t.Fatalf("status %s code %s", got.Status, got.FailureCode)
	}
	if got := available(t, f); got != 100000 {
		t.Errorf("available %d, want funds back", got)
	}
}
