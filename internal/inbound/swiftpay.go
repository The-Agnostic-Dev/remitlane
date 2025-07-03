package inbound

import (
	"net/http"

	"github.com/remitlane/remitlane/internal/payouts"
	"github.com/remitlane/remitlane/internal/providers"
	"github.com/remitlane/remitlane/internal/providers/swiftpay"
	"github.com/remitlane/remitlane/internal/sig"
)

func (h *Handler) swiftpay(w http.ResponseWriter, r *http.Request) {
	body, err := readBody(r)
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}
	if err := sig.VerifyHex(h.swiftpaySecret, body, r.Header.Get(swiftpay.SignatureHeader)); err != nil {
		h.record(r.Context(), providers.Swiftpay, "", "", false, "bad_signature", body)
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	u, err := swiftpay.ParseWebhook(body)
	if err != nil {
		h.record(r.Context(), providers.Swiftpay, "", "", true, "unparseable", body)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	outcome := h.apply(r.Context(), u.Reference, payouts.Update{
		Status: u.Status, ProviderRef: u.ProviderRef, Source: providers.Swiftpay,
		Detail:      u.Version + " webhook: " + u.RawStatus,
		FailureCode: failureCode(u), FailureMessage: failureMessage(u),
	})
	h.record(r.Context(), providers.Swiftpay, u.Reference, u.RawStatus, true, outcome, body)
	ack(w, outcome)
}

func failureCode(u swiftpay.Update) string {
	if u.Status.Terminal() {
		return u.Reason
	}
	return ""
}

func failureMessage(u swiftpay.Update) string {
	if u.Status.Terminal() && u.Reason != "" {
		return swiftpay.ReasonText(u.Reason)
	}
	return ""
}
