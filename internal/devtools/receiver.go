package devtools

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/httpx"
	"github.com/remitlane/remitlane/internal/store"
)

// receive is the fake partner endpoint. It records the webhook and answers
// with the status configured for the partner (200 unless set otherwise).
func (t *Tools) receive(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	body, _ := io.ReadAll(io.LimitReader(r.Body, httpx.MaxBody))
	var ev struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(body, &ev)

	status := http.StatusOK
	cfg, err := t.store.Q.GetReceiverConfig(r.Context(), slug)
	if err == nil && cfg.RespondWith >= 100 {
		status = int(cfg.RespondWith)
	} else if err != nil && !store.IsNotFound(err) {
		t.log.Error("receiver config", "slug", slug, "err", err)
	}
	if err := t.store.Q.InsertReceiverHit(r.Context(), db.InsertReceiverHitParams{
		Slug: slug, EventID: ev.ID, EventType: ev.Type, PayoutID: ev.Data.ID,
		Signature: r.Header.Get("Remitlane-Signature"), Responded: int32(status), Body: string(body),
	}); err != nil {
		t.log.Error("record receiver hit", "err", err)
	}
	httpx.WriteJSON(w, status, map[string]any{"received": status < 300, "event_id": ev.ID})
}

func (t *Tools) configure(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	code, err := strconv.Atoi(r.FormValue("respond_with"))
	if err != nil || code < 100 || code > 599 {
		http.Error(w, "respond_with must be an HTTP status code", http.StatusBadRequest)
		return
	}
	if err := t.store.Q.UpsertReceiverConfig(r.Context(), db.UpsertReceiverConfigParams{
		Slug: slug, RespondWith: int32(code), Note: r.FormValue("note"),
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/_dev/receiver", http.StatusSeeOther)
}

type receiverPage struct {
	Config []db.ReceiverConfig
	Hits   []db.ReceiverHit
	Slug   string
}

func (t *Tools) hits(r *http.Request, limit int32) ([]db.ReceiverHit, error) {
	q := r.URL.Query()
	return t.store.Q.ListReceiverHits(r.Context(), db.ListReceiverHitsParams{
		Slug:     pgtype.Text{String: q.Get("partner"), Valid: q.Get("partner") != ""},
		PayoutID: pgtype.Text{String: q.Get("payout"), Valid: q.Get("payout") != ""},
		MaxRows:  limit,
	})
}

func (t *Tools) receiverPage(w http.ResponseWriter, r *http.Request) {
	hits, err := t.hits(r, 200)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cfg, err := t.store.Q.ListReceiverConfig(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	t.page(w, "receiver.html", receiverPage{Config: cfg, Hits: hits, Slug: r.URL.Query().Get("partner")})
}

type hitJSON struct {
	ID         int64  `json:"id"`
	Partner    string `json:"partner"`
	EventID    string `json:"event_id"`
	EventType  string `json:"event_type"`
	PayoutID   string `json:"payout_id"`
	Responded  int32  `json:"responded"`
	ReceivedAt string `json:"received_at"`
}

func (t *Tools) receiverJSON(w http.ResponseWriter, r *http.Request) {
	hits, err := t.hits(r, 1000)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]hitJSON, 0, len(hits))
	for _, h := range hits {
		out = append(out, hitJSON{h.ID, h.Slug, h.EventID, h.EventType, h.PayoutID, h.Responded, h.ReceivedAt.UTC().Format("2006-01-02T15:04:05.000Z")})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"hits": out})
}
