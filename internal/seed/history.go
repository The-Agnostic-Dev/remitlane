package seed

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mrand "math/rand/v2"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/domain"
	"github.com/remitlane/remitlane/internal/ledger"
	"github.com/remitlane/remitlane/internal/money"
	"github.com/remitlane/remitlane/internal/providers"
	"github.com/remitlane/remitlane/internal/rates"
)

type generator struct {
	o      Options
	rnd    *mrand.Rand
	count  int
	seq    int
	recent map[string][]db.Payout
}

func newGenerator(o Options) *generator {
	return &generator{o: o, rnd: mrand.New(mrand.NewPCG(2026, 11)), seq: 40211, recent: map[string][]db.Payout{}}
}

type corridorMix struct {
	src, dst money.Currency
	ch       rates.Channel
	min, max int64 // destination major units
	weight   int
}

var mixes = map[string][]corridorMix{
	"prt_kobopay": {
		{money.USD, money.NGN, rates.Bank, 50000, 2500000, 7},
		{money.NGN, money.NGN, rates.Bank, 20000, 800000, 3},
	},
	"prt_dukamarket": {
		{money.USD, money.KES, rates.Bank, 2000, 200000, 4},
		{money.USD, money.KES, rates.MobileMoney, 500, 70000, 4},
		{money.USD, money.UGX, rates.MobileMoney, 20000, 2000000, 1},
		{money.USD, money.UGX, rates.Bank, 200000, 8000000, 1},
	},
	"prt_accrafreight": {
		{money.USD, money.GHS, rates.Bank, 300, 30000, 1},
		{money.USD, money.GHS, rates.MobileMoney, 50, 5000, 1},
	},
}

var refPrefix = map[string]string{"prt_kobopay": "KBP-TRF-", "prt_dukamarket": "DM-ORD-", "prt_accrafreight": "AF-INV-"}

type spec struct {
	partner   Partner
	mix       corridorMix
	destMajor int64
	at        time.Time
	status    domain.Status
	ref       string
	batchID   string
}

// history writes about a month of payouts plus the fixtures support relies
// on (a settlement batch, late-night payouts either side of midnight, UGX).
func (g *generator) history(ctx context.Context, q *db.Queries, tx pgx.Tx) error {
	now := g.o.Clock.Now()
	loc := g.o.Clock.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	for d := 30; d >= 1; d-- {
		day := today.AddDate(0, 0, -d)
		n := 4 + g.rnd.IntN(4)
		if wd := day.Weekday(); wd == time.Saturday || wd == time.Sunday {
			n = 1 + g.rnd.IntN(2)
		}
		for range n {
			at := day.Add(7*time.Hour + time.Duration(g.rnd.IntN(13*3600))*time.Second)
			if _, err := g.insert(ctx, q, tx, g.random(at, g.randomStatus())); err != nil {
				return err
			}
		}
	}
	// This morning's traffic, up to a few minutes ago.
	for at := today.Add(7*time.Hour + 20*time.Minute); at.Before(now.Add(-10 * time.Minute)); at = at.Add(time.Duration(50+g.rnd.IntN(70)) * time.Minute) {
		if _, err := g.insert(ctx, q, tx, g.random(at, domain.Completed)); err != nil {
			return err
		}
	}

	if err := g.batch(ctx, q, tx, today.AddDate(0, 0, -1).Add(10*time.Hour)); err != nil {
		return err
	}

	return nil
}
	return nil
}

func (g *generator) random(at time.Time, st domain.Status) spec {
	p := Partners[g.pickPartner()]
	ms := mixes[p.ID]
	total := 0
	for _, m := range ms {
		total += m.weight
	}
	r := g.rnd.IntN(total)
	var mix corridorMix
	for _, m := range ms {
		if r < m.weight {
			mix = m
			break
		}
		r -= m.weight
	}
	return spec{partner: p, mix: mix, destMajor: g.amount(mix), at: at, status: st}
}

func (g *generator) pickPartner() int {
	switch r := g.rnd.IntN(10); {
	case r < 4:
		return 0
	case r < 7:
		return 1
	}
	return 2
}

// amount picks a round destination amount in the corridor's range.
func (g *generator) amount(m corridorMix) int64 {
	step := int64(50)
	if m.max > 1000000 {
		step = 5000
	} else if m.max > 100000 {
		step = 500
	}
	v := m.min + g.rnd.Int64N(m.max-m.min)
	return max(m.min, v/step*step)
}

func (g *generator) randomStatus() domain.Status {
	switch r := g.rnd.IntN(100); {
	case r < 8:
		return domain.Failed
	case r < 10:
		return domain.Reversed
	}
	return domain.Completed
}

// batch writes yesterday's settlement batch for Accra Freight: 25 payouts
// booked in one request, so they share a created_at.
func (g *generator) batch(ctx context.Context, q *db.Queries, tx pgx.Tx, at time.Time) error {
	p := Partners[2]
	id := "bat_" + randHex(10)
	ref := "AF-SETTLE-" + at.Format("20060102")
	if _, err := q.CreateBatch(ctx, db.CreateBatchParams{ID: id, PartnerID: p.ID, Reference: ref, ItemCount: 25}); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE payout_batches SET created_at = $2 WHERE id = $1`, id, at); err != nil {
		return err
	}
	for i := range 25 {
		mix := mixes[p.ID][i%2]
		s := spec{partner: p, mix: mix, destMajor: g.amount(mix), at: at, status: domain.Completed,
			ref: fmt.Sprintf("%s-%02d", ref, i+1), batchID: id}
		if _, err := g.insert(ctx, q, tx, s); err != nil {
			return err
		}
	}
	return nil
}

func (g *generator) recipient(country string, ch rates.Channel) (name, bank, account, msisdn string) {
	ns := names[country]
	name = ns[g.rnd.IntN(len(ns))]
	if ch == rates.MobileMoney {
		ps := mobilePrefixes[country]
		msisdn = ps[g.rnd.IntN(len(ps))] + fmt.Sprintf("%07d", 1000000+g.rnd.IntN(8000000))
		return
	}
	bs := bankCodes[country]
	bank = bs[g.rnd.IntN(len(bs))]
	digits := map[string]int{"NG": 10, "KE": 12, "GH": 13, "UG": 11}[country]
	var b strings.Builder
	b.WriteByte(byte('1' + g.rnd.IntN(8)))
	for b.Len() < digits-2 {
		b.WriteByte(byte('0' + g.rnd.IntN(10)))
	}
	b.WriteString(fmt.Sprintf("%02d", 12+g.rnd.IntN(40)))
	account = b.String()
	return
}

func (g *generator) insert(ctx context.Context, q *db.Queries, tx pgx.Tx, s spec) (db.Payout, error) {
	m := s.mix
	price, err := g.o.Quoter.Calculate(ctx, rates.Request{
		From: m.src, To: m.dst, Channel: m.ch, AmountMinor: s.destMajor * pow10(m.dst.Exponent()), AmountType: rates.ByDestination,
	})
	if err != nil {
		return db.Payout{}, err
	}
	usd, err := rates.ToUSD(ctx, g.o.Feed, money.New(price.SourceMinor, m.src))
	if err != nil {
		return db.Payout{}, err
	}
	country := m.dst.Country()
	name, bank, account, msisdn := g.recipient(country, m.ch)
	if s.ref == "" {
		g.seq += 1 + g.rnd.IntN(7)
		s.ref = fmt.Sprintf("%s%d", refPrefix[s.partner.ID], g.seq)
	}
	provider, ref := providers.Swiftpay, "SWP2_"+strings.ToUpper(randHex(6))
	if m.ch == rates.MobileMoney {
		provider, ref = providers.MoMoBridge, "MMB-"+strings.ToUpper(randHex(5))
	}
	p, err := q.CreatePayout(ctx, db.CreatePayoutParams{
		ID: "po_" + randHex(10), PartnerID: s.partner.ID, BatchID: text(s.batchID), Reference: s.ref,
		Status: string(domain.Pending), Channel: string(m.ch), Provider: provider,
		SourceCurrency: string(m.src), SourceAmountMinor: price.SourceMinor, FeeMinor: price.FeeMinor,
		DestCurrency: string(m.dst), DestAmountMinor: price.DestMinor, UsdAmountMinor: usd,
		Rate: price.Rate.FloatString(8), RecipientName: name, RecipientCountry: country,
		BankCode: bank, AccountNumber: account, Msisdn: msisdn, Narration: pick(g.rnd, narrations[s.partner.ID]),
	})
	if err != nil {
		return db.Payout{}, err
	}
	done := s.at.Add(time.Duration(25+g.rnd.IntN(300)) * time.Second)
	code, msg := "", ""
	if s.status == domain.Failed {
		code, msg = failureFor(m.ch, g.rnd)
	}
	updated := done
	if s.status == domain.Reversed {
		updated = done.Add(2 * time.Hour)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE payouts SET status = $2, provider_ref = $3, failure_code = $4, failure_message = $5,
		       created_at = $6, submitted_at = $7, completed_at = $8, updated_at = $9
		WHERE id = $1`, p.ID, string(s.status), ref, code, msg, s.at, s.at.Add(2*time.Second), done, updated); err != nil {
		return db.Payout{}, err
	}
	if err := g.post(ctx, q, tx, p, s.status, s.at, done); err != nil {
		return db.Payout{}, err
	}
	if err := g.events(ctx, tx, p.ID, provider, s.status, s.at, done, updated); err != nil {
		return db.Payout{}, err
	}
	g.count++
	if s.status == domain.Completed && s.at.After(g.o.Clock.Now().AddDate(0, 0, -3)) {
		full, err := q.GetPayout(ctx, p.ID)
		if err != nil {
			return db.Payout{}, err
		}
		g.recent[s.partner.ID] = append(g.recent[s.partner.ID], full)
	}
	return p, nil
}

type posting struct {
	kind    string
	entries []ledger.Entry
	at      time.Time
}

func (g *generator) post(ctx context.Context, q *db.Queries, tx pgx.Tx, p db.Payout, st domain.Status, at, done time.Time) error {
	lp := ledger.Payout{ID: p.ID, PartnerID: p.PartnerID, Provider: p.Provider, Currency: money.Currency(p.SourceCurrency), Amount: p.SourceAmountMinor, Fee: p.FeeMinor}
	steps := []posting{{"payout_hold", ledger.Hold(lp), at}}
	switch st {
	case domain.Completed:
		steps = append(steps, posting{"payout_settle", ledger.Settle(lp), done})
	case domain.Failed:
		steps = append(steps, posting{"payout_release", ledger.Release(lp), done})
	case domain.Reversed:
		steps = append(steps, posting{"payout_settle", ledger.Settle(lp), done}, posting{"payout_reversal", ledger.Reverse(lp), done.Add(2 * time.Hour)})
	}
	for _, s := range steps {
		id, err := ledger.Post(ctx, q, s.kind, p.ID, "", s.entries...)
		if err != nil {
			return err
		}
		if err := backdateTxn(ctx, tx, id, s.at); err != nil {
			return err
		}
	}
	return nil
}

func (g *generator) events(ctx context.Context, tx pgx.Tx, id, provider string, st domain.Status, at, done, updated time.Time) error {
	rows := [][]any{
		{id, "pending", "api", "created", at},
		{id, "processing", provider, "accepted as PROCESSING", at.Add(2 * time.Second)},
	}
	switch st {
	case domain.Reversed:
		rows = append(rows, []any{id, "completed", provider, "webhook: COMPLETED", done}, []any{id, "reversed", provider, "webhook: REVERSED", updated})
	default:
		rows = append(rows, []any{id, string(st), provider, "webhook: " + strings.ToUpper(string(st)), done})
	}
	for _, r := range rows {
		if _, err := tx.Exec(ctx, `INSERT INTO payout_events (payout_id, status, source, detail, created_at) VALUES ($1, $2, $3, $4, $5)`, r...); err != nil {
			return err
		}
	}
	return nil
}

func failureFor(ch rates.Channel, r *mrand.Rand) (string, string) {
	if ch == rates.MobileMoney {
		return "SUBSCRIBER_NOT_FOUND", "subscriber not found"
	}
	if r.IntN(2) == 0 {
		return "INVALID_ACCOUNT", "The account number does not exist at this bank"
	}
	return "ACCOUNT_DORMANT", "The recipient's account is dormant"
}

func pick(r *mrand.Rand, s []string) string { return s[r.IntN(len(s))] }

func pow10(n int) int64 {
	p := int64(1)
	for range n {
		p *= 10
	}
	return p
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
