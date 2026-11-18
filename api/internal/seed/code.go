package seed

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/connector"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/gitevent"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/secretbox"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

// DevWebhookSecret is every demo repository's webhook secret, so deliveries can be tried locally.
const DevWebhookSecret = "simdesk-dev-webhook"

// Demo repositories (Integrations mock), by full name.
const (
	repoEOffice = "vendor-id/eoffice-jambi"
	repoBintan  = "vendor-id/web-bintan"
	repoSIPD    = "gov/sipd-connector"
)

// demoRepo is a connected repository and the projects that read commits from it. The SIPD
// connector serves two clients' projects: the same system run for each.
type demoRepo struct {
	provider gitevent.Provider
	url      string
	projects [][2]string // client, project
}

var demoRepos = []demoRepo{
	{gitevent.ProviderGitHub, "https://github.com/" + repoEOffice, [][2]string{{RequesterClient, projectEOffice}}},
	{gitevent.ProviderGitHub, "https://github.com/" + repoBintan, [][2]string{{clientBintan, projectBintanWeb}}},
	{gitevent.ProviderGitLab, "https://gitlab.vendor.co.id/" + repoSIPD, [][2]string{{clientRiau, projectSIPD}, {clientBatam, projectFinanceApps}}},
}

// gitLogins are the demo staff's GitHub/GitLab usernames.
var gitLogins = map[string]string{
	emailAdi: "adi.saputra", emailRina: "rina.p", emailYoga: "yoga.h", emailDewi: "dewi.w",
}

// commitPath is where a commit lives under a repository's web address.
var commitPath = map[gitevent.Provider]string{gitevent.ProviderGitHub: "/commit/", gitevent.ProviderGitLab: "/-/commit/"}

// demoCode is a commit naming a demo ticket, pushed minutes after the ticket was opened, then to
// each later branch (the merge into main) as it lands. Titles may hold %s for the ticket's key.
type demoCode struct {
	after    int
	repo     string
	sha      string
	title    string
	branches []string // first push first
	author   string   // email
}

// unknownKeyCode is a push naming a ticket that doesn't exist, for the recent-events log.
var unknownKeyCode = demoCode{
	repo: repoEOffice, sha: "f00ba125c2d94e1b8a3f6c7d8e9f0a1b2c3d4e5f",
	title: "refs HD-99999 clean up old session keys", branches: []string{"main"}, author: emailAdi,
}

// unknownKeyAge is how long ago that push happened, in minutes.
const unknownKeyAge = 50

// demoRepoRefs are the seeded repositories by full name.
type demoRepoRef struct {
	id       uuid.UUID
	provider gitevent.Provider
	webURL   string
}

// seedRepositories connects the demo repositories, links their projects, and marks them as
// having sent webhooks. A repository seeded before keeps its row.
func seedRepositories(ctx context.Context, q *db.Queries, box *secretbox.Box, now time.Time, r refs) (map[string]demoRepoRef, error) {
	out := map[string]demoRepoRef{}
	for _, d := range demoRepos {
		loc, err := connector.ParseLocation(d.provider, d.url)
		if err != nil {
			return nil, fmt.Errorf("repository %s: %w", d.url, err)
		}
		existing, err := q.GetRepositoryByName(ctx, db.GetRepositoryByNameParams{OrganizationID: r.org, Provider: string(d.provider), Host: loc.Host, FullName: loc.FullName})
		id := existing.ID
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			if id, err = uuid.NewV7(); err != nil {
				return nil, err
			}
			if err := q.InsertRepository(ctx, db.InsertRepositoryParams{
				ID: id, OrganizationID: r.org, Provider: string(d.provider), Host: loc.Host, FullName: loc.FullName, WebUrl: loc.WebURL,
				WebhookSecret: connector.Seal(box, id, DevWebhookSecret), CreatedBy: r.users[emailAdmin],
			}); err != nil {
				return nil, fmt.Errorf("repository %s: %w", d.url, err)
			}
		case err != nil:
			return nil, err
		}
		var ids []uuid.UUID
		for _, p := range d.projects {
			ids = append(ids, r.projects[p[0]][p[1]])
		}
		if err := q.SetProjectsRepository(ctx, db.SetProjectsRepositoryParams{RepositoryID: &id, Ids: ids}); err != nil {
			return nil, err
		}
		if err := q.MarkSeedRepositoryDelivered(ctx, db.MarkSeedRepositoryDeliveredParams{ID: id, At: &now}); err != nil {
			return nil, err
		}
		out[loc.FullName] = demoRepoRef{id: id, provider: d.provider, webURL: loc.WebURL}
	}
	return out, nil
}

// seedCodeEvent stores a demo commit, pushed to each of its branches in turn, and returns it as
// it reaches a ticket.
func seedCodeEvent(ctx context.Context, q *db.Queries, repos map[string]demoRepoRef, r refs, c demoCode, key string, at time.Time) (tickets.CodeEvent, error) {
	repo := repos[c.repo]
	title := c.title
	if key != "" {
		title = fmt.Sprintf(c.title, key)
	}
	author := r.user(c.author)
	var id uuid.UUID
	for _, branch := range c.branches {
		stored, err := q.UpsertCodeEvent(ctx, db.UpsertCodeEventParams{
			RepositoryID: repo.id, Kind: string(gitevent.KindCommit), ExternalID: c.sha, Sha: c.sha,
			Title: gitevent.FirstLine(title), Branch: branch, Url: repo.webURL + commitPath[repo.provider] + c.sha,
			AuthorName: author.Name, AuthorLogin: gitLogins[c.author], AuthorUserID: &author.ID, OccurredAt: at,
		})
		if err != nil {
			return tickets.CodeEvent{}, err
		}
		id = stored.ID
	}
	return tickets.CodeEvent{
		ID: id, Provider: repo.provider, Kind: gitevent.KindCommit, Ref: c.sha[:tickets.ShortSHALen],
		Branch: c.branches[0], At: at, AuthorID: &author.ID, AuthorName: author.Name,
	}, nil
}

// seedUnknownKey logs a push naming a ticket that doesn't exist.
func seedUnknownKey(ctx context.Context, q *db.Queries, repos map[string]demoRepoRef, r refs, now time.Time) error {
	at := now.Add(-unknownKeyAge * time.Minute)
	e, err := seedCodeEvent(ctx, q, repos, r, unknownKeyCode, "", at)
	if err != nil {
		return err
	}
	return q.InsertSeedCodeLink(ctx, db.InsertSeedCodeLinkParams{
		CodeEventID: e.ID, TicketKey: unknownTicketKey, Outcome: string(tickets.CodeUnknownTicket), At: at,
	})
}

// unknownTicketKey is the key unknownKeyCode names.
const unknownTicketKey = "HD-99999"
