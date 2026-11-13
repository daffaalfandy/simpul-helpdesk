// Package storage keeps uploaded files. Files are addressed by random keys that never derive
// from user input, so a key can't name a path outside the store. The filesystem driver keeps
// everything on the host, next to the database (one VPS per install).
package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"regexp"
	"time"
)

// keyBytes is the key's randomness: 128 bits, hex encoded to 32 characters.
const keyBytes = 16

// keyPattern matches keys made by NewKey.
var keyPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Errors.
var (
	ErrNotFound   = errors.New("storage: file not found")
	ErrInvalidKey = errors.New("storage: invalid key")
)

// Store keeps files by key.
type Store interface {
	// Put stores everything r yields under key and returns the byte count. The file appears
	// only once fully written; a failed Put leaves nothing behind.
	Put(ctx context.Context, key string, r io.Reader) (int64, error)
	// Open returns the file and its size. Callers close it.
	Open(ctx context.Context, key string) (io.ReadSeekCloser, int64, error)
	// Delete removes the file. A missing file is not an error.
	Delete(ctx context.Context, key string) error
	// PurgeTemp removes partial writes older than olderThan, left by crashes, and returns
	// how many it removed.
	PurgeTemp(ctx context.Context, olderThan time.Duration) (int, error)
	// Close releases the store.
	Close() error
}

// NewKey returns a fresh random key.
func NewKey() string {
	b := make([]byte, keyBytes)
	_, _ = rand.Read(b) // never fails (crypto/rand)
	return hex.EncodeToString(b)
}

// ValidKey reports whether key could have come from NewKey.
func ValidKey(key string) bool { return keyPattern.MatchString(key) }
