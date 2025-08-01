package ledger

import (
	"testing"

	"github.com/remitlane/remitlane/internal/money"
)

func sum(entries []Entry) int64 {
	var s int64
	for _, e := range entries {
		s += e.Minor
	}
	return s
}

func TestRecipesBalance(t *testing.T) {
	p := Payout{ID: "po_1", PartnerID: "prt_1", Provider: "swiftpay", Currency: money.USD, Amount: 12345, Fee: 150}
	for name, entries := range map[string][]Entry{
		"hold": Hold(p), "settle": Settle(p), "release": Release(p), "reverse": Reverse(p),
		"fund": Fund("prt_1", money.New(100000, money.USD)),
	} {
		if s := sum(entries); s != 0 {
			t.Errorf("%s sums to %d", name, s)
		}
	}
}

func TestHoldThenReleaseRestoresAvailable(t *testing.T) {
	p := Payout{PartnerID: "prt_1", Provider: "momobridge", Currency: money.USD, Amount: 5000, Fee: 75}
	avail := Available("prt_1", money.USD).Code
	var net int64
	for _, e := range append(Hold(p), Release(p)...) {
		if e.Account.Code == avail {
			net += e.Minor
		}
	}
	if net != 0 {
		t.Errorf("available moved by %d", net)
	}
}

func TestShortCode(t *testing.T) {
	if got := ShortCode("partner:prt_kobopay:available:USD"); got != "available USD" {
		t.Errorf("got %q", got)
	}
	if got := ShortCode("revenue:fees:USD"); got != "revenue:fees:USD" {
		t.Errorf("got %q", got)
	}
}
