package swiftpay

import (
	"encoding/json"
	"errors"

	"github.com/remitlane/remitlane/internal/domain"
)

// SignatureHeader carries "sha256=<hex hmac of the raw body>".
const SignatureHeader = "X-Swiftpay-Signature"

// Update is a parsed status webhook, v1 or v2.
type Update struct {
	Version     string
	ProviderRef string
	Reference   string
	RawStatus   string
	Status      domain.Status
	Reason      string
}

// WebhookV2 is the v2 webhook body.
type WebhookV2 struct {
	Event string   `json:"event"`
	Data  Transfer `json:"data"`
}

// WebhookV1 is the legacy v1 webhook body.
type WebhookV1 struct {
	Type       string `json:"type"`
	TransferID string `json:"transfer_id"`
	Ref        string `json:"ref"`
	State      string `json:"state"`
	Reason     string `json:"reason,omitempty"`
}

// ParseWebhook reads either webhook version.
func ParseWebhook(body []byte) (Update, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(body, &probe); err != nil {
		return Update{}, err
	}
	if _, ok := probe["event"]; ok {
		var w WebhookV2
		if err := json.Unmarshal(body, &w); err != nil {
			return Update{}, err
		}
		return Update{
			Version: "v2", ProviderRef: w.Data.ID, Reference: w.Data.Reference,
			RawStatus: w.Data.Status, Status: MapV2(w.Data.Status), Reason: w.Data.Reason,
		}, nil
	}
	if _, ok := probe["type"]; ok {
		var w WebhookV1
		if err := json.Unmarshal(body, &w); err != nil {
			return Update{}, err
		}
		return Update{
			Version: "v1", ProviderRef: w.TransferID, Reference: w.Ref,
			RawStatus: w.State, Status: MapV1(w.State), Reason: w.Reason,
		}, nil
	}
	return Update{}, errors.New("swiftpay: unrecognised webhook body")
}
