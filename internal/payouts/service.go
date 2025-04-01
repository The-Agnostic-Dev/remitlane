// Package payouts creates payouts and moves them through their lifecycle.
package payouts

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/remitlane/remitlane/internal/clock"
	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/domain"
	"github.com/remitlane/remitlane/internal/ledger"
	"github.com/remitlane/remitlane/internal/limits"
	"github.com/remitlane/remitlane/internal/money"
	"github.com/remitlane/remitlane/internal/outbound"
	"github.com/remitlane/remitlane/internal/providers"
	"github.com/remitlane/remitlane/internal/queue"
	"github.com/remitlane/remitlane/internal/rates"
	"github.com/remitlane/remitlane/internal/recipients"
	"github.com/remitlane/remitlane/internal/store"
)

// Service is the payout service.
type Service struct {
	store     *store.Store
	quoter    *rates.Quoter
	feed      rates.Feed
	limits    *limits.Service
	events    *outbound.Service
	queue     queue.Enqueuer
	providers providers.Registry
	clock     *clock.Clock
	log       *slog.Logger
	maxBulk   int
}

// Deps are the Service's collaborators.
type Deps struct {
	Store     *store.Store
	Quoter    *rates.Quoter
	Feed      rates.Feed
	Limits    *limits.Service
	Events    *outbound.Service
	Queue     queue.Enqueuer
	Providers providers.Registry
	Clock     *clock.Clock
	Log       *slog.Logger
	MaxBulk   int
}

// New returns a Service.
func New(d Deps) *Service {
	return &Service{
		store: d.Store, quoter: d.Quoter, feed: d.Feed, limits: d.Limits, events: d.Events,
		queue: d.Queue, providers: d.Providers, clock: d.Clock, log: d.Log.With("component", "payouts"),
		maxBulk: d.MaxBulk,
	}
}

// CreateRequest is a single payout against a quote.
type CreateRequest struct {
	QuoteID   string
	Reference string
	Narration string
	Recipient recipients.Recipient
}

var referencePattern = regexp.MustCompile(`^[A-Za-z0-9._:/-]{1,64}$`)

// Create books a payout against a quote and queues it for dispatch.
func (s *Service) Create(ctx context.Context, partner db.Partner, req CreateRequest) (db.Payout, error) {
	if !referencePattern.MatchString(req.Reference) {
		return db.Payout{}, &Error{Code: CodeValidation, Field: "reference",
			Message: "reference must be 1 to 64 letters, digits or . _ : / -"}
	}
	quote, err := s.store.Q.GetQuoteForPartner(ctx, db.GetQuoteForPartnerParams{ID: req.QuoteID, PartnerID: partner.ID})
	if store.IsNotFound(err) {
		return db.Payout{}, errorf(CodeQuoteNotFound, "no quote %q", req.QuoteID)
	}
	if err != nil {
		return db.Payout{}, err
	}
	if quote.UsedAt.Valid {
		return db.Payout{}, errorf(CodeQuoteUsed, "quote %s has already been used", quote.ID)
	}
	if !time.Now().Before(quote.ExpiresAt) {
		return db.Payout{}, errorf(CodeQuoteExpired, "quote %s expired; request a new quote", quote.ID)
	}
	src, dst := money.Currency(quote.SourceCurrency), money.Currency(quote.DestCurrency)
	ch := rates.Channel(quote.Channel)
	rcp, err := recipients.Validate(req.Recipient, dst, ch)
	if err != nil {
		return db.Payout{}, recipientError(err)
	}
	usd, err := rates.ToUSD(ctx, s.feed, money.New(quote.SourceAmountMinor, src))
	if err != nil {
		return db.Payout{}, err
	}
	if err := s.limits.Check(ctx, partner, usd); err != nil {
		return db.Payout{}, limitError(err)
	}

	in := newPayout{
		partnerID: partner.ID, quoteID: quote.ID, reference: req.Reference, channel: ch,
		src: money.New(quote.SourceAmountMinor, src), fee: quote.FeeMinor, dst: money.New(quote.DestAmountMinor, dst),
		usd: usd, rate: quote.Rate, recipient: rcp, narration: req.Narration,
	}
	var p db.Payout
	err = s.store.InTx(ctx, func(q *db.Queries, tx pgx.Tx) error {
		if err := checkFunds(ctx, tx, partner.ID, in.src.Currency, in.src.Minor+in.fee); err != nil {
			return err
		}
		if p, err = s.insert(ctx, q, in); err != nil {
			return err
		}
		return q.MarkQuoteUsed(ctx, quote.ID)
	})
	if err != nil {
		return db.Payout{}, err
	}
	s.afterCreate(ctx, partner.ID, p.ID)
	return p, nil
}

type newPayout struct {
	partnerID string
	quoteID   string
	batchID   string
	reference string
	channel   rates.Channel
	src       money.Amount
	fee       int64
	dst       money.Amount
	usd       int64
	rate      string
	recipient recipients.Recipient
	narration string
}

// insert writes the payout, its hold and its first event.
func (s *Service) insert(ctx context.Context, q *db.Queries, in newPayout) (db.Payout, error) {
	p, err := q.CreatePayout(ctx, db.CreatePayoutParams{
		ID: newID("po_"), PartnerID: in.partnerID, QuoteID: text(in.quoteID), BatchID: text(in.batchID),
		Reference: in.reference, Status: string(domain.Pending), Channel: string(in.channel),
		Provider: ProviderFor(in.channel), SourceCurrency: string(in.src.Currency), SourceAmountMinor: in.src.Minor,
		FeeMinor: in.fee, DestCurrency: string(in.dst.Currency), DestAmountMinor: in.dst.Minor,
		UsdAmountMinor: in.usd, Rate: in.rate, RecipientName: in.recipient.Name,
		RecipientCountry: in.recipient.Country, BankCode: in.recipient.BankCode,
		AccountNumber: in.recipient.AccountNumber, Msisdn: in.recipient.MSISDN, Narration: in.narration,
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return db.Payout{}, &Error{Code: CodeDuplicateRef, Field: "reference",
			Message: fmt.Sprintf("a payout with reference %q already exists", in.reference)}
	}
	if err != nil {
		return db.Payout{}, err
	}
	if _, err := ledger.Post(ctx, q, "payout_hold", p.ID, "", ledger.Hold(ledgerPayout(p))...); err != nil {
		return db.Payout{}, err
	}
	err = q.InsertPayoutEvent(ctx, db.InsertPayoutEventParams{PayoutID: p.ID, Status: p.Status, Source: "api", Detail: "created"})
	return p, err
}

func (s *Service) afterCreate(ctx context.Context, partnerID string, ids ...string) {
	for _, id := range ids {
		if err := s.queue.Dispatch(ctx, id); err != nil {
			s.log.Error("enqueue dispatch", "payout_id", id, "err", err)
		}
	}
	if err := s.queue.LowBalance(ctx, partnerID, s.clock.Today()); err != nil {
		s.log.Warn("enqueue low balance check", "partner_id", partnerID, "err", err)
	}
}

// checkFunds locks the partner's available account and checks the balance.
func checkFunds(ctx context.Context, tx pgx.Tx, partnerID string, c money.Currency, need int64) error {
	bal, err := ledger.LockedBalance(ctx, tx, ledger.Available(partnerID, c))
	if err != nil {
		return err
	}
	if bal < need {
		return errorf(CodeFunds, "available %s balance is %s, payout needs %s",
			c, money.Format(bal, c), money.Format(need, c))
	}
	return nil
}

// ProviderFor routes a channel to its provider.
func ProviderFor(ch rates.Channel) string {
	if ch == rates.MobileMoney {
		return providers.MoMoBridge
	}
	return providers.Swiftpay
}

func ledgerPayout(p db.Payout) ledger.Payout {
	return ledger.Payout{
		ID: p.ID, PartnerID: p.PartnerID, Provider: p.Provider, Currency: money.Currency(p.SourceCurrency),
		Amount: p.SourceAmountMinor, Fee: p.FeeMinor,
	}
}

func recipientError(err error) error {
	var fe *recipients.FieldError
	if errors.As(err, &fe) {
		return &Error{Code: CodeRecipient, Field: fe.Field, Message: fe.Message}
	}
	return err
}

func limitError(err error) error {
	var le *limits.ExceededError
	if errors.As(err, &le) {
		return &Error{Code: CodeDailyLimit, Message: le.Error()}
	}
	return err
}

func newID(prefix string) string {
	b := make([]byte, 10)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
