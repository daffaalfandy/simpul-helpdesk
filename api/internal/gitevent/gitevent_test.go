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
	h := http.Header{}
	if got, _ := JSON(h, []byte(`{"a":1}`)); string(got) != `{"a":1}` {
		t.Errorf("no content type = %s", got)
	}
	h.Set(HeaderContentType, "application/json")
	if got, _ := JSON(h, []byte(`{"a":1}`)); string(got) != `{"a":1}` {
		t.Errorf("JSON = %s", got)
	}
	h.Set(HeaderContentType, "application/x-www-form-urlencoded; charset=utf-8")
	form := url.Values{formPayloadField: {`{"a":1}`}}.Encode()
	if got, err := JSON(h, []byte(form)); err != nil || string(got) != `{"a":1}` {
		t.Errorf("form = %s, %v", got, err)
	}
	if _, err := JSON(h, []byte("other=1")); !errors.Is(err, ErrMalformed) {
		t.Errorf("form without payload = %v", err)
	}
}

func TestSupported(t *testing.T) {
	for _, e := range []string{"push", "pull_request", "deployment_status", "workflow_run", "check_suite", "ping"} {
		if !Supported(ProviderGitHub, e) {
			t.Errorf("GitHub %s unsupported", e)
		}
	}
	for _, e := range []string{"Push Hook", "Merge Request Hook", "Pipeline Hook", "Deployment Hook"} {
		if !Supported(ProviderGitLab, e) {
			t.Errorf("GitLab %s unsupported", e)
		}
	}
	for _, e := range []string{"issues", "star", "Tag Push Hook", ""} {
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

func TestGitHubPullRequest(t *testing.T) {
	d := parse(t, ProviderGitHub, "pull_request", "github_pull_request_merged.json")
	want := Event{
		Kind: KindPRMerged, ExternalID: "142", SHA: "e4b2a9d1c0f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9", Number: 142,
		Title: "Revert SSO token TTL change", Text: "Revert SSO token TTL change\nCloses HD-24871.\nfix/HD-24871-sso",
		Branch: "fix/HD-24871-sso", Target: "main", URL: "https://github.com/vendor-id/eoffice-jambi/pull/142",
		Author: Author{Login: "dewi.w"}, OccurredAt: utc("2026-11-16T05:40:00Z"),
	}
	if len(d.Events) != 1 || d.Events[0] != want {
		t.Fatalf("events = %+v\nwant     %+v", d.Events, want)
	}

	pr := func(action string, merged bool) Event {
		body := `{"action":"` + action + `","pull_request":{"number":7,"title":"t","merged":` + map[bool]string{true: "true", false: "false"}[merged] +
			`,"created_at":"2026-11-16T00:00:00Z","updated_at":"2026-11-16T01:00:00Z","closed_at":"2026-11-16T02:00:00Z",` +
			`"user":{"login":"author"},"head":{"ref":"fix/HD-7","sha":"h"},"base":{"ref":"main"}},"repository":{"full_name":"o/r"}}`
		d, err := Parse(ProviderGitHub, "pull_request", []byte(body))
		if err != nil {
			t.Fatal(err)
		}
		if len(d.Events) == 0 {
			return Event{}
		}
		return d.Events[0]
	}
	for _, a := range []string{"opened", "reopened", "edited", "ready_for_review"} {
		if e := pr(a, false); e.Kind != KindPROpened || e.Author.Login != "author" || e.OccurredAt != utc("2026-11-16T01:00:00Z") {
			t.Errorf("%s = %+v", a, e)
		}
	}
	if e := pr("closed", false); e.Kind != KindPRClosed || e.OccurredAt != utc("2026-11-16T02:00:00Z") {
		t.Errorf("closed unmerged = %+v", e)
	}
	for _, a := range []string{"synchronize", "labeled", "review_requested"} {
		if e := pr(a, false); e.Kind != "" {
			t.Errorf("%s = %+v", a, e)
		}
	}
}

func TestGitHubDeployment(t *testing.T) {
	d := parse(t, ProviderGitHub, "deployment_status", "github_deployment_status.json")
	want := Event{
		Kind: KindDeployment, ExternalID: "3301", SHA: "c0ffee1234567890c0ffee1234567890c0ffee12", Number: 3301,
		Title: "production", Branch: "main", URL: "https://github.com/vendor-id/web-bintan/actions/runs/55",
		Author: Author{Login: "github-actions[bot]"}, OccurredAt: utc("2026-11-16T01:11:00Z"),
	}
	if len(d.Events) != 1 || d.Events[0] != want {
		t.Fatalf("events = %+v", d.Events)
	}
	for name, body := range map[string]string{
		"pending":  `{"deployment_status":{"state":"pending","environment":"production","created_at":"2026-11-16T00:00:00Z"},"deployment":{"id":1,"environment":"production"},"repository":{"full_name":"o/r"}}`,
		"staging":  `{"deployment_status":{"state":"success","environment":"staging","created_at":"2026-11-16T00:00:00Z"},"deployment":{"id":1,"environment":"staging"},"repository":{"full_name":"o/r"}}`,
		"failed":   `{"deployment_status":{"state":"failure","environment":"production","created_at":"2026-11-16T00:00:00Z"},"deployment":{"id":1},"repository":{"full_name":"o/r"}}`,
		"inactive": `{"deployment_status":{"state":"inactive","environment":"production","created_at":"2026-11-16T00:00:00Z"},"deployment":{"id":1},"repository":{"full_name":"o/r"}}`,
	} {
		if d, err := Parse(ProviderGitHub, "deployment_status", []byte(body)); err != nil || len(d.Events) != 0 {
			t.Errorf("%s: %+v, %v", name, d.Events, err)
		}
	}
	flagged := `{"deployment_status":{"state":"success","environment":"live","created_at":"2026-11-16T00:00:00Z"},"deployment":{"id":2,"environment":"live","production_environment":true},"repository":{"full_name":"o/r","html_url":"https://github.com/o/r"}}`
	d, err := Parse(ProviderGitHub, "deployment_status", []byte(flagged))
	if err != nil || len(d.Events) != 1 || d.Events[0].Title != "live" {
		t.Errorf("production flag: %+v, %v", d.Events, err)
	}
}

func TestGitHubPipelines(t *testing.T) {
	run := parse(t, ProviderGitHub, "workflow_run", "github_workflow_run.json")
	wantRun := Event{
		Kind: KindPipeline, ExternalID: "run:880011", SHA: "e4b2a9d1c0f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9", Number: 881, Title: "CI",
		Branch: "main", URL: "https://github.com/vendor-id/eoffice-jambi/actions/runs/880011", Author: Author{Login: "dewi.w"},
		OccurredAt: utc("2026-11-16T05:41:05Z"),
	}
	if len(run.Events) != 1 || run.Events[0] != wantRun {
		t.Fatalf("workflow run = %+v", run.Events)
	}
	suite := parse(t, ProviderGitHub, "check_suite", "github_check_suite.json")
	wantSuite := Event{
		Kind: KindPipeline, ExternalID: "suite:4401", SHA: "e4b2a9d1c0f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9", Title: "Jenkins", Branch: "main",
		URL:        "https://github.com/vendor-id/eoffice-jambi/commit/e4b2a9d1c0f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9",
		OccurredAt: utc("2026-11-16T05:41:00Z"),
	}
	if len(suite.Events) != 1 || suite.Events[0] != wantSuite {
		t.Fatalf("check suite = %+v", suite.Events)
	}
	failed := `{"action":"completed","workflow_run":{"id":1,"conclusion":"failure","created_at":"2026-11-16T00:00:00Z"},"repository":{"full_name":"o/r"}}`
	if d, err := Parse(ProviderGitHub, "workflow_run", []byte(failed)); err != nil || len(d.Events) != 0 {
		t.Errorf("failed run: %+v, %v", d.Events, err)
	}
	requested := `{"action":"requested","workflow_run":{"id":1,"conclusion":"","created_at":"2026-11-16T00:00:00Z"},"repository":{"full_name":"o/r"}}`
	if d, err := Parse(ProviderGitHub, "workflow_run", []byte(requested)); err != nil || len(d.Events) != 0 {
		t.Errorf("requested run: %+v, %v", d.Events, err)
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

func TestGitLabMergeRequest(t *testing.T) {
	d := parse(t, ProviderGitLab, "Merge Request Hook", "gitlab_merge_request.json")
	want := Event{
		Kind: KindPRMerged, ExternalID: "57", SHA: "f00dfacef00dfacef00dfacef00dfacef00dface", Number: 57,
		Title: "SIPD export uses the new date format", Text: "SIPD export uses the new date format\nRefs HD-24826\nfix/HD-24826-dates",
		Branch: "fix/HD-24826-dates", Target: "release/2026.11", URL: "https://gitlab.vendor.co.id/gov/sipd-connector/-/merge_requests/57",
		Author: Author{Name: "Dewi Wulandari", Login: "dewi.w"}, OccurredAt: utc("2026-11-16T01:30:00Z"),
	}
	if len(d.Events) != 1 || d.Events[0] != want {
		t.Fatalf("events = %+v\nwant     %+v", d.Events, want)
	}
	mr := func(action string) Kind {
		body := `{"user":{"username":"u"},"project":{"path_with_namespace":"g/p"},"object_attributes":{"iid":1,"action":"` + action +
			`","created_at":"2026-11-16T00:00:00Z","updated_at":"2026-11-16T00:00:00Z"}}`
		d, err := Parse(ProviderGitLab, "Merge Request Hook", []byte(body))
		if err != nil {
			t.Fatal(err)
		}
		if len(d.Events) == 0 {
			return ""
		}
		return d.Events[0].Kind
	}
	for action, want := range map[string]Kind{"open": KindPROpened, "reopen": KindPROpened, "update": KindPROpened, "close": KindPRClosed, "approved": ""} {
		if got := mr(action); got != want {
			t.Errorf("%s = %q, want %q", action, got, want)
		}
	}
}

func TestGitLabPipelineAndDeployment(t *testing.T) {
	pl := parse(t, ProviderGitLab, "Pipeline Hook", "gitlab_pipeline.json")
	wantPipeline := Event{
		Kind: KindPipeline, ExternalID: "31", SHA: "f00dfacef00dfacef00dfacef00dfacef00dface", Number: 31, Title: "Build and test",
		Branch: "release/2026.11", URL: "https://gitlab.vendor.co.id/gov/sipd-connector/-/pipelines/31",
		Author: Author{Name: "Dewi Wulandari", Login: "dewi.w"}, OccurredAt: utc("2026-11-16T01:31:00Z"),
	}
	if len(pl.Events) != 1 || pl.Events[0] != wantPipeline {
		t.Fatalf("pipeline = %+v", pl.Events)
	}
	dep := parse(t, ProviderGitLab, "Deployment Hook", "gitlab_deployment.json")
	wantDeploy := Event{
		Kind: KindDeployment, ExternalID: "15", SHA: "f00dface", Number: 15, Title: "prod", Branch: "release/2026.11",
		URL:    "https://gitlab.vendor.co.id/gov/sipd-connector/-/jobs/796",
		Author: Author{Name: "Dewi Wulandari", Login: "dewi.w"}, OccurredAt: utc("2026-11-16T02:11:00Z"),
	}
	if len(dep.Events) != 1 || dep.Events[0] != wantDeploy {
		t.Fatalf("deployment = %+v", dep.Events)
	}
	for name, c := range map[string][2]string{
		"failed pipeline": {"Pipeline Hook", `{"object_attributes":{"id":1,"status":"failed","created_at":"2026-11-16 00:00:00 UTC"},"project":{"path_with_namespace":"g/p"}}`},
		"tag pipeline":    {"Pipeline Hook", `{"object_attributes":{"id":1,"status":"success","tag":true,"created_at":"2026-11-16 00:00:00 UTC"},"project":{"path_with_namespace":"g/p"}}`},
		"staging deploy":  {"Deployment Hook", `{"status":"success","environment":"staging","environment_tier":"staging","status_changed_at":"2026-11-16 00:00:00 +0000","project":{"path_with_namespace":"g/p"}}`},
		"running deploy":  {"Deployment Hook", `{"status":"running","environment":"production","status_changed_at":"2026-11-16 00:00:00 +0000","project":{"path_with_namespace":"g/p"}}`},
	} {
		if d, err := Parse(ProviderGitLab, c[0], []byte(c[1])); err != nil || len(d.Events) != 0 {
			t.Errorf("%s: %+v, %v", name, d.Events, err)
		}
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
