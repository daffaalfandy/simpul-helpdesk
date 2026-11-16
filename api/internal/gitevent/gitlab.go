package gitevent

import (
	"encoding/json"
	"strings"
)

// GitLab event names (X-Gitlab-Event) the connector reads. Tag pushes have their own name and
// are not read.
const (
	gitLabPush         = "Push Hook"
	gitLabMergeRequest = "Merge Request Hook"
	gitLabPipeline     = "Pipeline Hook"
	gitLabDeployment   = "Deployment Hook"
)

var gitLabEvents = []string{gitLabPush, gitLabMergeRequest, gitLabPipeline, gitLabDeployment}

// GitLab merge request actions and states the connector reads.
const (
	gitLabActionOpen    = "open"
	gitLabActionReopen  = "reopen"
	gitLabActionUpdate  = "update"
	gitLabActionMerge   = "merge"
	gitLabActionClose   = "close"
	gitLabStatusSuccess = "success"
	// gitLabDeletedSHA is the "after" of a push that deleted the branch.
	gitLabDeletedSHA = "0000000000000000000000000000000000000000"
)

type glUser struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// author drops the email newer GitLab versions send as "[REDACTED]".
func (u glUser) author() Author {
	a := Author{Name: u.Name, Login: u.Username}
	if strings.Contains(u.Email, emailAt) {
		a.Email = u.Email
	}
	return a
}

// emailAt is in every real email address.
const emailAt = "@"

type glPayload struct {
	Project struct {
		ID                int64  `json:"id"`
		PathWithNamespace string `json:"path_with_namespace"`
		WebURL            string `json:"web_url"`
	} `json:"project"`
	User glUser `json:"user"`

	// push
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

	// merge request and pipeline
	ObjectAttributes struct {
		ID             int64  `json:"id"`
		IID            int64  `json:"iid"`
		Title          string `json:"title"`
		Description    string `json:"description"`
		SourceBranch   string `json:"source_branch"`
		TargetBranch   string `json:"target_branch"`
		Action         string `json:"action"`
		URL            string `json:"url"`
		CreatedAt      string `json:"created_at"`
		UpdatedAt      string `json:"updated_at"`
		MergeCommitSHA string `json:"merge_commit_sha"`
		LastCommit     struct {
			ID string `json:"id"`
		} `json:"last_commit"`
		// pipeline
		Ref    string `json:"ref"`
		Tag    bool   `json:"tag"`
		SHA    string `json:"sha"`
		Status string `json:"status"`
		Name   string `json:"name"`
	} `json:"object_attributes"`

	// deployment
	Status          string `json:"status"`
	StatusChangedAt string `json:"status_changed_at"`
	DeploymentID    int64  `json:"deployment_id"`
	DeployableURL   string `json:"deployable_url"`
	Environment     string `json:"environment"`
	EnvironmentTier string `json:"environment_tier"`
	CommitURL       string `json:"commit_url"`
	ShortSHA        string `json:"short_sha"`
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
	case gitLabMergeRequest:
		d.Events, err = gitLabMergeRequestEvents(p)
	case gitLabPipeline:
		d.Events, err = gitLabPipelineEvents(p)
	case gitLabDeployment:
		d.Events, err = gitLabDeploymentEvents(p)
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

func gitLabMergeRequestEvents(p glPayload) ([]Event, error) {
	mr := p.ObjectAttributes
	e := Event{
		ExternalID: formatID(mr.IID), Number: mr.IID, Title: FirstLine(mr.Title),
		Text: joinText(mr.Title, mr.Description, mr.SourceBranch), Branch: mr.SourceBranch, Target: mr.TargetBranch,
		URL: mr.URL, Author: p.User.author(), SHA: mr.LastCommit.ID,
	}
	switch mr.Action {
	case gitLabActionOpen, gitLabActionReopen, gitLabActionUpdate:
		e.Kind = KindPROpened
	case gitLabActionMerge:
		e.Kind = KindPRMerged
		if mr.MergeCommitSHA != "" {
			e.SHA = mr.MergeCommitSHA
		}
	case gitLabActionClose:
		e.Kind = KindPRClosed
	default:
		return nil, nil // approvals and the like
	}
	at, err := firstTime(mr.UpdatedAt, mr.CreatedAt)
	if err != nil {
		return nil, err
	}
	e.OccurredAt = at
	return []Event{e}, nil
}

func gitLabPipelineEvents(p glPayload) ([]Event, error) {
	pl := p.ObjectAttributes
	if pl.Status != gitLabStatusSuccess || pl.Tag {
		return nil, nil
	}
	at, err := parseTime(pl.CreatedAt)
	if err != nil {
		return nil, err
	}
	return []Event{{
		Kind: KindPipeline, ExternalID: formatID(pl.ID), SHA: pl.SHA, Number: pl.ID, Title: FirstLine(pl.Name),
		Branch: pl.Ref, URL: pl.URL, Author: p.User.author(), OccurredAt: at,
	}}, nil
}

func gitLabDeploymentEvents(p glPayload) ([]Event, error) {
	if p.Status != gitLabStatusSuccess || !isProduction(false, p.EnvironmentTier, p.Environment) {
		return nil, nil
	}
	at, err := parseTime(p.StatusChangedAt)
	if err != nil {
		return nil, err
	}
	return []Event{{
		Kind: KindDeployment, ExternalID: formatID(p.DeploymentID), SHA: p.ShortSHA, Number: p.DeploymentID,
		Title: p.Environment, Branch: p.Ref, URL: firstNonEmpty(p.DeployableURL, p.CommitURL),
		Author: p.User.author(), OccurredAt: at,
	}}, nil
}
