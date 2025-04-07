// Package providers defines the interface every payout rail implements.
package providers

import (
	"context"
	"errors"
	"fmt"

	"github.com/remitlane/remitlane/internal/domain"
	"github.com/remitlane/remitlane/internal/money"
)

// Names of the configured providers.
const (
	Swiftpay   = "swiftpay"
	MoMoBridge = "momobridge"
)

// TransferRequest asks a provider to pay a recipient.
type TransferRequest struct {
	PayoutID      string // sent as the provider-side reference
	Amount        money.Amount
	RecipientName string
	Country       string
	BankCode      string
	AccountNumber string
	MSISDN        string
	Narration     string
}

// Result is a provider's view of a transfer.
type Result struct {
	ProviderRef    string
	RawStatus      string
	Status         domain.Status
	FailureCode    string
	FailureMessage string
}

// Provider is a payout rail.
type Provider interface {
	Name() string
	Send(ctx context.Context, req TransferRequest) (Result, error)
	Status(ctx context.Context, providerRef string) (Result, error)
}

// RejectedError is a provider refusing a transfer outright. Retrying the same
// request will not help.
type RejectedError struct {
	Provider string
	Code     string
	Message  string
}

func (e *RejectedError) Error() string {
	return fmt.Sprintf("%s rejected the transfer: %s (%s)", e.Provider, e.Message, e.Code)
}

// IsRejected reports whether err is a *RejectedError.
func IsRejected(err error) bool {
	var r *RejectedError
	return errors.As(err, &r)
}

// Registry looks providers up by name.
type Registry map[string]Provider

// Get returns the named provider.
func (r Registry) Get(name string) (Provider, error) {
	p, ok := r[name]
	if !ok {
		return nil, fmt.Errorf("providers: %q is not configured", name)
	}
	return p, nil
}
