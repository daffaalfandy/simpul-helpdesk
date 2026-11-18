package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/netip"
	"strings"
)

// tokenBytes is the entropy of session and reset tokens (256 bits).
const tokenBytes = 32

// Rate-limit key prefixes. Emails are hashed so raw addresses aren't stored.
const (
	keyPrefixLoginEmail = "email:"
	keyPrefixLoginIP    = "ip:"
	keyPrefixReset      = "reset:"
	keyPrefixInviteIP   = "invite-ip:"
	keyPrefixSetupIP    = "setup-ip:"
	keyPrefixTokenIP    = "token-ip:"
	// keyPrefixResetIP counts wrong reset links; keyPrefixResetRequestIP counts reset requests.
	keyPrefixResetIP        = "reset-ip:"
	keyPrefixResetRequestIP = "reset-request-ip:"
)

// ipv6LimitPrefixBits groups IPv6 addresses by /64 for rate limits: a single host is usually
// given a whole /64, so counting each address would let it rotate through them freely.
const ipv6LimitPrefixBits = 64

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

// ipLimitKey is the rate-limit key for the caller's address, or "" when it is unknown.
func ipLimitKey(prefix string, ip netip.Addr) string {
	if !ip.IsValid() {
		return ""
	}
	if ip.Is6() && !ip.Is4In6() {
		if p, err := ip.WithZone("").Prefix(ipv6LimitPrefixBits); err == nil {
			return prefix + p.String()
		}
	}
	return prefix + ip.String()
}

// hashedKey builds a rate-limit key from a prefix and a hashed email.
func hashedKey(prefix, email string) string {
	sum := sha256.Sum256([]byte(email))
	return prefix + hex.EncodeToString(sum[:])
}
