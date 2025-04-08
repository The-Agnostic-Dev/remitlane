package swiftpay

import (
	"testing"

	"github.com/remitlane/remitlane/internal/domain"
)


func TestMapV1(t *testing.T) {
	cases := map[string]domain.Status{
		"pending": domain.Processing, "successful": domain.Completed, "failed": domain.Failed, "reversed": domain.Reversed,
	}
	for in, want := range cases {
		if got := MapV1(in); got != want {
			t.Errorf("MapV1(%q) = %s, want %s", in, got, want)
		}
	}
}


func TestReasonText(t *testing.T) {
	if ReasonText("INVALID_ACCOUNT") == "" || ReasonText("SOMETHING_NEW") != "something new" {
		t.Error("reason text")
	}
}
