package sig

import (
	"errors"
	"testing"
)

func TestHexRoundTrip(t *testing.T) {
	body := []byte(`{"event":"transfer.updated"}`)
	s := Hex("secret", body)
	if err := VerifyHex("secret", body, "sha256="+s); err != nil {
		t.Fatal(err)
	}
	if err := VerifyHex("secret", body, s); err != nil {
		t.Fatal(err)
	}
	if err := VerifyHex("other", body, s); !errors.Is(err, ErrMismatch) {
		t.Fatalf("want mismatch, got %v", err)
	}
	if err := VerifyHex("secret", body, "zz"); !errors.Is(err, ErrMismatch) {
		t.Fatalf("want mismatch for junk, got %v", err)
	}
}

func TestBase64RoundTrip(t *testing.T) {
	body := []byte(`{"status":"SUCCESSFUL"}`)
	if err := VerifyBase64("k", body, Base64("k", body)); err != nil {
		t.Fatal(err)
	}
	if err := VerifyBase64("k", []byte(`{"status":"FAILED"}`), Base64("k", body)); !errors.Is(err, ErrMismatch) {
		t.Fatalf("want mismatch, got %v", err)
	}
}

func TestVerifyPayload(t *testing.T) {
	type cb struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	body := []byte(`{"id":"mmb_1","status":"SUCCESSFUL"}`)
	if err := VerifyPayload("k", cb{"mmb_1", "SUCCESSFUL"}, Base64("k", body)); err != nil {
		t.Fatal(err)
	}
}
