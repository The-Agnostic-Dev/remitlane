package recipients

import (
	"errors"
	"testing"

	"github.com/remitlane/remitlane/internal/money"
	"github.com/remitlane/remitlane/internal/rates"
)

func TestNormalizeMSISDN(t *testing.T) {
	cases := []struct {
		raw, country, want string
	}{
		{"+254 712 345 678", "KE", "254712345678"},
		{"0712345678", "KE", "254712345678"},
		{"0110345678", "KE", "254110345678"},
		{"00256772123456", "UG", "256772123456"},
		{"0244123456", "GH", "233244123456"},
		{"+233 55 412 3456", "GH", "233554123456"},
	}
	for _, tc := range cases {
		got, err := NormalizeMSISDN(tc.raw, tc.country)
		if err != nil || got != tc.want {
			t.Errorf("NormalizeMSISDN(%q, %s) = %q, %v; want %q", tc.raw, tc.country, got, err, tc.want)
		}
	}
}

func TestNormalizeMSISDNRejects(t *testing.T) {
	for _, tc := range []struct{ raw, country string }{
		{"0201234567", "KE"},
		{"0302123456", "GH"}, // Accra landline
		{"07123", "KE"},
		{"0712abc678", "KE"},
		{"", "UG"},
		{"0803 447 1205", "NG"},
	} {
		if _, err := NormalizeMSISDN(tc.raw, tc.country); err == nil {
			t.Errorf("NormalizeMSISDN(%q, %s) accepted", tc.raw, tc.country)
		}
	}
}

func TestValidateBank(t *testing.T) {
	r, err := Validate(Recipient{Name: "  Adaeze   Okoro ", Country: "ng", BankCode: "058", AccountNumber: "0123 456 789"}, money.NGN, rates.Bank)
	if err != nil {
		t.Fatal(err)
	}
	if r.Name != "Adaeze Okoro" || r.Country != "NG" || r.AccountNumber != "0123456789" {
		t.Errorf("normalised to %+v", r)
	}
	_, err = Validate(Recipient{Name: "Adaeze Okoro", Country: "NG", BankCode: "058", AccountNumber: "12345"}, money.NGN, rates.Bank)
	var fe *FieldError
	if !errors.As(err, &fe) || fe.Field != "recipient.account_number" {
		t.Errorf("want account_number error, got %v", err)
	}
}

func TestValidateCountryMustMatchCurrency(t *testing.T) {
	_, err := Validate(Recipient{Name: "Brian Otieno", Country: "UG", MSISDN: "0712345678"}, money.KES, rates.MobileMoney)
	if err == nil {
		t.Fatal("KES payout to a UG recipient accepted")
	}
}
