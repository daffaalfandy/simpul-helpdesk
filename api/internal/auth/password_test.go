package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
)

// cheapParams keep tests fast; production uses DefaultParams.
var cheapParams = Params{Memory: 64, Time: 1, Threads: 1, SaltLen: 16, KeyLen: 32}

func TestHashAndVerify(t *testing.T) {
	ctx := context.Background()
	h := NewHasher(cheapParams, 2)

	encoded, err := h.Hash(ctx, "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=64,t=1,p=1$") {
		t.Fatalf("unexpected PHC string %q", encoded)
	}

	ok, rehash, err := h.Verify(ctx, "correct horse battery", encoded)
	if err != nil || !ok || rehash {
		t.Fatalf("Verify(correct) = %v, %v, %v; want true, false, nil", ok, rehash, err)
	}
	ok, _, err = h.Verify(ctx, "wrong horse battery", encoded)
	if err != nil || ok {
		t.Fatalf("Verify(wrong) = %v, %v; want false, nil", ok, err)
	}

	again, _ := h.Hash(ctx, "correct horse battery")
	if again == encoded {
		t.Fatal("two hashes of the same password must differ (random salt)")
	}
}

func TestVerifyTamperedOrMalformed(t *testing.T) {
	ctx := context.Background()
	h := NewHasher(cheapParams, 1)
	encoded, _ := h.Hash(ctx, "correct horse battery")

	parts := strings.Split(encoded, "$")
	parts[5] = strings.Repeat("A", len(parts[5])) // different key, same length
	ok, _, err := h.Verify(ctx, "correct horse battery", strings.Join(parts, "$"))
	if err != nil || ok {
		t.Fatalf("tampered hash verified: ok=%v err=%v", ok, err)
	}

	for _, bad := range []string{"", "plain", "$argon2i$v=19$m=64,t=1,p=1$c2FsdA$a2V5", "$argon2id$v=19$m=0,t=1,p=1$c2FsdA$a2V5"} {
		if _, _, err := h.Verify(ctx, "x", bad); !errors.Is(err, ErrMalformedHash) {
			t.Errorf("Verify(%q) err = %v, want ErrMalformedHash", bad, err)
		}
	}
}

func TestNeedsRehashWhenParamsChange(t *testing.T) {
	ctx := context.Background()
	old := NewHasher(cheapParams, 1)
	encoded, _ := old.Hash(ctx, "correct horse battery")

	stronger := cheapParams
	stronger.Time = 2
	ok, rehash, err := NewHasher(stronger, 1).Verify(ctx, "correct horse battery", encoded)
	if err != nil || !ok || !rehash {
		t.Fatalf("Verify with new params = %v, %v, %v; want true, true, nil", ok, rehash, err)
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name string
		pw   string
		ok   bool
	}{
		{"too short", "123456789", false},
		{"minimum", "1234567890", true},
		{"multibyte counts characters", strings.Repeat("é", MinPasswordLen), true},
		{"maximum", strings.Repeat("a", MaxPasswordLen), true},
		{"too long", strings.Repeat("a", MaxPasswordLen+1), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.pw)
			if tt.ok != (err == nil) {
				t.Fatalf("ValidatePassword() err = %v, want ok=%v", err, tt.ok)
			}
			var ae *apperr.Error
			if err != nil && (!errors.As(err, &ae) || ae.Code != apperr.CodeWeakPassword) {
				t.Fatalf("want weak_password error, got %v", err)
			}
		})
	}
}

func TestTokens(t *testing.T) {
	raw1, hash1, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	raw2, _, _ := NewToken()
	if raw1 == raw2 {
		t.Fatal("tokens must be unique")
	}
	if string(HashToken(raw1)) != string(hash1) {
		t.Fatal("HashToken must match the hash returned by NewToken")
	}
	if strings.ContainsAny(raw1, "+/=") {
		t.Fatalf("token %q is not URL-safe", raw1)
	}
}

func TestNormalizeEmail(t *testing.T) {
	if got := NormalizeEmail("  Rina@Demo-Vendor.TEST "); got != "rina@demo-vendor.test" {
		t.Fatalf("NormalizeEmail = %q", got)
	}
}
