package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

// tokenBytes is the entropy of session and reset tokens (256 bits).
const tokenBytes = 32

// Rate-limit key prefixes. Emails are hashed so raw addresses aren't stored.
const (
	keyPrefixLoginEmail = "email:"
	keyPrefixLoginIP    = "ip:"
	keyPrefixReset      = "reset:"
)

// NewToken returns a random URL-safe token and its SHA-256 hash.
// Store only the hash; give the raw token to the user.
func NewToken() (raw string, hash []byte, err error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, HashToken(raw), nil
}

// HashToken returns the SHA-256 of a raw token.
func HashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

// NormalizeEmail trims and lowercases an email address.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// hashedKey builds a rate-limit key from a prefix and a hashed email.
func hashedKey(prefix, email string) string {
	sum := sha256.Sum256([]byte(email))
	return prefix + hex.EncodeToString(sum[:])
}
