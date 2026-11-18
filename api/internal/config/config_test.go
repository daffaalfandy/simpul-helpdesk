package config

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/mail"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/secretbox"
)

// prodEnv sets a valid production environment; tests change one variable from there.
func prodEnv(t *testing.T) {
	t.Helper()
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", secretbox.KeyBytes)))
	for name, value := range map[string]string{
		VarAppEnv: EnvProd, VarAppURL: "https://help.example.test", VarDatabaseURL: "postgres://localhost/simdesk",
		VarStorageDir: "/var/lib/simdesk", VarEncryptionKey: key, VarTrustedProxies: "", VarSMTPTLS: "",
	} {
		t.Setenv(name, value)
	}
}

func TestLoadProduction(t *testing.T) {
	prodEnv(t)
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !c.CookieSecure || c.DocsEnabled || c.SMTPTLS != mail.TLSAuto {
		t.Fatalf("production defaults = %+v", c)
	}
}

func TestLoadRefusesUnsafeSettings(t *testing.T) {
	devKey := strings.TrimRight(DefaultDevEncryptionKey, "=")
	tests := []struct {
		name, variable, value, want string
	}{
		{"the dev key", VarEncryptionKey, DefaultDevEncryptionKey, "public development key"},
		{"the dev key unpadded", VarEncryptionKey, devKey, "public development key"},
		{"the dev key URL-safe", VarEncryptionKey, strings.NewReplacer("+", "-", "/", "_").Replace(devKey), "public development key"},
		{"trusting every IPv4 address", VarTrustedProxies, "10.0.0.1, 0.0.0.0/0", "trusts every address"},
		{"trusting every IPv6 address", VarTrustedProxies, "::/0", "trusts every address"},
		{"an unknown TLS policy", VarSMTPTLS, "sometimes", VarSMTPTLS},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prodEnv(t)
			t.Setenv(tt.variable, tt.value)
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Load() error = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}
