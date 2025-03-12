package money

import (
	"strings"
)

// Format renders minor units for people: "KES 1,234.50", "UGX 150,000".
func Format(minor int64, c Currency) string {
	return string(c) + " " + Group(MinorToDecimal(minor, c))
}

// Group inserts thousands separators into a plain decimal string.
func Group(dec string) string {
	whole, frac, hasFrac := strings.Cut(dec, ".")
	var b strings.Builder
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	out := b.String()
	if hasFrac {
		out += "." + frac
	}
	return out
}
