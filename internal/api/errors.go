package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/remitlane/remitlane/internal/httpx"
	"github.com/remitlane/remitlane/internal/obs"
	"github.com/remitlane/remitlane/internal/payouts"
)

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Field   string `json:"field,omitempty"`
		Item    int    `json:"item,omitempty"`
	} `json:"error"`
	RequestID string `json:"request_id"`
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, msg string) {
	var b errorBody
	b.Error.Code, b.Error.Message = code, msg
	b.RequestID = obs.RequestID(r.Context())
	httpx.WriteJSON(w, status, b)
}

func internalError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	log.Error("internal error", "request_id", obs.RequestID(r.Context()), "path", r.URL.Path, "err", err)
	writeError(w, r, http.StatusInternalServerError, "internal_error", "something went wrong on our side; it has been logged")
}

var statusByCode = map[string]int{
	payouts.CodeValidation:     http.StatusBadRequest,
	payouts.CodeQuoteNotFound:  http.StatusNotFound,
	payouts.CodePayoutNotFound: http.StatusNotFound,
	payouts.CodeDuplicateRef:   http.StatusConflict,
}

// writePayoutError renders a domain error, or a 500 for anything else.
func (a *API) writePayoutError(w http.ResponseWriter, r *http.Request, err error) {
	var pe *payouts.Error
	if !errors.As(err, &pe) {
		internalError(w, r, a.log, err)
		return
	}
	status, ok := statusByCode[pe.Code]
	if !ok {
		status = http.StatusUnprocessableEntity
	}
	var b errorBody
	b.Error.Code, b.Error.Message, b.Error.Field, b.Error.Item = pe.Code, pe.Message, pe.Field, pe.Item
	b.RequestID = obs.RequestID(r.Context())
	httpx.WriteJSON(w, status, b)
}
