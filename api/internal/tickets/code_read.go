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

// CodeCommit is a commit that named the ticket, with every branch it has reached so far.
type CodeCommit struct {
	Provider gitevent.Provider
	SHA      string // short
	Title    string
	Branches []string // in the order they were seen: the fix branch, then main
	Author   string
	URL      string
	At       time.Time
}

// RepositoryRef is the repository a ticket's project reads commits from.
type RepositoryRef struct {
	ID       uuid.UUID
	Provider gitevent.Provider
	FullName string
	WebURL   string
}

// TicketCode is a ticket's code activity, newest first (staff only).
type TicketCode struct {
	Repository *RepositoryRef
	Commits    []CodeCommit
}

// CardCode is what a board card's code chip says: how many commits named the ticket, and the
// latest one with the last branch it reached.
type CardCode struct {
	Provider gitevent.Provider
	Commits  int
	SHA      string
	Branch   string
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
	out := TicketCode{Commits: []CodeCommit{}}
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
	out.Commits = commitsOf(rows)
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
		commits := commitsOf(evs)
		latest := commits[0]
		out[id] = &CardCode{Provider: latest.Provider, Commits: len(commits), SHA: latest.SHA, Branch: latest.Branches[len(latest.Branches)-1]}
	}
	return out, nil
}

// commitsOf lists a ticket's commits, newest first.
func commitsOf(rows []db.ListLinkedCodeEventsRow) []CodeCommit {
	out := make([]CodeCommit, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		branches := r.Branches
		if len(branches) == 0 {
			branches = []string{""}
		}
		out = append(out, CodeCommit{
			Provider: gitevent.Provider(r.Provider), SHA: shortSHA(r.Sha), Title: r.Title, Branches: branches,
			Author: authorOf(r), URL: r.Url, At: r.OccurredAt,
		})
	}
	slices.SortFunc(out, func(a, b CodeCommit) int { return b.At.Compare(a.At) })
	return out
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
