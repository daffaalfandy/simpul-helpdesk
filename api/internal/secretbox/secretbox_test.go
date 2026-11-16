package secretbox

import (
	"bytes"
	"encoding/base64"
	"errors"
	"testing"
)

var key = bytes.Repeat([]byte{7}, KeyBytes)

func TestRoundTrip(t *testing.T) {
	b, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	sealed := b.Seal([]byte("webhook secret"), []byte("repo-1"))
	if bytes.Contains(sealed, []byte("webhook secret")) {
		t.Fatal("sealed value holds the plain text")
	}
	if again := b.Seal([]byte("webhook secret"), []byte("repo-1")); bytes.Equal(again, sealed) {
		t.Fatal("two seals are equal: nonce reused")
	}
	plain, err := b.Open(sealed, []byte("repo-1"))
	if err != nil || string(plain) != "webhook secret" {
		t.Fatalf("Open = %q, %v", plain, err)
	}
}

func TestOpenRefuses(t *testing.T) {
	b, _ := New(key)
	sealed := b.Seal([]byte("s"), []byte("repo-1"))
	other, _ := New(bytes.Repeat([]byte{8}, KeyBytes))
	tampered := bytes.Clone(sealed)
	tampered[len(tampered)-1] ^= 1
	tests := map[string]struct {
		box     *Box
		sealed  []byte
		context string
	}{
		"other row":     {b, sealed, "repo-2"},
		"other key":     {other, sealed, "repo-1"},
		"tampered":      {b, tampered, "repo-1"},
		"short":         {b, sealed[:5], "repo-1"},
		"empty":         {b, nil, "repo-1"},
		"other version": {b, append([]byte{9}, sealed[1:]...), "repo-1"},
	}
	for name, tt := range tests {
		if _, err := tt.box.Open(tt.sealed, []byte(tt.context)); !errors.Is(err, ErrOpen) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestParseKey(t *testing.T) {
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if k, err := ParseKey(" " + enc.EncodeToString(key) + "\n"); err != nil || !bytes.Equal(k, key) {
			t.Errorf("ParseKey = %v, %v", k, err)
		}
	}
	for _, s := range []string{"", "not base64!", base64.StdEncoding.EncodeToString(key[:16])} {
		if _, err := ParseKey(s); !errors.Is(err, ErrKeyLength) {
			t.Errorf("ParseKey(%q) = %v", s, err)
		}
	}
	if _, err := New(key[:16]); !errors.Is(err, ErrKeyLength) {
		t.Errorf("New with a short key = %v", err)
	}
}
