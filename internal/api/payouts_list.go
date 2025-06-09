package api

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/httpx"
	"github.com/remitlane/remitlane/internal/store"
	"github.com/remitlane/remitlane/internal/wire"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

type listResponse struct {
	Object     string        `json:"object"`
	Data       []wire.Payout `json:"data"`
	HasMore    bool          `json:"has_more"`
	NextCursor string        `json:"next_cursor,omitempty"`
}

// encodeCursor makes the opaque cursor for the page after p.
func encodeCursor(p db.Payout) string {
	return base64.RawURLEncoding.EncodeToString([]byte(p.CreatedAt.UTC().Format(time.RFC3339Nano)))
}

func decodeCursor(s string) (time.Time, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, errors.New("cursor is not valid")
	}
	t, err := time.Parse(time.RFC3339Nano, string(raw))
	if err != nil {
		return time.Time{}, errors.New("cursor is not valid")
	}
	return t, nil
}

func (a *API) listPayouts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := defaultPageSize
	if s := q.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > maxPageSize {
			writeError(w, r, http.StatusBadRequest, "validation_error", "limit must be between 1 and 100")
			return
		}
		limit = n
	}
	f := store.PartnerPayoutFilter{
		PartnerID: partnerFrom(r.Context()).ID, Reference: q.Get("reference"), BatchID: q.Get("batch_id"), Limit: limit,
	}
	if st := q.Get("status"); st != "" {
		f.Status = st
	}
	if c := q.Get("cursor"); c != "" {
		before, err := decodeCursor(c)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "validation_error", err.Error())
			return
		}
		f.Before = before
	}
	rows, more, err := a.store.ListPartnerPayouts(r.Context(), f)
	if err != nil {
		internalError(w, r, a.log, err)
		return
	}
	resp := listResponse{Object: "list", Data: make([]wire.Payout, 0, len(rows)), HasMore: more}
	for _, p := range rows {
		resp.Data = append(resp.Data, wire.FromPayout(p))
	}
	if more {
		resp.NextCursor = encodeCursor(rows[len(rows)-1])
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

