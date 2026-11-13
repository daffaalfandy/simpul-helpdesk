// Package config loads runtime settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	EnvDev  = "dev"
	EnvProd = "prod"
)

// Environment variable names. See .env.example for documentation.
const (
	VarAppEnv         = "APP_ENV"
	VarAppURL         = "APP_URL"
	VarHTTPAddr       = "HTTP_ADDR"
	VarDatabaseURL    = "DATABASE_URL"
	VarCookieSecure   = "COOKIE_SECURE"
	VarTrustedProxies = "TRUSTED_PROXIES"
	VarSMTPAddr       = "SMTP_ADDR"
	VarSMTPUser       = "SMTP_USER"
	VarSMTPPass       = "SMTP_PASS"
	VarMailFrom       = "MAIL_FROM"
	VarMailFromName   = "MAIL_FROM_NAME"
	VarDocsEnabled    = "DOCS_ENABLED"
	VarStorageDir     = "STORAGE_DIR"
)

// Defaults used when a variable is unset.
const (
	DefaultEnv          = EnvProd
	DefaultHTTPAddr     = ":8080"
	DefaultDevAppURL    = "http://localhost:5173"
	DefaultSMTPAddr     = "localhost:25"
	DefaultMailFrom     = "simdesk@localhost"
	DefaultMailFromName = "SimDesk"
	// DefaultDevStorageDir is relative to the working directory; production needs an
	// absolute STORAGE_DIR on persistent, backed-up disk.
	DefaultDevStorageDir = "data/storage"
)

const listSeparator = ","

type Config struct {
	Env            string
	AppURL         *url.URL
	HTTPAddr       string
	DatabaseURL    string
	CookieSecure   bool
	TrustedProxies []netip.Prefix
	SMTPAddr       string
	SMTPUser       string
	SMTPPass       string
	MailFrom       string
	MailFromName   string
	DocsEnabled    bool
	StorageDir     string
}

func (c Config) IsDev() bool { return c.Env == EnvDev }

func Load() (Config, error) {
	var errs []error
	c := Config{
		Env:          getenv(VarAppEnv, DefaultEnv),
		HTTPAddr:     getenv(VarHTTPAddr, DefaultHTTPAddr),
		DatabaseURL:  os.Getenv(VarDatabaseURL),
		SMTPAddr:     getenv(VarSMTPAddr, DefaultSMTPAddr),
		SMTPUser:     os.Getenv(VarSMTPUser),
		SMTPPass:     os.Getenv(VarSMTPPass),
		MailFrom:     getenv(VarMailFrom, DefaultMailFrom),
		MailFromName: getenv(VarMailFromName, DefaultMailFromName),
	}
	if c.Env != EnvDev && c.Env != EnvProd {
		errs = append(errs, fmt.Errorf("%s must be %s or %s, got %q", VarAppEnv, EnvDev, EnvProd, c.Env))
	}
	if c.DatabaseURL == "" {
		errs = append(errs, fmt.Errorf("%s is required", VarDatabaseURL))
	}

	rawURL := os.Getenv(VarAppURL)
	if rawURL == "" && c.IsDev() {
		rawURL = DefaultDevAppURL
	}
	if u, err := url.Parse(strings.TrimRight(rawURL, "/")); err != nil || u.Scheme == "" || u.Host == "" {
		errs = append(errs, fmt.Errorf("%s must be an absolute URL, got %q", VarAppURL, rawURL))
	} else {
		c.AppURL = u
	}

	var err error
	if c.CookieSecure, err = getbool(VarCookieSecure, !c.IsDev()); err != nil {
		errs = append(errs, err)
	}
	if c.DocsEnabled, err = getbool(VarDocsEnabled, c.IsDev()); err != nil {
		errs = append(errs, err)
	}
	if c.TrustedProxies, err = parsePrefixes(os.Getenv(VarTrustedProxies)); err != nil {
		errs = append(errs, err)
	}
	c.StorageDir = strings.TrimSpace(os.Getenv(VarStorageDir))
	switch {
	case c.StorageDir == "" && c.IsDev():
		c.StorageDir = DefaultDevStorageDir
	case c.StorageDir == "":
		errs = append(errs, fmt.Errorf("%s is required", VarStorageDir))
	case !c.IsDev() && !filepath.IsAbs(c.StorageDir):
		errs = append(errs, fmt.Errorf("%s must be an absolute path, got %q", VarStorageDir, c.StorageDir))
	}
	return c, errors.Join(errs...)
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getbool(key string, fallback bool) (bool, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false, got %q", key, v)
	}
	return b, nil
}

// parsePrefixes parses a comma-separated list of CIDRs or single addresses.
func parsePrefixes(s string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for part := range strings.SplitSeq(s, listSeparator) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if p, err := netip.ParsePrefix(part); err == nil {
			out = append(out, p.Masked())
			continue
		}
		addr, err := netip.ParseAddr(part)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid address or CIDR %q", VarTrustedProxies, part)
		}
		out = append(out, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return out, nil
}
