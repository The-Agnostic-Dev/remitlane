// Package recipients validates and normalises who a payout goes to.
package recipients

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/remitlane/remitlane/internal/money"
	"github.com/remitlane/remitlane/internal/rates"
)

// Recipient is the payee of a payout.
type Recipient struct {
	Name          string `json:"name"`
	Country       string `json:"country"`
	BankCode      string `json:"bank_code,omitempty"`
	AccountNumber string `json:"account_number,omitempty"`
	MSISDN        string `json:"msisdn,omitempty"`
}

// FieldError says which field is wrong and why.
type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Message }

func fieldErr(field, format string, args ...any) error {
	return &FieldError{Field: "recipient." + field, Message: fmt.Sprintf(format, args...)}
}

var digits = regexp.MustCompile(`^\d+$`)

// bankRules are account and bank code formats per country.
var bankRules = map[string]struct {
	account  *regexp.Regexp
	bankCode *regexp.Regexp
	hint     string
}{
	"NG": {regexp.MustCompile(`^\d{10}$`), regexp.MustCompile(`^\d{3}(\d{3})?$`), "a 10 digit NUBAN"},
	"KE": {regexp.MustCompile(`^\d{6,16}$`), regexp.MustCompile(`^\d{2,3}$`), "6 to 16 digits"},
	"GH": {regexp.MustCompile(`^\d{10,16}$`), regexp.MustCompile(`^\d{6}$`), "10 to 16 digits"},
	"UG": {regexp.MustCompile(`^\d{9,14}$`), regexp.MustCompile(`^[A-Z0-9]{4,11}$`), "9 to 14 digits"},
}

// Validate checks a recipient for a payout in currency cur over channel ch
// and returns it normalised (trimmed, msisdn in international form).
func Validate(r Recipient, cur money.Currency, ch rates.Channel) (Recipient, error) {
	r.Name = strings.Join(strings.Fields(r.Name), " ")
	r.Country = strings.ToUpper(strings.TrimSpace(r.Country))
	r.BankCode = strings.ToUpper(strings.TrimSpace(r.BankCode))
	r.AccountNumber = strings.ReplaceAll(strings.TrimSpace(r.AccountNumber), " ", "")

	if n := utf8.RuneCountInString(r.Name); n < 2 || n > 100 {
		return r, fieldErr("name", "must be 2 to 100 characters")
	}
	if r.Country == "" {
		return r, fieldErr("country", "is required")
	}
	if r.Country != cur.Country() {
		return r, fieldErr("country", "%s payouts go to %s recipients, not %s", cur, cur.Country(), r.Country)
	}

	switch ch {
	case rates.Bank:
		rule, ok := bankRules[r.Country]
		if !ok {
			return r, fieldErr("country", "bank payouts to %s are not supported", r.Country)
		}
		if !rule.account.MatchString(r.AccountNumber) {
			return r, fieldErr("account_number", "must be %s", rule.hint)
		}
		if !rule.bankCode.MatchString(r.BankCode) {
			return r, fieldErr("bank_code", "is not a valid %s bank code", r.Country)
		}
		r.MSISDN = ""
	case rates.MobileMoney:
		m, err := NormalizeMSISDN(r.MSISDN, r.Country)
		if err != nil {
			return r, err
		}
		r.MSISDN = m
		r.BankCode, r.AccountNumber = "", ""
	default:
		return r, &FieldError{Field: "channel", Message: "must be bank or mobile_money"}
	}
	return r, nil
}
