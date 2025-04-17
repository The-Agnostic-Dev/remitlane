package outbound

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/remitlane/remitlane/internal/sig"
)

func TestSign(t *testing.T) {
	ts := time.Unix(1762430400, 0)
	body := []byte(`{"id":"evt_1"}`)
	got := Sign("whsec", ts, body)
	if !strings.HasPrefix(got, "t=1762430400,v1=") {
		t.Fatalf("got %q", got)
	}
	mac := strings.TrimPrefix(got, "t=1762430400,v1=")
	if err := sig.VerifyHex("whsec", []byte(`1762430400.{"id":"evt_1"}`), mac); err != nil {
		t.Errorf("signature does not verify: %v", err)
	}
}

func TestRetryable(t *testing.T) {
	for _, code := range []int{0, http.StatusInternalServerError, http.StatusBadGateway, http.StatusTooManyRequests} {
		if !(&AttemptError{Status: code}).Retryable() {
			t.Errorf("%d should be retried", code)
		}
	}
	for _, code := range []int{http.StatusBadRequest, http.StatusNotFound, http.StatusGone} {
		if (&AttemptError{Status: code}).Retryable() {
			t.Errorf("%d should not be retried", code)
		}
	}
}
