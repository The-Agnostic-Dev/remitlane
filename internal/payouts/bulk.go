package payouts

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/money"
	"github.com/remitlane/remitlane/internal/rates"
	"github.com/remitlane/remitlane/internal/recipients"
)

// BulkItem is one payout in a batch. Amounts are fixed on the destination
// side: the recipient gets exactly AmountMinor.
type BulkItem struct {
	Reference      string
	SourceCurrency money.Currency
	DestCurrency   money.Currency
	Channel        rates.Channel
	AmountMinor    int64
	Narration      string
	Recipient      recipients.Recipient
}

// BulkRequest is a batch of payouts priced and booked together. Either every
// item is accepted or none is.
type BulkRequest struct {
	Reference string
	Items     []BulkItem
}

type pricedItem struct {
	item      BulkItem
	price     rates.Price
	recipient recipients.Recipient
}

// Bulk prices, validates and books a batch.
func (s *Service) Bulk(ctx context.Context, partner db.Partner, req BulkRequest) (db.PayoutBatch, []db.Payout, error) {
	if !referencePattern.MatchString(req.Reference) {
		return db.PayoutBatch{}, nil, &Error{Code: CodeValidation, Field: "reference", Message: "batch reference is required"}
	}
	if len(req.Items) == 0 || len(req.Items) > s.maxBulk {
		return db.PayoutBatch{}, nil, errorf(CodeBatchTooLarge, "a batch must have 1 to %d items", s.maxBulk)
	}

	priced := make([]pricedItem, 0, len(req.Items))
	need := map[money.Currency]int64{}
	var total int64
	for i, it := range req.Items {
		if !referencePattern.MatchString(it.Reference) {
			return db.PayoutBatch{}, nil, &Error{Code: CodeValidation, Field: "reference", Item: i + 1, Message: "item reference is required"}
		}
		price, err := s.quoter.Calculate(ctx, rates.Request{
			From: it.SourceCurrency, To: it.DestCurrency, Channel: it.Channel,
			AmountMinor: it.AmountMinor, AmountType: rates.ByDestination,
		})
		if err != nil {
			return db.PayoutBatch{}, nil, itemError(i, priceError(err))
		}
		rcp, err := recipients.Validate(it.Recipient, it.DestCurrency, it.Channel)
		if err != nil {
			return db.PayoutBatch{}, nil, itemError(i, recipientError(err))
		}
		priced = append(priced, pricedItem{item: it, price: price, recipient: rcp})
		need[it.SourceCurrency] += price.SourceMinor + price.FeeMinor
		total += it.AmountMinor
	}
	if err := s.limits.Check(ctx, partner, total); err != nil {
		return db.PayoutBatch{}, nil, limitError(err)
	}

	var batch db.PayoutBatch
	payouts := make([]db.Payout, 0, len(priced))
	err := s.store.InTx(ctx, func(q *db.Queries, tx pgx.Tx) error {
		for c, amount := range need {
			if err := checkFunds(ctx, tx, partner.ID, c, amount); err != nil {
				return err
			}
		}
		var err error
		batch, err = q.CreateBatch(ctx, db.CreateBatchParams{
			ID: newID("bat_"), PartnerID: partner.ID, Reference: req.Reference, ItemCount: int32(len(priced)),
		})
		if err != nil {
			return duplicateBatch(err, req.Reference)
		}
		for i, pi := range priced {
			usd, err := rates.ToUSD(ctx, s.feed, money.New(pi.price.SourceMinor, pi.item.SourceCurrency))
			if err != nil {
				return err
			}
			p, err := s.insert(ctx, q, newPayout{
				partnerID: partner.ID, batchID: batch.ID, reference: pi.item.Reference, channel: pi.item.Channel,
				src: money.New(pi.price.SourceMinor, pi.item.SourceCurrency), fee: pi.price.FeeMinor,
				dst: money.New(pi.price.DestMinor, pi.item.DestCurrency), usd: usd,
				rate: pi.price.Rate.FloatString(8), recipient: pi.recipient, narration: pi.item.Narration,
			})
			if err != nil {
				return itemError(i, err)
			}
			payouts = append(payouts, p)
		}
		return nil
	})
	if err != nil {
		return db.PayoutBatch{}, nil, err
	}
	ids := make([]string, len(payouts))
	for i, p := range payouts {
		ids[i] = p.ID
	}
	s.afterCreate(ctx, partner.ID, ids...)
	return batch, payouts, nil
}

func itemError(i int, err error) error {
	var pe *Error
	if errors.As(err, &pe) {
		cp := *pe
		cp.Item = i + 1
		return &cp
	}
	return err
}

func duplicateBatch(err error, ref string) error {
	var pe *Error
	if errors.As(err, &pe) {
		return err
	}
	if isUniqueViolation(err) {
		return &Error{Code: CodeDuplicateRef, Field: "reference", Message: "a batch with reference " + ref + " already exists"}
	}
	return err
}

// priceError maps quoting errors to payout errors.
func priceError(err error) error {
	switch {
	case errors.Is(err, rates.ErrNoCorridor):
		return &Error{Code: CodeCorridor, Message: err.Error()}
	case errors.Is(err, rates.ErrAmountRange), errors.Is(err, rates.ErrBadAmount):
		return &Error{Code: CodeAmount, Message: err.Error()}
	case errors.Is(err, rates.ErrBadAmountTyp):
		return &Error{Code: CodeValidation, Field: "amount_type", Message: err.Error()}
	}
	return err
}

// PriceError is priceError for the API package's quote endpoint.
func PriceError(err error) error { return priceError(err) }
