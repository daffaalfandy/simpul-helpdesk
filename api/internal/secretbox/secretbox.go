// Package secretbox encrypts secrets kept in the database, such as webhook secrets (HANDOFF §12:
// secrets encrypted at rest). AES-256-GCM with a random nonce; the key comes from ENCRYPTION_KEY.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
)

// KeyBytes is the key length: AES-256.
const KeyBytes = 32

// version1 starts every sealed value, so the format or key can change later.
const version1 byte = 1

var (
	ErrKeyLength = errors.New("the encryption key must be 32 bytes, base64-encoded")
	ErrOpen      = errors.New("the secret can't be decrypted with this key")
)

// Box seals and opens secrets with one key.
type Box struct {
	aead cipher.AEAD
}

// ParseKey reads a base64 key (standard or URL alphabet, padded or not).
func ParseKey(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if k, err := enc.DecodeString(s); err == nil && len(k) == KeyBytes {
			return k, nil
		}
	}
	return nil, ErrKeyLength
}

func New(key []byte) (*Box, error) {
	if len(key) != KeyBytes {
		return nil, ErrKeyLength
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// Seal encrypts plain. context binds the value to where it is stored (a row id), so a sealed
// value copied to another row doesn't open.
func (b *Box) Seal(plain, context []byte) []byte {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	out := append([]byte{version1}, nonce...)
	return b.aead.Seal(out, nonce, plain, context)
}

// Open decrypts a value sealed with the same key and context.
func (b *Box) Open(sealed, context []byte) ([]byte, error) {
	n := b.aead.NonceSize()
	if len(sealed) < 1+n || sealed[0] != version1 {
		return nil, ErrOpen
	}
	plain, err := b.aead.Open(nil, sealed[1:1+n], sealed[1+n:], context)
	if err != nil {
		return nil, ErrOpen
	}
	return plain, nil
}
