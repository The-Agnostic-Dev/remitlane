package swiftpay

import (
	"testing"

	"github.com/remitlane/remitlane/internal/domain"
)

func TestMapV2(t *testing.T) {
	cases := map[string]domain.Status{
		"QUEUED": domain.Processing, "PROCESSING": domain.Processing, "processing": domain.Processing,
		"COMPLETED": domain.Completed, "FAILED": domain.Failed, "REVERSED": domain.Reversed,
	}
	for in, want := range cases {
		if got := MapV2(in); got != want {
			t.Errorf("MapV2(%q) = %s, want %s", in, got, want)
		}
	}
}

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

func TestParseWebhook(t *testing.T) {
	u, err := ParseWebhook([]byte(`{"event":"transfer.updated","data":{"id":"SWP2_1","reference":"po_1","status":"COMPLETED","amount":"100.00","currency":"NGN"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if u.Version != "v2" || u.Reference != "po_1" || u.Status != domain.Completed {
		t.Errorf("v2 parsed as %+v", u)
	}
	u, err = ParseWebhook([]byte(`{"type":"transfer.status","transfer_id":"SWP_9","ref":"po_2","state":"failed","reason":"INVALID_ACCOUNT"}`))
	if err != nil {
		t.Fatal(err)
	}
	if u.Version != "v1" || u.ProviderRef != "SWP_9" || u.Status != domain.Failed || u.Reason != "INVALID_ACCOUNT" {
		t.Errorf("v1 parsed as %+v", u)
	}
	if _, err := ParseWebhook([]byte(`{"hello":1}`)); err == nil {
		t.Error("expected error for unknown body")
	}
}

func TestReasonText(t *testing.T) {
	if ReasonText("INVALID_ACCOUNT") == "" || ReasonText("SOMETHING_NEW") != "something new" {
		t.Error("reason text")
	}
}
