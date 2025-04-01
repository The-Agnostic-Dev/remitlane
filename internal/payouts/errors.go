package payouts

import "fmt"

// Error is a payout request the partner must change. Code is stable and
// documented in docs/api.md; Message is for people.
type Error struct {
	Code    string
	Message string
	Field   string
	Item    int // 1-based bulk item, 0 when not a bulk item
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Error codes.
const (
	CodeValidation     = "validation_error"
	CodeQuoteNotFound  = "quote_not_found"
	CodeQuoteExpired   = "quote_expired"
	CodeQuoteUsed      = "quote_already_used"
	CodeRecipient      = "invalid_recipient"
	CodeDailyLimit     = "daily_limit_exceeded"
	CodeFunds          = "insufficient_funds"
	CodeDuplicateRef   = "duplicate_reference"
	CodeCorridor       = "corridor_not_supported"
	CodeAmount         = "amount_out_of_range"
	CodeBatchTooLarge  = "batch_too_large"
	CodePayoutNotFound = "payout_not_found"
)

func errorf(code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}
