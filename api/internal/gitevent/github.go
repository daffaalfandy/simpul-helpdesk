package gitevent

import (
	"encoding/json"
	"strconv"
	"strings"
)

// GitHub event names (X-GitHub-Event) the connector reads: the webhook check and pushes.
const (
	gitHubPing = "ping"
	gitHubPush = "push"
)

var gitHubEvents = []string{gitHubPing, gitHubPush}

type ghPayload struct {
	Repository struct {
		ID       int64  `json:"id"`
		FullName string `json:"full_name"`
		HTMLURL  string `json:"html_url"`
	} `json:"repository"`
	Ref     string `json:"ref"`
	Deleted bool   `json:"deleted"`
	Commits []struct {
		ID        string `json:"id"`
		Message   string `json:"message"`
		Timestamp string `json:"timestamp"`
		URL       string `json:"url"`
		Author    struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Username string `json:"username"`
		} `json:"author"`
	} `json:"commits"`
}

func parseGitHub(event string, payload []byte) (Delivery, error) {
	var p ghPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return Delivery{}, err
	}
	d := Delivery{Repository: Repository{
		ExternalID: formatID(p.Repository.ID), FullName: p.Repository.FullName, WebURL: p.Repository.HTMLURL,
	}}
	var err error
	switch event {
	case gitHubPing:
		d.Ping = true
	case gitHubPush:
		d.Events, err = gitHubPushEvents(p)
	default:
		err = errUnexpectedEvent
	}
	return d, err
}

func gitHubPushEvents(p ghPayload) ([]Event, error) {
	branch, ok := strings.CutPrefix(p.Ref, branchRef)
	if !ok || p.Deleted {
		return nil, nil
	}
	out := make([]Event, 0, len(p.Commits))
	for _, c := range p.Commits {
		at, err := parseTime(c.Timestamp)
		if err != nil {
			return nil, err
		}
		out = append(out, Event{
			Kind: KindCommit, ExternalID: c.ID, SHA: c.ID, Title: FirstLine(c.Message), Text: c.Message,
			Branch: branch, URL: c.URL, OccurredAt: at,
			Author: Author{Name: c.Author.Name, Email: c.Author.Email, Login: c.Author.Username},
		})
	}
	return out, nil
}

func formatID(n int64) string {
	if n == 0 {
		return ""
	}
	return strconv.FormatInt(n, 10)
}
