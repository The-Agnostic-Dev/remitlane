package momobridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/remitlane/remitlane/internal/domain"
	"github.com/remitlane/remitlane/internal/money"
	"github.com/remitlane/remitlane/internal/providers"
)

func TestMapStatus(t *testing.T) {
	for in, want := range map[string]domain.Status{
		"PENDING": domain.Processing, "SUCCESSFUL": domain.Completed, "FAILED": domain.Failed, "pending": domain.Processing,
	} {
		if got := MapStatus(in); got != want {
			t.Errorf("MapStatus(%q) = %s", in, got)
		}
	}
}

func TestSendSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "k" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["amount"] != "1500.00" || body["msisdn"] != "254712345678" {
			t.Errorf("unexpected body %v", body)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"disbursement_id":"MMB-1","external_id":"po_1","status":"PENDING"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "k", srv.Client())
	res, err := c.Send(context.Background(), providers.TransferRequest{
		PayoutID: "po_1", Amount: money.New(150000, money.KES), MSISDN: "254712345678", Country: "KE", RecipientName: "Achieng",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ProviderRef != "MMB-1" || res.Status != domain.Processing {
		t.Errorf("got %+v", res)
	}
}

func TestSendRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":"SUBSCRIBER_NOT_FOUND","message":"no wallet for this number"}`))
	}))
	defer srv.Close()
	_, err := New(srv.URL, "k", srv.Client()).Send(context.Background(), providers.TransferRequest{Amount: money.New(100, money.GHS)})
	if !providers.IsRejected(err) {
		t.Fatalf("want rejection, got %v", err)
	}
}
