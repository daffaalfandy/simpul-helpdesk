package tickets

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/gitevent"
)

// ShortSHALen is how a commit is named, as on GitHub and GitLab.
const ShortSHALen = 7

// RequestState is where a pull/merge request stands. Values match the CodeRequestState enum.
type RequestState string

const (
	RequestOpen   RequestState = "open" // "In review"
	RequestMerged RequestState = "merged"
	RequestClosed RequestState = "closed" // closed without merging
)

// CodeRequest is a pull/merge request that named the ticket.
type CodeRequest struct {
	Provider gitevent.Provider
	Number   int64
	Title    string
	State    RequestState
	Branch   string
	Target   string
	URL      string
	Author   string // who opened it, or who merged it once merged
	At       time.Time
}

// CodeCommit is a commit that named the ticket.
type CodeCommit struct {
	Provider gitevent.Provider
	SHA      string // short
	Title    string
	Branch   string
	Author   string
	URL      string
	At       time.Time
}

// RepositoryRef is the repository a ticket's project reads code from.
type RepositoryRef struct {
	ID       uuid.UUID
	Provider gitevent.Provider
	FullName string
	WebURL   string
}

// TicketCode is a ticket's code activity, newest first (staff only).
type TicketCode struct {
	Repository *RepositoryRef
	Requests   []CodeRequest
	Commits    []CodeCommit
}

// CardCode is what a board card's code chip says: the latest pull/merge request, or else how
// many commits on which branch, and whether the fix was deployed.
type CardCode struct {
	Provider gitevent.Provider
	Request  *CodeRequest
	Commits  int
	Branch   string // the latest commit's branch
	Deployed bool
}

// Code returns a ticket's code activity. Commit messages are internal, so client staff get 404.
func (s *Service) Code(ctx context.Context, p *auth.Principal, key string) (TicketCode, error) {
	a, err := staffAccess(p)
	if err != nil {
		return TicketCode{}, err
	}
	q := s.store.Q()
	t, err := s.getRow(ctx, q, a, key)
	if err != nil {
		return TicketCode{}, err
	}
	out := TicketCode{Requests: []CodeRequest{}, Commits: []CodeCommit{}}
	repo, err := q.GetTicketRepository(ctx, t.ID)
	switch {
	case err == nil:
		out.Repository = &RepositoryRef{ID: repo.ID, Provider: gitevent.Provider(repo.Provider), FullName: repo.FullName, WebURL: repo.WebUrl}
	case !errors.Is(err, pgx.ErrNoRows):
		return TicketCode{}, err
	}
	rows, err := q.ListLinkedCodeEvents(ctx, []uuid.UUID{t.ID})
	if err != nil {
		return TicketCode{}, err
	}
	out.Requests, out.Commits, _ = summarizeCode(rows)
	return out, nil
}

// cardCodes reads the code chips of the board's cards in one query.
func cardCodes(ctx context.Context, q *db.Queries, ids []uuid.UUID) (map[uuid.UUID]*CardCode, error) {
	rows, err := q.ListLinkedCodeEvents(ctx, ids)
	if err != nil {
		return nil, err
	}
	byTicket := map[uuid.UUID][]db.ListLinkedCodeEventsRow{}
	for _, r := range rows {
		byTicket[*r.TicketID] = append(byTicket[*r.TicketID], r)
	}
	out := make(map[uuid.UUID]*CardCode, len(byTicket))
	for id, evs := range byTicket {
		requests, commits, deployed := summarizeCode(evs)
		c := &CardCode{Provider: gitevent.Provider(evs[0].Provider), Commits: len(commits), Deployed: deployed}
		if len(requests) > 0 {
			c.Request = &requests[0]
		}
		if len(commits) > 0 {
			c.Branch = commits[0].Branch
		}
		out[id] = c
	}
	return out, nil
}

// summarizeCode folds a ticket's code events into its pull/merge requests (one per number,
// in its latest state) and commits, newest first, and says whether a deployment reached it.
func summarizeCode(rows []db.ListLinkedCodeEventsRow) (requests []CodeRequest, commits []CodeCommit, deployed bool) {
	type acc struct {
		latest, opened, merged, closed *db.ListLinkedCodeEventsRow
	}
	byNumber := map[int64]*acc{}
	var numbers []int64
	later := func(a, b *db.ListLinkedCodeEventsRow) bool { return a == nil || b.OccurredAt.After(a.OccurredAt) }
	for i := range rows {
		r := &rows[i]
		switch gitevent.Kind(r.Kind) {
		case gitevent.KindCommit:
			commits = append(commits, CodeCommit{
				Provider: gitevent.Provider(r.Provider), SHA: shortSHA(r.Sha), Title: r.Title, Branch: r.Branch,
				Author: authorOf(r), URL: r.Url, At: r.OccurredAt,
			})
		case gitevent.KindDeployment:
			deployed = true
		case gitevent.KindPROpened, gitevent.KindPRMerged, gitevent.KindPRClosed:
			if r.Number == nil {
				continue
			}
			a, ok := byNumber[*r.Number]
			if !ok {
				a = &acc{}
				byNumber[*r.Number] = a
				numbers = append(numbers, *r.Number)
			}
			if later(a.latest, r) {
				a.latest = r
			}
			switch gitevent.Kind(r.Kind) {
			case gitevent.KindPROpened:
				a.opened = r
			case gitevent.KindPRMerged:
				a.merged = r
			case gitevent.KindPRClosed:
				a.closed = r
			}
		}
	}
	for _, n := range numbers {
		a := byNumber[n]
		req := CodeRequest{
			Provider: gitevent.Provider(a.latest.Provider), Number: n, Title: a.latest.Title, State: RequestOpen,
			Branch: a.latest.Branch, Target: a.latest.Target, URL: a.latest.Url, Author: authorOf(a.latest), At: a.latest.OccurredAt,
		}
		switch {
		case a.merged != nil:
			req.State, req.Author, req.At = RequestMerged, authorOf(a.merged), a.merged.OccurredAt
		case a.closed != nil && (a.opened == nil || !a.closed.OccurredAt.Before(a.opened.OccurredAt)):
			req.State = RequestClosed
		case a.opened != nil:
			req.Author = authorOf(a.opened)
		}
		requests = append(requests, req)
	}
	slices.SortFunc(requests, func(a, b CodeRequest) int { return b.At.Compare(a.At) })
	slices.SortFunc(commits, func(a, b CodeCommit) int { return b.At.Compare(a.At) })
	if requests == nil {
		requests = []CodeRequest{}
	}
	if commits == nil {
		commits = []CodeCommit{}
	}
	return requests, commits, deployed
}

func authorOf(r *db.ListLinkedCodeEventsRow) string {
	switch {
	case r.AuthorUserName != nil:
		return *r.AuthorUserName
	case r.AuthorLogin != "":
		return r.AuthorLogin
	}
	return r.AuthorName
}

func shortSHA(sha string) string {
	if len(sha) > ShortSHALen {
		return sha[:ShortSHALen]
	}
	return sha
}
