package gitevent

import (
	"encoding/json"
	"strconv"
	"strings"
)

// GitHub event names (X-GitHub-Event) the connector reads.
const (
	gitHubPing             = "ping"
	gitHubPush             = "push"
	gitHubPullRequest      = "pull_request"
	gitHubDeploymentStatus = "deployment_status"
	gitHubWorkflowRun      = "workflow_run"
	gitHubCheckSuite       = "check_suite"
)

var gitHubEvents = []string{gitHubPing, gitHubPush, gitHubPullRequest, gitHubDeploymentStatus, gitHubWorkflowRun, gitHubCheckSuite}

// GitHub pull request actions and states the connector reads.
const (
	gitHubActionOpened         = "opened"
	gitHubActionReopened       = "reopened"
	gitHubActionEdited         = "edited"
	gitHubActionReadyForReview = "ready_for_review"
	gitHubActionClosed         = "closed"
	gitHubActionCompleted      = "completed"
	gitHubConclusionSuccess    = "success"
	gitHubStateSuccess         = "success"
)

// Pipelines from workflow runs and check suites share a kind; their ids are prefixed so they
// never collide.
const (
	gitHubRunIDPrefix   = "run:"
	gitHubSuiteIDPrefix = "suite:"
	gitHubCommitPath    = "/commit/"
)

type ghUser struct {
	Login string `json:"login"`
}

type ghRepository struct {
	ID       int64  `json:"id"`
	FullName string `json:"full_name"`
	HTMLURL  string `json:"html_url"`
}

type ghPayload struct {
	Repository ghRepository `json:"repository"`

	// push
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

	// pull_request, workflow_run, check_suite
	Action      string `json:"action"`
	PullRequest struct {
		Number    int64  `json:"number"`
		HTMLURL   string `json:"html_url"`
		Title     string `json:"title"`
		Body      string `json:"body"`
		Merged    bool   `json:"merged"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
		ClosedAt  string `json:"closed_at"`
		MergedAt  string `json:"merged_at"`
		MergeSHA  string `json:"merge_commit_sha"`
		User      ghUser `json:"user"`
		MergedBy  ghUser `json:"merged_by"`
		Head      struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	} `json:"pull_request"`

	// deployment_status
	DeploymentStatus struct {
		State          string `json:"state"`
		Environment    string `json:"environment"`
		TargetURL      string `json:"target_url"`
		EnvironmentURL string `json:"environment_url"`
		CreatedAt      string `json:"created_at"`
		Creator        ghUser `json:"creator"`
	} `json:"deployment_status"`
	Deployment struct {
		ID                    int64  `json:"id"`
		SHA                   string `json:"sha"`
		Ref                   string `json:"ref"`
		Environment           string `json:"environment"`
		ProductionEnvironment bool   `json:"production_environment"`
	} `json:"deployment"`

	WorkflowRun struct {
		ID           int64  `json:"id"`
		Name         string `json:"name"`
		RunNumber    int64  `json:"run_number"`
		HeadBranch   string `json:"head_branch"`
		HeadSHA      string `json:"head_sha"`
		Conclusion   string `json:"conclusion"`
		HTMLURL      string `json:"html_url"`
		CreatedAt    string `json:"created_at"`
		RunStartedAt string `json:"run_started_at"`
		Actor        ghUser `json:"actor"`
	} `json:"workflow_run"`

	CheckSuite struct {
		ID         int64  `json:"id"`
		HeadBranch string `json:"head_branch"`
		HeadSHA    string `json:"head_sha"`
		Conclusion string `json:"conclusion"`
		CreatedAt  string `json:"created_at"`
		App        struct {
			Name string `json:"name"`
		} `json:"app"`
	} `json:"check_suite"`
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
	case gitHubPullRequest:
		d.Events, err = gitHubPullRequestEvents(p)
	case gitHubDeploymentStatus:
		d.Events, err = gitHubDeploymentEvents(p)
	case gitHubWorkflowRun:
		d.Events, err = gitHubWorkflowEvents(p)
	case gitHubCheckSuite:
		d.Events, err = gitHubCheckSuiteEvents(p)
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

func gitHubPullRequestEvents(p ghPayload) ([]Event, error) {
	pr := p.PullRequest
	e := Event{
		ExternalID: formatID(pr.Number), Number: pr.Number, Title: FirstLine(pr.Title),
		Text: joinText(pr.Title, pr.Body, pr.Head.Ref), Branch: pr.Head.Ref, Target: pr.Base.Ref, URL: pr.HTMLURL,
	}
	var at string
	switch p.Action {
	case gitHubActionOpened, gitHubActionReopened, gitHubActionReadyForReview, gitHubActionEdited:
		// An edit may add a ticket key to the title or description.
		e.Kind, e.SHA, e.Author, at = KindPROpened, pr.Head.SHA, Author{Login: pr.User.Login}, pr.UpdatedAt
	case gitHubActionClosed:
		if pr.Merged {
			e.Kind, e.SHA, e.Author, at = KindPRMerged, pr.MergeSHA, Author{Login: pr.MergedBy.Login}, pr.MergedAt
		} else {
			e.Kind, e.SHA, e.Author, at = KindPRClosed, pr.Head.SHA, Author{Login: pr.User.Login}, pr.ClosedAt
		}
	default:
		return nil, nil // new commits, labels, reviews: the push and merge events cover them
	}
	t, err := firstTime(at, pr.CreatedAt)
	if err != nil {
		return nil, err
	}
	e.OccurredAt = t
	return []Event{e}, nil
}

func gitHubDeploymentEvents(p ghPayload) ([]Event, error) {
	s, dep := p.DeploymentStatus, p.Deployment
	if s.State != gitHubStateSuccess || !isProduction(dep.ProductionEnvironment, s.Environment, dep.Environment) {
		return nil, nil
	}
	at, err := parseTime(s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return []Event{{
		Kind: KindDeployment, ExternalID: formatID(dep.ID), SHA: dep.SHA, Number: dep.ID,
		Title: firstNonEmpty(s.Environment, dep.Environment), Branch: dep.Ref,
		URL:    firstNonEmpty(s.TargetURL, s.EnvironmentURL, commitURL(p.Repository.HTMLURL, dep.SHA)),
		Author: Author{Login: s.Creator.Login}, OccurredAt: at,
	}}, nil
}

func gitHubWorkflowEvents(p ghPayload) ([]Event, error) {
	r := p.WorkflowRun
	if p.Action != gitHubActionCompleted || r.Conclusion != gitHubConclusionSuccess {
		return nil, nil
	}
	at, err := firstTime(r.RunStartedAt, r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return []Event{{
		Kind: KindPipeline, ExternalID: gitHubRunIDPrefix + formatID(r.ID), SHA: r.HeadSHA, Number: r.RunNumber,
		Title: FirstLine(r.Name), Branch: r.HeadBranch, URL: r.HTMLURL, Author: Author{Login: r.Actor.Login}, OccurredAt: at,
	}}, nil
}

func gitHubCheckSuiteEvents(p ghPayload) ([]Event, error) {
	s := p.CheckSuite
	if p.Action != gitHubActionCompleted || s.Conclusion != gitHubConclusionSuccess || s.HeadBranch == "" {
		return nil, nil
	}
	at, err := parseTime(s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return []Event{{
		Kind: KindPipeline, ExternalID: gitHubSuiteIDPrefix + formatID(s.ID), SHA: s.HeadSHA,
		Title: FirstLine(s.App.Name), Branch: s.HeadBranch, URL: commitURL(p.Repository.HTMLURL, s.HeadSHA), OccurredAt: at,
	}}, nil
}

func commitURL(repoURL, sha string) string {
	if repoURL == "" || sha == "" {
		return repoURL
	}
	return strings.TrimSuffix(repoURL, "/") + gitHubCommitPath + sha
}

func formatID(n int64) string {
	if n == 0 {
		return ""
	}
	return strconv.FormatInt(n, 10)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
