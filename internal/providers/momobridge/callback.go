package momobridge

// SignatureHeader carries the base64 HMAC-SHA256 of the callback body.
const SignatureHeader = "X-MoMoBridge-Signature"

// Callback is the body MoMoBridge posts when a disbursement settles.
type Callback struct {
	DisbursementID string `json:"disbursement_id"`
	ExternalID     string `json:"external_id"`
	Status         string `json:"status"`
	Amount         string `json:"amount"`
	Currency       string `json:"currency"`
	MSISDN         string `json:"msisdn"`
	PayeeName      string `json:"payee_name"`
	Reason         string `json:"reason,omitempty"`
	CompletedAt    string `json:"completed_at"`
}
