// Package money holds currency rules and amount conversions. Amounts travel
// through the system as int64 minor units; the ledger stores NUMERIC(20,4)
// major units, converted at the edges with the helpers in this package.
package money

import (
	"fmt"
	"strings"
)

// Currency is an ISO 4217 code.
type Currency string

// Supported currencies.
const (
	USD Currency = "USD"
	NGN Currency = "NGN"
	KES Currency = "KES"
	GHS Currency = "GHS"
	UGX Currency = "UGX"
)

type currencyInfo struct {
	exponent int
	name     string
	country  string
}

var currencies = map[Currency]currencyInfo{
	USD: {2, "US dollar", "US"},
	NGN: {2, "Nigerian naira", "NG"},
	KES: {2, "Kenyan shilling", "KE"},
	GHS: {2, "Ghanaian cedi", "GH"},
	UGX: {0, "Ugandan shilling", "UG"},
}

// ParseCurrency validates a currency code.
func ParseCurrency(s string) (Currency, error) {
	c := Currency(strings.ToUpper(strings.TrimSpace(s)))
	if _, ok := currencies[c]; !ok {
		return "", fmt.Errorf("unsupported currency %q", s)
	}
	return c, nil
}

// Exponent is the number of decimal places of the currency's minor unit.
func (c Currency) Exponent() int {
	if info, ok := currencies[c]; ok {
		return info.exponent
	}
	return 2
}


// Country is the ISO country code where the currency is paid out.
func (c Currency) Country() string { return currencies[c].country }

// Valid reports whether the currency is supported.
func (c Currency) Valid() bool {
	_, ok := currencies[c]
	return ok
}

func (c Currency) String() string { return string(c) }

// ForCountry returns the payout currency for an ISO country code.
func ForCountry(country string) (Currency, bool) {
	country = strings.ToUpper(country)
	for c, info := range currencies {
		if info.country == country && c != USD {
			return c, true
		}
	}
	return "", false
}

// pow10 returns 10^n for small n.
func pow10(n int) int64 {
	p := int64(1)
	for range n {
		p *= 10
	}
	return p
}
