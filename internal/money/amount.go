package money

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// Amount is a value in minor units of a currency.
type Amount struct {
	Minor    int64
	Currency Currency
}

// New returns an Amount.
func New(minor int64, c Currency) Amount { return Amount{Minor: minor, Currency: c} }

// Add returns a+b. It panics if the currencies differ, which is always a
// programming error.
func (a Amount) Add(b Amount) Amount {
	if a.Currency != b.Currency {
		panic(fmt.Sprintf("money: adding %s to %s", b.Currency, a.Currency))
	}
	return Amount{Minor: a.Minor + b.Minor, Currency: a.Currency}
}

// IsZero reports whether the amount is zero.
func (a Amount) IsZero() bool { return a.Minor == 0 }

// Decimal renders the amount in major units with the currency's exponent,
// for example 150000 UGX as "150000" and 123450 KES as "1234.50".
func (a Amount) Decimal() string { return MinorToDecimal(a.Minor, a.Currency) }

func (a Amount) String() string { return Format(a.Minor, a.Currency) }

// MinorToDecimal renders minor units as a plain decimal string.
func MinorToDecimal(minor int64, c Currency) string {
	exp := c.Exponent()
	neg := minor < 0
	if neg {
		minor = -minor
	}
	s := fmt.Sprintf("%d", minor)
	if exp > 0 {
		for len(s) <= exp {
			s = "0" + s
		}
		s = s[:len(s)-exp] + "." + s[len(s)-exp:]
	}
	if neg {
		s = "-" + s
	}
	return s
}

// LedgerDecimal renders minor units with four decimal places for the
// NUMERIC(20,4) ledger columns.
func LedgerDecimal(minor int64, c Currency) string {
	r := new(big.Rat).SetFrac(big.NewInt(minor), big.NewInt(pow10(c.Exponent())))
	return r.FloatString(4)
}

// ErrBadDecimal is returned for strings that are not plain decimals.
var ErrBadDecimal = errors.New("money: not a decimal amount")

// DecimalToMinor parses a decimal major-unit string ("1234.5", "1234.5000")
// into minor units, rounding half away from zero at the currency's exponent.
func DecimalToMinor(s string, c Currency) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, ErrBadDecimal
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return 0, fmt.Errorf("%w: %q", ErrBadDecimal, s)
	}
	r.Mul(r, new(big.Rat).SetInt64(pow10(c.Exponent())))
	return RoundRat(r, HalfUp), nil
}

// RoundMode says how to round a fractional minor amount.
type RoundMode int

// Rounding modes.
const (
	Down   RoundMode = iota // towards zero
	Up                      // away from zero
	HalfUp                  // to nearest, halves away from zero
)

// RoundRat rounds a rational number of minor units to an integer.
func RoundRat(r *big.Rat, mode RoundMode) int64 {
	num := new(big.Int).Set(r.Num())
	den := r.Denom()
	neg := num.Sign() < 0
	if neg {
		num.Neg(num)
	}
	q, rem := new(big.Int).QuoRem(num, den, new(big.Int))
	if rem.Sign() != 0 {
		switch mode {
		case Up:
			q.Add(q, big.NewInt(1))
		case HalfUp:
			if new(big.Int).Mul(rem, big.NewInt(2)).Cmp(den) >= 0 {
				q.Add(q, big.NewInt(1))
			}
		}
	}
	if neg {
		q.Neg(q)
	}
	return q.Int64()
}
