package gitevent

import (
	"encoding/json"
	"strings"
)

// GitLab event names (X-Gitlab-Event) the connector reads: branch pushes. Tag pushes have their
// own name and are not read.
const gitLabPush = "Push Hook"

var gitLabEvents = []string{gitLabPush}

// gitLabDeletedSHA is the "after" of a push that deleted the branch.
const gitLabDeletedSHA = "0000000000000000000000000000000000000000"

type glPayload struct {
	Project struct {
		ID                int64  `json:"id"`
		PathWithNamespace string `json:"path_with_namespace"`
		WebURL            string `json:"web_url"`
	} `json:"project"`
	Ref          string `json:"ref"`
	After        string `json:"after"`
	UserName     string `json:"user_name"`
	UserUsername string `json:"user_username"`
	Commits      []struct {
		ID        string `json:"id"`
		Message   string `json:"message"`
		Timestamp string `json:"timestamp"`
		URL       string `json:"url"`
		Author    struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"author"`
	} `json:"commits"`
}

func parseGitLab(event string, payload []byte) (Delivery, error) {
	var p glPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return Delivery{}, err
	}
	d := Delivery{Repository: Repository{
		ExternalID: formatID(p.Project.ID), FullName: p.Project.PathWithNamespace, WebURL: p.Project.WebURL,
	}}
	var err error
	switch event {
	case gitLabPush:
		d.Events, err = gitLabPushEvents(p)
	default:
		err = errUnexpectedEvent
	}
	return d, err
}

// gitLabPushEvents reads the commits a push carries. GitLab lists at most 20 per push; the rest
// would need an access token to fetch.
func gitLabPushEvents(p glPayload) ([]Event, error) {
	branch, ok := strings.CutPrefix(p.Ref, branchRef)
	if !ok || p.After == gitLabDeletedSHA {
		return nil, nil
	}
	out := make([]Event, 0, len(p.Commits))
	for _, c := range p.Commits {
		at, err := parseTime(c.Timestamp)
		if err != nil {
			return nil, err
		}
		a := Author{Name: c.Author.Name, Email: c.Author.Email}
		if strings.EqualFold(c.Author.Name, p.UserName) {
			a.Login = p.UserUsername // the pusher wrote it; GitLab names commit authors by email only
		}
		out = append(out, Event{
			Kind: KindCommit, ExternalID: c.ID, SHA: c.ID, Title: FirstLine(c.Message), Text: c.Message,
			Branch: branch, URL: c.URL, OccurredAt: at, Author: a,
		})
	}
	return out, nil
}
