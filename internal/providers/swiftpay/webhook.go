package swiftpay

import (
	"encoding/json"

	"github.com/remitlane/remitlane/internal/domain"
)

// SignatureHeader carries "sha256=<hex hmac of the raw body>".
const SignatureHeader = "X-Swiftpay-Signature"

// Update is a parsed status webhook.
type Update struct {
	ProviderRef string
	Reference   string
	RawStatus   string
	Status      domain.Status
	Reason      string
}

// WebhookV1 is the webhook body.
type WebhookV1 struct {
	Type       string `json:"type"`
	TransferID string `json:"transfer_id"`
	Ref        string `json:"ref"`
	State      string `json:"state"`
	Reason     string `json:"reason,omitempty"`
}

// ParseWebhook reads a webhook body.
func ParseWebhook(body []byte) (Update, error) {
	var w WebhookV1
	if err := json.Unmarshal(body, &w); err != nil {
		return Update{}, err
	}
	return Update{ProviderRef: w.TransferID, Reference: w.Ref, RawStatus: w.State, Status: MapV1(w.State), Reason: w.Reason}, nil
}
