package providers

import (
	"context"
	"errors"
	"log/slog"
)

// Failover sends to a primary provider and falls back to a secondary one
// when the primary is unavailable (not when it rejects the transfer).
type Failover struct {
	Primary   Provider
	Secondary Provider
	Log       *slog.Logger
}

// Name implements Provider.
func (f *Failover) Name() string { return f.Primary.Name() }

// Send implements Provider.
func (f *Failover) Send(ctx context.Context, req TransferRequest) (Result, error) {
	res, err := f.Primary.Send(ctx, req)
	if err == nil || IsRejected(err) {
		return res, err
	}
	f.Log.Warn("primary provider unavailable, failing over", "primary", f.Primary.Name(), "secondary", f.Secondary.Name(), "err", err)
	res, err2 := f.Secondary.Send(ctx, req)
	if err2 != nil {
		return Result{}, errors.Join(err, err2)
	}
	return res, nil
}

// Status implements Provider. Status is always asked of the primary.
func (f *Failover) Status(ctx context.Context, ref string) (Result, error) {
	return f.Primary.Status(ctx, ref)
}
