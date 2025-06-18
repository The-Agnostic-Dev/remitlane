package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/payouts"
	"github.com/remitlane/remitlane/internal/quotes"
	"github.com/remitlane/remitlane/internal/rates"
)

func testAPI() *API {
	return New(Deps{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), RatePerMinute: 600})
}

func TestMissingKeyIsUnauthorized(t *testing.T) {
	rec := httptest.NewRecorder()
	testAPI().Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/payouts", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error.Code != "unauthorized" {
		t.Errorf("body %s", rec.Body.String())
	}
}

func TestPayoutErrorStatus(t *testing.T) {
	a := testAPI()
	cases := map[string]int{
		payouts.CodeDuplicateRef:  http.StatusConflict,
		payouts.CodeQuoteNotFound: http.StatusNotFound,
		payouts.CodeDailyLimit:    http.StatusUnprocessableEntity,
		payouts.CodeValidation:    http.StatusBadRequest,
	}
	for code, want := range cases {
		rec := httptest.NewRecorder()
		a.writePayoutError(rec, httptest.NewRequest(http.MethodPost, "/payouts", nil), &payouts.Error{Code: code, Message: "x", Item: 2})
		if rec.Code != want {
			t.Errorf("%s: status %d, want %d", code, rec.Code, want)
		}
	}
	rec := httptest.NewRecorder()
	a.writePayoutError(rec, httptest.NewRequest(http.MethodPost, "/payouts", nil), errors.New("boom"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("plain error: %d", rec.Code)
	}
}


