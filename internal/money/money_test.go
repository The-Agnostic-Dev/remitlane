package money

import (
	"math/big"
	"testing"
)

func TestMinorToDecimal(t *testing.T) {
	cases := []struct {
		minor int64
		cur   Currency
		want  string
	}{
		{123450, KES, "1234.50"},
		{5, USD, "0.05"},
		{-250, NGN, "-2.50"},
		{150000, UGX, "150000"},
		{0, GHS, "0.00"},
	}
	for _, tc := range cases {
		if got := MinorToDecimal(tc.minor, tc.cur); got != tc.want {
			t.Errorf("MinorToDecimal(%d, %s) = %q, want %q", tc.minor, tc.cur, got, tc.want)
		}
	}
}

func TestFormat(t *testing.T) {
	if got := Format(1234567, NGN); got != "NGN 12,345.67" {
		t.Errorf("got %q", got)
	}
	if got := Format(1850000, UGX); got != "UGX 1,850,000" {
		t.Errorf("got %q", got)
	}
	if got := Format(-99, USD); got != "USD -0.99" {
		t.Errorf("got %q", got)
	}
}

func TestDecimalToMinor(t *testing.T) {
	cases := []struct {
		in   string
		cur  Currency
		want int64
	}{
		{"1234.5000", KES, 123450},
		{"12.345", USD, 1235},
		{"150000.0000", UGX, 150000},
		{"-0.005", USD, -1},
	}
	for _, tc := range cases {
		got, err := DecimalToMinor(tc.in, tc.cur)
		if err != nil || got != tc.want {
			t.Errorf("DecimalToMinor(%q, %s) = %d, %v; want %d", tc.in, tc.cur, got, err, tc.want)
		}
	}
	if _, err := DecimalToMinor("12,00", USD); err == nil {
		t.Error("expected error for comma decimal")
	}
}

func TestLedgerDecimal(t *testing.T) {
	if got := LedgerDecimal(150000, UGX); got != "150000.0000" {
		t.Errorf("got %q", got)
	}
	if got := LedgerDecimal(1999, USD); got != "19.9900" {
		t.Errorf("got %q", got)
	}
}

func TestRoundRat(t *testing.T) {
	r := big.NewRat(7, 2) // 3.5
	if RoundRat(r, Down) != 3 || RoundRat(r, Up) != 4 || RoundRat(r, HalfUp) != 4 {
		t.Error("rounding 3.5")
	}
	r = big.NewRat(-7, 2)
	if RoundRat(r, Down) != -3 || RoundRat(r, HalfUp) != -4 {
		t.Error("rounding -3.5")
	}
}

func TestParseCurrency(t *testing.T) {
	if c, err := ParseCurrency(" kes "); err != nil || c != KES {
		t.Errorf("got %v %v", c, err)
	}
	if _, err := ParseCurrency("ZAR"); err == nil {
		t.Error("ZAR should not be supported")
	}
	if c, ok := ForCountry("ug"); !ok || c != UGX {
		t.Errorf("ForCountry(ug) = %v", c)
	}
}
