package outbound

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/remitlane/remitlane/internal/sig"
)


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
