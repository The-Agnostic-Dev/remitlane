// Package partners handles partner API keys.
package partners

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// KeyPrefixLen is how much of a key we keep in clear for display.
const KeyPrefixLen = 16

// HashKey returns the stored form of an API key.
func HashKey(key string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(key)))
	return hex.EncodeToString(sum[:])
}

// Prefix returns the displayable start of a key.
func Prefix(key string) string {
	if len(key) <= KeyPrefixLen {
		return key
	}
	return key[:KeyPrefixLen]
}

// NewKey creates a key like "rl_live_<32 hex>".
func NewKey(env string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "rl_" + env + "_" + hex.EncodeToString(b)
}

// Mask renders a key prefix for the console: "rl_sandbox_kobo…".
func Mask(prefix string) string { return prefix + "…" }
