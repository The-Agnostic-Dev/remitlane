package swiftpay

import (
	"strings"

	"github.com/remitlane/remitlane/internal/domain"
)


// MapV1 maps a Swiftpay transfer state to a payout status.
func MapV1(s string) domain.Status {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "pending", "processing":
		return domain.Processing
	case "successful":
		return domain.Completed
	case "reversed":
		return domain.Reversed
	default:
		return domain.Failed
	}
}

var reasons = map[string]string{
	"INVALID_ACCOUNT":   "The account number does not exist at this bank",
	"ACCOUNT_DORMANT":   "The recipient's account is dormant",
	"BANK_UNAVAILABLE":  "The recipient's bank is not responding",
	"COMPLIANCE_REVIEW": "Held for compliance review",
	"RETURNED_BY_BANK":  "The recipient's bank returned the funds",
	"LIMIT_EXCEEDED":    "The amount is above the recipient bank's limit",
}

// ReasonText is a human message for a Swiftpay reason code.
func ReasonText(code string) string {
	if t, ok := reasons[code]; ok {
		return t
	}
	return strings.ReplaceAll(strings.ToLower(code), "_", " ")
}
