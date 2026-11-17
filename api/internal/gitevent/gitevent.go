// Package gitevent reads GitHub and GitLab webhook deliveries (HANDOFF §8): it checks the
// secret, names the delivery and the repository it came from, and turns pushes into
// provider-neutral commit events. It is pure; the connector stores the events and links them.
// Only pushes are read (owner, 17 Nov): pull/merge requests, CI and deployments are not.
package gitevent

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// Provider is where a repository is hosted. Values match the repositories.provider CHECK.
type Provider string

const (
	ProviderGitHub Provider = "github"
	ProviderGitLab Provider = "gitlab"
)

var Providers = []Provider{ProviderGitHub, ProviderGitLab}

func (p Provider) Valid() bool { return slices.Contains(Providers, p) }

// Kind is what happened in the repository. Values match the code_events.kind CHECK.
type Kind string

// KindCommit is a commit pushed to a branch, the only kind read.
const KindCommit Kind = "commit"

// Author is who wrote the commit, as the provider names them.
type Author struct {
	Name  string
	Email string
	Login string
}

// Event is one commit a push carried, in a form both providers share.
type Event struct {
	Kind       Kind
	ExternalID string // unique per repository and kind: the commit sha
	SHA        string
	Title      string // first line of the commit message
	Text       string // the whole message, searched for ticket keys; never stored
	Branch     string // the branch it was pushed to
	URL        string
	Author     Author
	OccurredAt time.Time
}

// Repository is the repository a delivery says it came from.
type Repository struct {
	ExternalID string // the provider's numeric id
	FullName   string // owner/name, or the GitLab path with its groups
	WebURL     string
}

// Delivery is a parsed webhook delivery.
type Delivery struct {
	Repository Repository
	Ping       bool    // GitHub's check that the webhook works; nothing to act on
	Events     []Event // empty when the delivery holds nothing the connector acts on
}

// Webhook request headers.
const (
	HeaderGitHubEvent     = "X-GitHub-Event"
	HeaderGitHubDelivery  = "X-GitHub-Delivery"
	HeaderGitHubSignature = "X-Hub-Signature-256"
	HeaderGitLabEvent     = "X-Gitlab-Event"
	HeaderGitLabToken     = "X-Gitlab-Token"
	HeaderGitLabEventUUID = "X-Gitlab-Event-UUID"
	HeaderIdempotencyKey  = "Idempotency-Key" // newer GitLab versions; the same on retries
)

// gitHubSignaturePrefix starts X-Hub-Signature-256: "sha256=<hex HMAC of the body>".
const gitHubSignaturePrefix = "sha256="

// GitHub sends a form with the JSON in this field unless the webhook is set to application/json.
const formPayloadField = "payload"

// MaxTitleRunes caps the stored first line of a message or title.
const MaxTitleRunes = 200

var (
	ErrNoSecret        = errors.New("the delivery carries no signature or token")
	ErrBadSecret       = errors.New("the signature or token does not match")
	ErrUnsupported     = errors.New("unsupported provider")
	ErrMalformed       = errors.New("malformed payload")
	errNoRepository    = errors.New("the payload names no repository")
	errUnreadableTime  = errors.New("unreadable time")
	errUnexpectedEvent = errors.New("unexpected event")
)

// Verify checks that the delivery was sent with the repository's secret: an HMAC of the body
// on GitHub, the plain token on GitLab. Both comparisons take constant time.
func Verify(p Provider, h http.Header, body, secret []byte) error {
	switch p {
	case ProviderGitHub:
		sig, ok := strings.CutPrefix(h.Get(HeaderGitHubSignature), gitHubSignaturePrefix)
		if !ok || sig == "" {
			return ErrNoSecret
		}
		got, err := hex.DecodeString(sig)
		if err != nil {
			return ErrBadSecret
		}
		mac := hmac.New(sha256.New, secret)
		mac.Write(body)
		if !hmac.Equal(got, mac.Sum(nil)) {
			return ErrBadSecret
		}
		return nil
	case ProviderGitLab:
		token := h.Get(HeaderGitLabToken)
		if token == "" {
			return ErrNoSecret
		}
		if subtle.ConstantTimeCompare([]byte(token), secret) != 1 {
			return ErrBadSecret
		}
		return nil
	}
	return ErrUnsupported
}

// DeliveryID names a delivery so a redelivery is recognised. GitLab versions without an event
// UUID get a hash of the body, which a retry repeats.
func DeliveryID(p Provider, h http.Header, body []byte) string {
	var id string
	switch p {
	case ProviderGitHub:
		id = h.Get(HeaderGitHubDelivery)
	case ProviderGitLab:
		id = h.Get(HeaderGitLabEventUUID)
		if id == "" {
			id = h.Get(HeaderIdempotencyKey)
		}
	}
	if id == "" {
		sum := sha256.Sum256(body)
		id = hex.EncodeToString(sum[:])
	}
	return id
}

// EventName is the provider's name for the delivery's event, from its header.
func EventName(p Provider, h http.Header) string {
	switch p {
	case ProviderGitHub:
		return h.Get(HeaderGitHubEvent)
	case ProviderGitLab:
		return h.Get(HeaderGitLabEvent)
	}
	return ""
}

// Supported reports whether the connector reads this event at all. Others are acknowledged and
// dropped without being stored.
func Supported(p Provider, eventName string) bool {
	switch p {
	case ProviderGitHub:
		return slices.Contains(gitHubEvents, eventName)
	case ProviderGitLab:
		return slices.Contains(gitLabEvents, eventName)
	}
	return false
}

// JSON returns the delivery's JSON: the body itself, or the payload field of the form GitHub
// sends when the webhook's content type is left at application/x-www-form-urlencoded.
func JSON(body []byte) ([]byte, error) {
	if !bytes.HasPrefix(bytes.TrimSpace(body), []byte(formPayloadField+"=")) {
		return body, nil
	}
	form, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, ErrMalformed
	}
	payload := form.Get(formPayloadField)
	if payload == "" {
		return nil, ErrMalformed
	}
	return []byte(payload), nil
}

// Parse reads a delivery's JSON (see JSON). A push the connector doesn't act on, such as a tag
// push or a deleted branch, gives a delivery without events.
func Parse(p Provider, eventName string, payload []byte) (Delivery, error) {
	var (
		d   Delivery
		err error
	)
	switch p {
	case ProviderGitHub:
		d, err = parseGitHub(eventName, payload)
	case ProviderGitLab:
		d, err = parseGitLab(eventName, payload)
	default:
		return Delivery{}, ErrUnsupported
	}
	if err != nil {
		return Delivery{}, errors.Join(ErrMalformed, err)
	}
	if d.Repository.FullName == "" {
		return Delivery{}, errors.Join(ErrMalformed, errNoRepository)
	}
	return d, nil
}

// FirstLine is the first line of a message, trimmed and capped at MaxTitleRunes.
func FirstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	line = strings.TrimSpace(line)
	if utf8.RuneCountInString(line) <= MaxTitleRunes {
		return line
	}
	return string([]rune(line)[:MaxTitleRunes])
}

// branchRef is the prefix of a branch in a push's ref; tags use refs/tags/ and are ignored.
const branchRef = "refs/heads/"

// timeLayouts are the time formats the providers use: GitHub and newer GitLab send RFC 3339,
// older GitLab hooks "2026-11-16 08:30:00 UTC" or "… +0000".
var timeLayouts = []string{time.RFC3339, "2006-01-02 15:04:05 MST", "2006-01-02 15:04:05 -0700"}

func parseTime(s string) (time.Time, error) {
	for _, l := range timeLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, errors.Join(errUnreadableTime, errors.New(s))
}
