package payouts

import (
	"context"
	"errors"
	"time"

	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/domain"
	"github.com/remitlane/remitlane/internal/money"
	"github.com/remitlane/remitlane/internal/providers"
)

// Dispatch sends a pending payout to its provider. A temporary failure is
// returned so the job retries; on the last attempt, or when the provider
// rejects the transfer, the payout fails and its funds are released.
func (s *Service) Dispatch(ctx context.Context, payoutID string, lastAttempt bool) error {
	p, err := s.store.Q.GetPayout(ctx, payoutID)
	if err != nil {
		return err
	}
	if domain.Status(p.Status) != domain.Pending {
		return nil
	}
	prov, err := s.providers.Get(p.Provider)
	if err != nil {
		return err
	}
	res, err := prov.Send(ctx, transferRequest(p))
	if err != nil {
		var rej *providers.RejectedError
		switch {
		case errors.As(err, &rej):
			return s.fail(ctx, p, rej.Code, rej.Message)
		case lastAttempt:
			return s.fail(ctx, p, "provider_unavailable", err.Error())
		}
		s.log.Warn("dispatch failed, will retry", "payout_id", p.ID, "provider", p.Provider, "err", err)
		return err
	}
	_, err = s.Apply(ctx, p.ID, Update{
		Status: res.Status, ProviderRef: res.ProviderRef, Source: p.Provider,
		Detail: "accepted as " + res.RawStatus, FailureCode: res.FailureCode, FailureMessage: res.FailureMessage,
	})
	if errors.Is(err, ErrIgnored) {
		return nil
	}
	return err
}

func (s *Service) fail(ctx context.Context, p db.Payout, code, msg string) error {
	_, err := s.Apply(ctx, p.ID, Update{Status: domain.Failed, FailureCode: code, FailureMessage: msg, Source: p.Provider})
	if errors.Is(err, ErrIgnored) {
		return nil
	}
	return err
}

func transferRequest(p db.Payout) providers.TransferRequest {
	return providers.TransferRequest{
		PayoutID: p.ID, Amount: money.New(p.DestAmountMinor, money.Currency(p.DestCurrency)),
		RecipientName: p.RecipientName, Country: p.RecipientCountry, BankCode: p.BankCode,
		AccountNumber: p.AccountNumber, MSISDN: p.Msisdn, Narration: p.Narration,
	}
}

// PollStuck asks providers about payouts that have not moved for a while, in
// case a webhook was lost. It returns how many payouts changed.
func (s *Service) PollStuck(ctx context.Context, olderThan time.Duration, max int) (int, error) {
	stuck, err := s.store.Q.ListStuckPayouts(ctx, db.ListStuckPayoutsParams{
		UpdatedAt: time.Now().Add(-olderThan), Limit: int32(max),
	})
	if err != nil {
		return 0, err
	}
	changed := 0
	for _, p := range stuck {
		if p.ProviderRef == "" {
			continue
		}
		prov, err := s.providers.Get(p.Provider)
		if err != nil {
			return changed, err
		}
		res, err := prov.Status(ctx, p.ProviderRef)
		if err != nil {
			s.log.Warn("poll provider", "payout_id", p.ID, "err", err)
			_ = s.store.Q.TouchPayout(ctx, p.ID)
			continue
		}
		if res.Status == domain.Status(p.Status) {
			_ = s.store.Q.TouchPayout(ctx, p.ID)
			continue
		}
		_, err = s.Apply(ctx, p.ID, Update{
			Status: res.Status, Source: "poller", Detail: "polled: " + res.RawStatus,
			FailureCode: res.FailureCode, FailureMessage: res.FailureMessage,
		})
		if err != nil && !errors.Is(err, ErrIgnored) {
			return changed, err
		}
		changed++
	}
	return changed, nil
}
