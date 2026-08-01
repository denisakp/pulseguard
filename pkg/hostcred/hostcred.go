package hostcred

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const (
	prefix           = "ag_live_"
	entropyByteCount = 16
	publicPrefixLen  = 12
)

// Generate creates a new raw host-agent credential and returns the raw token,
// its SHA-256 hash, and the non-secret public prefix. Mirrors pkg/apikey.
func Generate() (string, string, string, error) {
	buf := make([]byte, entropyByteCount)
	if _, err := rand.Read(buf); err != nil {
		return "", "", "", fmt.Errorf("generate host credential: %w", err)
	}

	raw := prefix + hex.EncodeToString(buf)
	return raw, Hash(raw), ExtractPrefix(raw), nil
}

// Hash returns the SHA-256 hex digest of a raw credential.
func Hash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// ExtractPrefix returns the non-sensitive credential prefix used in logs and UI.
func ExtractPrefix(raw string) string {
	if len(raw) <= publicPrefixLen {
		return raw
	}
	return raw[:publicPrefixLen]
}

// IsFormat reports whether the token looks like a host-agent credential.
func IsFormat(token string) bool {
	return len(token) > len(prefix) && token[:len(prefix)] == prefix
}
