package gitevent

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func parse(t *testing.T, p Provider, event, file string) Delivery {
	t.Helper()
	d, err := Parse(p, event, fixture(t, file))
	if err != nil {
		t.Fatalf("Parse %s: %v", file, err)
	}
	return d
}

func utc(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t.UTC()
}

func TestVerifyGitHub(t *testing.T) {
	body, secret := []byte(`{"zen":"Keep it logically awesome."}`), []byte("s3cret")
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	good := gitHubSignaturePrefix + hex.EncodeToString(mac.Sum(nil))
	tests := []struct {
		name string
		sig  string
		want error
	}{
		{"good", good, nil},
		{"missing", "", ErrNoSecret},
		{"old sha1 header only", "sha1=abc", ErrNoSecret},
		{"not hex", gitHubSignaturePrefix + "zz", ErrBadSecret},
		{"other secret", gitHubSignaturePrefix + strings.Repeat("0", sha256.Size*2), ErrBadSecret},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := http.Header{}
			if tt.sig != "" {
				h.Set(HeaderGitHubSignature, tt.sig)
			}
			if err := Verify(ProviderGitHub, h, body, secret); !errors.Is(err, tt.want) {
				t.Fatalf("Verify = %v, want %v", err, tt.want)
			}
		})
	}
	h := http.Header{}
	h.Set(HeaderGitHubSignature, good)
	if err := Verify(ProviderGitHub, h, append(body, ' '), secret); !errors.Is(err, ErrBadSecret) {
		t.Fatalf("changed body = %v", err)
	}
}

func TestVerifyGitLab(t *testing.T) {
	secret := []byte("s3cret")
	for token, want := range map[string]error{"s3cret": nil, "": ErrNoSecret, "s3cre": ErrBadSecret, "S3CRET": ErrBadSecret} {
		h := http.Header{}
		if token != "" {
			h.Set(HeaderGitLabToken, token)
		}
		if err := Verify(ProviderGitLab, h, nil, secret); !errors.Is(err, want) {
			t.Errorf("token %q: Verify = %v, want %v", token, err, want)
		}
	}
	if err := Verify("bitbucket", http.Header{}, nil, secret); !errors.Is(err, ErrUnsupported) {
		t.Errorf("unknown provider = %v", err)
	}
}

func TestDeliveryID(t *testing.T) {
	body := []byte(`{}`)
	gh := http.Header{}
	gh.Set(HeaderGitHubDelivery, "72d3162e-cc78-11e3-81ab-4c9367dc0958")
	if got := DeliveryID(ProviderGitHub, gh, body); got != "72d3162e-cc78-11e3-81ab-4c9367dc0958" {
		t.Errorf("GitHub = %q", got)
	}
	gl := http.Header{}
	gl.Set(HeaderIdempotencyKey, "idem-1")
	if got := DeliveryID(ProviderGitLab, gl, body); got != "idem-1" {
		t.Errorf("GitLab idempotency key = %q", got)
	}
	gl.Set(HeaderGitLabEventUUID, "uuid-1")
	if got := DeliveryID(ProviderGitLab, gl, body); got != "uuid-1" {
		t.Errorf("GitLab event UUID = %q", got)
	}
	a, b := DeliveryID(ProviderGitLab, http.Header{}, body), DeliveryID(ProviderGitLab, http.Header{}, []byte(`{ }`))
	if a == "" || a == b || a != DeliveryID(ProviderGitLab, http.Header{}, body) {
		t.Errorf("body hashes = %q, %q", a, b)
	}
}

func TestJSONFromForm(t *testing.T) {
	if got, _ := JSON([]byte(`{"a":1}`)); string(got) != `{"a":1}` {
		t.Errorf("JSON = %s", got)
	}
	form := url.Values{formPayloadField: {`{"a":1}`}}.Encode()
	if got, err := JSON([]byte(form)); err != nil || string(got) != `{"a":1}` {
		t.Errorf("form = %s, %v", got, err)
	}
	if _, err := JSON([]byte("payload=")); !errors.Is(err, ErrMalformed) {
		t.Errorf("empty payload = %v", err)
	}
	if _, err := JSON([]byte("payload=%zz")); !errors.Is(err, ErrMalformed) {
		t.Errorf("bad form = %v", err)
	}
}

func TestSupported(t *testing.T) {
	for _, e := range []string{"push", "ping"} {
		if !Supported(ProviderGitHub, e) {
			t.Errorf("GitHub %s unsupported", e)
		}
	}
	if !Supported(ProviderGitLab, "Push Hook") {
		t.Error("GitLab Push Hook unsupported")
	}
	// Only pushes are read: requests, CI and deployments are acknowledged and dropped.
	for _, e := range []string{"pull_request", "deployment_status", "workflow_run", "check_suite", "Merge Request Hook",
		"Pipeline Hook", "Deployment Hook", "issues", "star", "Tag Push Hook", ""} {
		if Supported(ProviderGitHub, e) || Supported(ProviderGitLab, e) {
			t.Errorf("%q supported", e)
		}
	}
}

func TestGitHubPush(t *testing.T) {
	d := parse(t, ProviderGitHub, "push", "github_push.json")
	wantRepo := Repository{ExternalID: "7001", FullName: "vendor-id/eoffice-jambi", WebURL: "https://github.com/vendor-id/eoffice-jambi"}
	if d.Repository != wantRepo || d.Ping {
		t.Fatalf("delivery = %+v", d)
	}
	if len(d.Events) != 2 {
		t.Fatalf("events = %d", len(d.Events))
	}
	e := d.Events[1]
	want := Event{
		Kind: KindCommit, ExternalID: "a1f9c2e5d7b8c9a0b1c2d3e4f5a6b7c8d9e0f1a2", SHA: "a1f9c2e5d7b8c9a0b1c2d3e4f5a6b7c8d9e0f1a2",
		Title: "Fix HD-24871: revert SSO token TTL to 8h", Text: "Fix HD-24871: revert SSO token TTL to 8h\n\nThe 30-minute TTL logged everyone out at lunch.",
		Branch: "fix/HD-24871-sso", URL: "https://github.com/vendor-id/eoffice-jambi/commit/a1f9c2e5d7b8c9a0b1c2d3e4f5a6b7c8d9e0f1a2",
		Author:     Author{Name: "Adi Saputra", Email: "adi@vendor.example", Login: "adi.saputra"},
		OccurredAt: utc("2026-11-16T01:52:00Z"),
	}
	if e != want {
		t.Fatalf("commit = %+v\nwant     %+v", e, want)
	}
}

func TestGitHubPushIgnored(t *testing.T) {
	for name, body := range map[string]string{
		"tag":            `{"ref":"refs/tags/v1","commits":[{"id":"a","message":"x","timestamp":"2026-11-16T00:00:00Z"}],"repository":{"full_name":"o/r"}}`,
		"deleted branch": `{"ref":"refs/heads/old","deleted":true,"commits":[],"repository":{"full_name":"o/r"}}`,
	} {
		d, err := Parse(ProviderGitHub, "push", []byte(body))
		if err != nil || len(d.Events) != 0 {
			t.Errorf("%s: %+v, %v", name, d, err)
		}
	}
}

func TestGitHubPing(t *testing.T) {
	d, err := Parse(ProviderGitHub, "ping", []byte(`{"zen":"z","hook_id":1,"repository":{"id":7001,"full_name":"vendor-id/eoffice-jambi"}}`))
	if err != nil || !d.Ping || d.Repository.ExternalID != "7001" {
		t.Fatalf("ping = %+v, %v", d, err)
	}
}

func TestGitLabPush(t *testing.T) {
	d := parse(t, ProviderGitLab, "Push Hook", "gitlab_push.json")
	wantRepo := Repository{ExternalID: "15", FullName: "gov/sipd-connector", WebURL: "https://gitlab.vendor.co.id/gov/sipd-connector"}
	if d.Repository != wantRepo {
		t.Fatalf("repository = %+v", d.Repository)
	}
	want := Event{
		Kind: KindCommit, ExternalID: "da1560886d4f094c3e6c9ef40349f7d38b5d27d7", SHA: "da1560886d4f094c3e6c9ef40349f7d38b5d27d7",
		Title: "Closes HD-24826 date format on SIPD export", Text: "Closes HD-24826 date format on SIPD export\n",
		Branch: "release/2026.11", URL: "https://gitlab.vendor.co.id/gov/sipd-connector/-/commit/da1560886d4f094c3e6c9ef40349f7d38b5d27d7",
		Author:     Author{Name: "Rina Pratiwi", Email: "rina@vendor.example", Login: "rina.p"},
		OccurredAt: utc("2026-11-16T01:20:00Z"),
	}
	if len(d.Events) != 1 || d.Events[0] != want {
		t.Fatalf("events = %+v\nwant     %+v", d.Events, want)
	}
	deleted := `{"ref":"refs/heads/old","after":"0000000000000000000000000000000000000000","commits":[],"project":{"path_with_namespace":"g/p"}}`
	if d, err := Parse(ProviderGitLab, "Push Hook", []byte(deleted)); err != nil || len(d.Events) != 0 {
		t.Errorf("deleted branch: %+v, %v", d.Events, err)
	}
}

func TestMalformed(t *testing.T) {
	for name, c := range map[string][3]string{
		"not JSON":      {"github", "push", `{`},
		"no repository": {"github", "push", `{"ref":"refs/heads/main","commits":[]}`},
		"bad time":      {"gitlab", "Push Hook", `{"ref":"refs/heads/main","commits":[{"id":"a","timestamp":"yesterday"}],"project":{"path_with_namespace":"g/p"}}`},
	} {
		if _, err := Parse(Provider(c[0]), c[1], []byte(c[2])); !errors.Is(err, ErrMalformed) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := Parse("bitbucket", "push", []byte(`{}`)); !errors.Is(err, ErrUnsupported) {
		t.Errorf("unknown provider: %v", err)
	}
}

func TestFirstLine(t *testing.T) {
	if got := FirstLine("  Fix HD-1: a\n\nbody"); got != "Fix HD-1: a" {
		t.Errorf("FirstLine = %q", got)
	}
	long := strings.Repeat("é", MaxTitleRunes+5)
	if got := FirstLine(long); len([]rune(got)) != MaxTitleRunes {
		t.Errorf("long = %d runes", len([]rune(got)))
	}
}
