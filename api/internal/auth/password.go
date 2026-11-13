package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
)

// Password length limits, in characters.
const (
	MinPasswordLen = 10
	MaxPasswordLen = 128
)

// Params are argon2id cost parameters.
type Params struct {
	Memory  uint32 // KiB
	Time    uint32
	Threads uint8
	SaltLen uint32
	KeyLen  uint32
}

const (
	defaultMemoryKiB = 19 * 1024 // 19 MiB
	defaultTime      = 2
	defaultThreads   = 1
	defaultSaltLen   = 16
	defaultKeyLen    = 32

	// DefaultMaxConcurrentHashes bounds memory use: each hash allocates defaultMemoryKiB.
	DefaultMaxConcurrentHashes = 4

	// dummyPassword is hashed once at startup to equalise timing for unknown emails.
	dummyPassword = "timing-equaliser-not-a-real-password"

	phcAlgorithm = "argon2id"
	phcSeparator = "$"
	phcParts     = 6
)

// DefaultParams follow the OWASP minimum for argon2id.
var DefaultParams = Params{
	Memory: defaultMemoryKiB, Time: defaultTime, Threads: defaultThreads,
	SaltLen: defaultSaltLen, KeyLen: defaultKeyLen,
}

// Hasher hashes and verifies passwords. A semaphore caps concurrent hashes,
// because each one allocates Params.Memory.
type Hasher struct {
	params Params
	sem    chan struct{}
	dummy  string
}

// ErrMalformedHash means a stored hash could not be parsed.
var ErrMalformedHash = errors.New("malformed password hash")

// NewHasher returns a hasher allowing maxConcurrent hashes at once.
func NewHasher(p Params, maxConcurrent int) *Hasher {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	h := &Hasher{params: p, sem: make(chan struct{}, maxConcurrent)}
	dummy, err := h.hash(dummyPassword)
	if err != nil {
		panic(err) // only fails if the system RNG fails
	}
	h.dummy = dummy
	return h
}

func ValidatePassword(pw string) error {
	n := utf8.RuneCountInString(pw)
	if n < MinPasswordLen {
		return apperr.WeakPassword(fmt.Sprintf("Use at least %d characters.", MinPasswordLen))
	}
	if n > MaxPasswordLen {
		return apperr.WeakPassword(fmt.Sprintf("Use at most %d characters.", MaxPasswordLen))
	}
	return nil
}

// Hash returns a PHC-formatted argon2id hash.
func (h *Hasher) Hash(ctx context.Context, pw string) (string, error) {
	if err := h.acquire(ctx); err != nil {
		return "", err
	}
	defer h.release()
	return h.hash(pw)
}

func (h *Hasher) hash(pw string) (string, error) {
	salt := make([]byte, h.params.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(pw), salt, h.params.Time, h.params.Memory, h.params.Threads, h.params.KeyLen)
	return encode(h.params, salt, key), nil
}

// Verify checks pw against an encoded hash. needsRehash is true when the hash
// was made with different parameters than the hasher's current ones.
func (h *Hasher) Verify(ctx context.Context, pw, encoded string) (ok, needsRehash bool, err error) {
	p, salt, key, err := decode(encoded)
	if err != nil {
		return false, false, err
	}
	if err := h.acquire(ctx); err != nil {
		return false, false, err
	}
	defer h.release()
	got := argon2.IDKey([]byte(pw), salt, p.Time, p.Memory, p.Threads, uint32(len(key)))
	if subtle.ConstantTimeCompare(got, key) != 1 {
		return false, false, nil
	}
	needsRehash = p.Memory != h.params.Memory || p.Time != h.params.Time ||
		p.Threads != h.params.Threads || uint32(len(key)) != h.params.KeyLen
	return true, needsRehash, nil
}

// VerifyDummy spends the same time as a real check, so unknown emails can't be
// told apart from wrong passwords by timing.
func (h *Hasher) VerifyDummy(ctx context.Context, pw string) {
	_, _, _ = h.Verify(ctx, pw, h.dummy)
}

func (h *Hasher) acquire(ctx context.Context) error {
	select {
	case h.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *Hasher) release() { <-h.sem }

// encode writes the PHC string format: $argon2id$v=19$m=…,t=…,p=…$salt$key
func encode(p Params, salt, key []byte) string {
	return fmt.Sprintf("$%s$v=%d$m=%d,t=%d,p=%d$%s$%s",
		phcAlgorithm, argon2.Version, p.Memory, p.Time, p.Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key))
}

func decode(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, phcSeparator)
	if len(parts) != phcParts || parts[0] != "" || parts[1] != phcAlgorithm {
		return Params{}, nil, nil, ErrMalformedHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return Params{}, nil, nil, ErrMalformedHash
	}
	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Time, &p.Threads); err != nil {
		return Params{}, nil, nil, ErrMalformedHash
	}
	if p.Memory == 0 || p.Time == 0 || p.Threads == 0 {
		return Params{}, nil, nil, ErrMalformedHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return Params{}, nil, nil, ErrMalformedHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return Params{}, nil, nil, ErrMalformedHash
	}
	p.SaltLen = uint32(len(salt))
	p.KeyLen = uint32(len(key))
	return p, salt, key, nil
}
