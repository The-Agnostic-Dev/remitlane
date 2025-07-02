package swiftpay

import (
	"strings"

	"github.com/remitlane/remitlane/internal/domain"
)

// MapV2 maps a v2 transfer status to a payout status.
//
// v2 statuses: QUEUED, PROCESSING, ON_HOLD, COMPLETED, FAILED, REVERSED.
func MapV2(s string) domain.Status {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "QUEUED", "PROCESSING":
		return domain.Processing
	case "COMPLETED":
		return domain.Completed
	case "REVERSED":
		return domain.Reversed
	default:
		return domain.Failed
	}
}

// MapV1 maps a legacy v1 webhook state. Some older Swiftpay accounts still
// send v1 webhooks; see docs/sandbox.md.
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
