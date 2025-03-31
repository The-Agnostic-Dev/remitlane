package recipients

import (
	"regexp"
	"strings"
)

// dialCodes maps countries to their international dialling code.
var dialCodes = map[string]string{
	"KE": "254",
	"GH": "233",
	"UG": "256",
}

// mobilePatterns are the mobile money number ranges each rail accepts,
// in international form without the plus.
var mobilePatterns = map[string]*regexp.Regexp{
	// Safaricom and Airtel: 07xx and 01xx.
	"KE": regexp.MustCompile(`^254(7\d|1[01])\d{7}$`),
	// MTN, Telecel and AirtelTigo numbers all start 02 or 05.
	"GH": regexp.MustCompile(`^233[25]\d{8}$`),
	// MTN 077/078/076, Airtel 070/074/075.
	"UG": regexp.MustCompile(`^256(7[0-8])\d{7}$`),
}

// NormalizeMSISDN turns "+254 712 345 678", "0712345678" or
// "254712345678" into "254712345678" and checks it is a mobile money number
// for the country.
func NormalizeMSISDN(raw, country string) (string, error) {
	code, ok := dialCodes[country]
	if !ok {
		return "", fieldErr("country", "mobile money payouts to %s are not supported", country)
	}
	s := strings.ReplaceAll(strings.TrimSpace(raw), " ", "")
	s = strings.TrimPrefix(s, "+")
	switch {
	case s == "":
		return "", fieldErr("msisdn", "is required for mobile money")
	case strings.HasPrefix(s, "00"+code):
		s = s[2:]
	case strings.HasPrefix(s, "0"):
		s = code + s[1:]
	}
	if !digits.MatchString(s) {
		return "", fieldErr("msisdn", "must contain only digits")
	}
	if !mobilePatterns[country].MatchString(s) {
		return "", fieldErr("msisdn", "%s is not a %s mobile money number", raw, country)
	}
	return s, nil
}
