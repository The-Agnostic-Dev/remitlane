// Package sig signs and verifies HMAC-SHA256 webhook signatures.
package sig

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

// ErrMismatch is returned when a signature does not match.
var ErrMismatch = errors.New("sig: signature mismatch")

// Sum returns HMAC-SHA256(secret, body).
func Sum(secret string, body []byte) []byte {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return m.Sum(nil)
}

// Hex returns the hex encoded signature of body.
func Hex(secret string, body []byte) string { return hex.EncodeToString(Sum(secret, body)) }

// Base64 returns the standard base64 encoded signature of body.
func Base64(secret string, body []byte) string {
	return base64.StdEncoding.EncodeToString(Sum(secret, body))
}

// VerifyHex checks a hex signature, with or without a "sha256=" prefix.
func VerifyHex(secret string, body []byte, signature string) error {
	got, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(signature), "sha256="))
	if err != nil {
		return ErrMismatch
	}
	if !hmac.Equal(got, Sum(secret, body)) {
		return ErrMismatch
	}
	return nil
}

// VerifyBase64 checks a base64 signature over body.
func VerifyBase64(secret string, body []byte, signature string) error {
	got, err := base64.StdEncoding.DecodeString(strings.TrimSpace(signature))
	if err != nil {
		return ErrMismatch
	}
	if !hmac.Equal(got, Sum(secret, body)) {
		return ErrMismatch
	}
	return nil
}

