package inbound

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/remitlane/remitlane/internal/payouts"
	"github.com/remitlane/remitlane/internal/providers"
	"github.com/remitlane/remitlane/internal/providers/momobridge"
	"github.com/remitlane/remitlane/internal/sig"
)

func (h *Handler) momobridge(w http.ResponseWriter, r *http.Request) {
	body, err := readBody(r)
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}
	if err := sig.VerifyBase64(h.momoSecret, body, r.Header.Get(momobridge.SignatureHeader)); err != nil {
		h.record(r.Context(), providers.MoMoBridge, "", "", false, "bad_signature", body)
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	var cb momobridge.Callback
	if err := json.Unmarshal(body, &cb); err != nil {
		h.record(r.Context(), providers.MoMoBridge, "", "", true, "unparseable", body)
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	st := momobridge.MapStatus(cb.Status)
	u := payouts.Update{
		Status: st, ProviderRef: cb.DisbursementID, Source: providers.MoMoBridge, Detail: "callback: " + cb.Status,
	}
	if st.Terminal() && cb.Reason != "" {
		u.FailureCode = cb.Reason
		u.FailureMessage = strings.ReplaceAll(strings.ToLower(cb.Reason), "_", " ")
	}
	outcome := h.apply(r.Context(), cb.ExternalID, u)
	h.record(r.Context(), providers.MoMoBridge, cb.ExternalID, cb.Status, true, outcome, body)
	ack(w, outcome)
}
