// Package connector is the read-only GitHub/GitLab connector (HANDOFF §8): the repositories an
// admin connects, the webhooks they send, and what their commits, pull/merge requests, CI runs
// and deployments do to tickets. Parsing lives in the pure gitevent, commitmsg and codeflow
// packages; ticket changes go through the tickets service.
package connector

import (
	"context"
	"crypto/rand"
	"errors"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/clients"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/gitevent"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/secretbox"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

// Audit actions and target type written by this package.
const (
	ActionRepositoryCreated       = "repository.created"
	ActionRepositoryUpdated       = "repository.updated"
	ActionRepositorySecretRotated = "repository.secret_rotated"
	ActionRepositoryRemoved       = "repository.removed"

	TargetRepository = "repository"
)

const (
	dataProvider = "provider"
	dataHost     = "host"
	dataFullName = "full_name"
	dataProjects = "project_ids"
	dataHidden   = "hidden"
)

// WebhookPath is where webhooks arrive under the App URL: WebhookPath + repository id. It
// mirrors the webhook operation's path in openapi.yaml (checked by a test in httpapi).
const WebhookPath = "/api/v1/webhooks/"

// MaxLinkedProjects caps the projects one request links, matching the API.
const MaxLinkedProjects = 200

const constraintRepositoryName = "repositories_name_key"

// Status is how a repository's webhook is doing. Values match the RepositoryStatus enum.
type Status string

const (
	StatusWaiting   Status = "waiting"   // no verified delivery yet
	StatusConnected Status = "connected" // the latest delivery was accepted
	StatusFailing   Status = "failing"   // the latest delivery was refused
)

// DeliveryError says why a delivery was refused. Values match the repositories.last_error CHECK.
type DeliveryError string

const (
	ErrorBadSignature       DeliveryError = "bad_signature"
	ErrorRepositoryMismatch DeliveryError = "repository_mismatch"
	ErrorMalformed          DeliveryError = "malformed"
	ErrorTooLarge           DeliveryError = "too_large"
)

// LinkedProject is a project that reads code from a repository.
type LinkedProject struct {
	ID     uuid.UUID
	Name   string
	Client clients.Ref
}

// Repository is a connected repository as the Integrations page shows it.
type Repository struct {
	ID             uuid.UUID
	Provider       gitevent.Provider
	Location       Location
	WebhookURL     string
	Status         Status
	LastDeliveryAt *time.Time
	LastError      *DeliveryError
	LastErrorAt    *time.Time
	Projects       []LinkedProject
}

// Overview is everything the Integrations page lists.
type Overview struct {
	Repositories []Repository
	TicketPrefix string
}

// CreateInput connects a repository and links projects to it.
type CreateInput struct {
	Provider   gitevent.Provider
	URL        string
	ProjectIDs []uuid.UUID
}

type Service struct {
	store  *store.Store
	box    *secretbox.Box
	appURL *url.URL
	jobs   tickets.JobInserter
}

func NewService(st *store.Store, box *secretbox.Box, appURL *url.URL, jobs tickets.JobInserter) *Service {
	return &Service{store: st, box: box, appURL: appURL, jobs: jobs}
}

// Overview lists the connected repositories with their projects.
func (s *Service) Overview(ctx context.Context) (Overview, error) {
	q := s.store.Q()
	org, err := q.GetOrganization(ctx)
	if err != nil {
		return Overview{}, err
	}
	repos, err := s.list(ctx, q)
	if err != nil {
		return Overview{}, err
	}
	return Overview{Repositories: repos, TicketPrefix: org.TicketPrefix}, nil
}

// Create connects a repository and returns it with its webhook secret, shown this once. A
// repository removed earlier is connected again, with a new secret.
func (s *Service) Create(ctx context.Context, actor audit.Actor, createdBy uuid.UUID, in CreateInput) (Repository, string, error) {
	loc, err := ParseLocation(in.Provider, in.URL)
	if err != nil {
		return Repository{}, "", apperr.Invalid(apperr.CodeInvalidRepositoryURL, "Check the repository address", err.Error())
	}
	secret := newSecret()
	var id uuid.UUID
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		existing, err := tx.Q.GetRepositoryByName(ctx, db.GetRepositoryByNameParams{
			Provider: string(in.Provider), Host: loc.Host, FullName: loc.FullName,
		})
		switch {
		case err == nil && !existing.Hidden:
			return errTaken()
		case err == nil:
			id = existing.ID
			if err := tx.Q.RestoreRepository(ctx, db.RestoreRepositoryParams{
				ID: id, Host: loc.Host, FullName: loc.FullName, WebUrl: loc.WebURL, WebhookSecret: s.seal(id, secret),
			}); err != nil {
				return err
			}
		case errors.Is(err, pgx.ErrNoRows):
			if id, err = uuid.NewV7(); err != nil {
				return err
			}
			if err := tx.Q.InsertRepository(ctx, db.InsertRepositoryParams{
				ID: id, Provider: string(in.Provider), Host: loc.Host, FullName: loc.FullName, WebUrl: loc.WebURL,
				WebhookSecret: s.seal(id, secret), CreatedBy: createdBy,
			}); err != nil {
				return err
			}
		default:
			return err
		}
		if _, err := link(ctx, tx.Q, id, in.ProjectIDs); err != nil {
			return err
		}
		return audit.Write(ctx, tx.Q, actor.Entry(ActionRepositoryCreated, TargetRepository, &id, map[string]any{
			dataProvider: in.Provider, dataHost: loc.Host, dataFullName: loc.FullName, dataProjects: in.ProjectIDs,
		}))
	})
	if err != nil {
		return Repository{}, "", repositoryError(err)
	}
	r, err := s.get(ctx, id)
	return r, secret, err
}

// SetProjects replaces the projects that read code from a repository.
func (s *Service) SetProjects(ctx context.Context, actor audit.Actor, id uuid.UUID, projectIDs []uuid.UUID) (Repository, error) {
	err := s.store.Tx(ctx, func(tx store.Tx) error {
		if _, err := lockVisible(ctx, tx.Q, id); err != nil {
			return err
		}
		before, err := link(ctx, tx.Q, id, projectIDs)
		if err != nil || sameIDs(before, projectIDs) {
			return err
		}
		return audit.Write(ctx, tx.Q, actor.Entry(ActionRepositoryUpdated, TargetRepository, &id, map[string]any{
			dataProjects: audit.Change{From: before, To: projectIDs},
		}))
	})
	if err != nil {
		return Repository{}, repositoryError(err)
	}
	return s.get(ctx, id)
}

// RotateSecret gives a repository a new webhook secret, shown this once. The old one stops
// working at once.
func (s *Service) RotateSecret(ctx context.Context, actor audit.Actor, id uuid.UUID) (Repository, string, error) {
	secret := newSecret()
	err := s.store.Tx(ctx, func(tx store.Tx) error {
		if _, err := lockVisible(ctx, tx.Q, id); err != nil {
			return err
		}
		if err := tx.Q.SetRepositorySecret(ctx, db.SetRepositorySecretParams{ID: id, WebhookSecret: s.seal(id, secret)}); err != nil {
			return err
		}
		return audit.Write(ctx, tx.Q, actor.Entry(ActionRepositorySecretRotated, TargetRepository, &id, nil))
	})
	if err != nil {
		return Repository{}, "", repositoryError(err)
	}
	r, err := s.get(ctx, id)
	return r, secret, err
}

// Remove disconnects a repository: its projects lose it and its webhooks are refused. One with
// code events is hidden, so tickets keep their code activity; one without is deleted.
func (s *Service) Remove(ctx context.Context, actor audit.Actor, id uuid.UUID) error {
	err := s.store.Tx(ctx, func(tx store.Tx) error {
		if _, err := lockVisible(ctx, tx.Q, id); err != nil {
			return err
		}
		before, err := link(ctx, tx.Q, id, nil)
		if err != nil {
			return err
		}
		used, err := tx.Q.RepositoryHasEvents(ctx, id)
		if err != nil {
			return err
		}
		if used {
			err = tx.Q.HideRepository(ctx, id)
		} else {
			err = tx.Q.DeleteRepository(ctx, id)
		}
		if err != nil {
			return err
		}
		return audit.Write(ctx, tx.Q, actor.Entry(ActionRepositoryRemoved, TargetRepository, &id, map[string]any{
			dataProjects: before, dataHidden: used,
		}))
	})
	return repositoryError(err)
}

// link makes exactly projectIDs read code from the repository and returns the ones that did
// before. Projects linked elsewhere move here. A project whose Definition of Done reads code
// can't be left without a repository.
func link(ctx context.Context, q *db.Queries, repoID uuid.UUID, projectIDs []uuid.UUID) ([]uuid.UUID, error) {
	if len(projectIDs) > MaxLinkedProjects {
		return nil, apperr.InvalidRequest("too many projects")
	}
	projectIDs = uniqueIDs(projectIDs)
	rows, err := q.LockProjectsForLink(ctx, db.LockProjectsForLinkParams{Ids: projectIDs, RepositoryID: &repoID})
	if err != nil {
		return nil, err
	}
	var before, leaving []uuid.UUID
	var stranded []string
	found := 0
	for _, r := range rows {
		wanted := slices.Contains(projectIDs, r.ID)
		if wanted {
			found++
		}
		if r.RepositoryID == nil || *r.RepositoryID != repoID {
			continue
		}
		before = append(before, r.ID)
		if wanted {
			continue
		}
		leaving = append(leaving, r.ID)
		cfg, err := dod.Parse(r.DodConfig)
		if err != nil {
			return nil, err
		}
		if errors.Is(dod.Validate(cfg, false), dod.ErrNeedsRepository) {
			stranded = append(stranded, r.Name)
		}
	}
	switch {
	case found != len(projectIDs):
		return nil, apperr.Invalid(apperr.CodeInvalidProject, "Choose projects from the list", "")
	case len(stranded) > 0:
		return nil, apperr.Conflict(apperr.CodeRepositoryInUse, "These projects read merges, CI or deploys from it: "+
			strings.Join(stranded, ", "), "Turn those Definition of Done conditions off first, or link the projects to another repository.")
	}
	if len(projectIDs) > 0 {
		if err := q.SetProjectsRepository(ctx, db.SetProjectsRepositoryParams{RepositoryID: &repoID, Ids: projectIDs}); err != nil {
			return nil, err
		}
	}
	if len(leaving) > 0 {
		if err := q.SetProjectsRepository(ctx, db.SetProjectsRepositoryParams{Ids: leaving}); err != nil {
			return nil, err
		}
	}
	return before, nil
}

func lockVisible(ctx context.Context, q *db.Queries, id uuid.UUID) (db.GetRepositoryForUpdateRow, error) {
	r, err := q.GetRepositoryForUpdate(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && r.Hidden) {
		return r, apperr.NotFound()
	}
	return r, err
}

func (s *Service) get(ctx context.Context, id uuid.UUID) (Repository, error) {
	repos, err := s.list(ctx, s.store.Q())
	if err != nil {
		return Repository{}, err
	}
	for _, r := range repos {
		if r.ID == id {
			return r, nil
		}
	}
	return Repository{}, apperr.NotFound()
}

func (s *Service) list(ctx context.Context, q *db.Queries) ([]Repository, error) {
	rows, err := q.ListRepositories(ctx)
	if err != nil {
		return nil, err
	}
	linked, err := q.ListLinkedProjects(ctx)
	if err != nil {
		return nil, err
	}
	byRepo := map[uuid.UUID][]LinkedProject{}
	for _, p := range linked {
		byRepo[*p.RepositoryID] = append(byRepo[*p.RepositoryID], LinkedProject{
			ID: p.ID, Name: p.Name, Client: clients.Ref{ID: p.ClientID, Name: p.ClientName},
		})
	}
	out := make([]Repository, len(rows))
	for i, r := range rows {
		var lastError *DeliveryError
		if r.LastError != nil {
			e := DeliveryError(*r.LastError)
			lastError = &e
		}
		projects := byRepo[r.ID]
		if projects == nil {
			projects = []LinkedProject{}
		}
		out[i] = Repository{
			ID: r.ID, Provider: gitevent.Provider(r.Provider),
			Location:   Location{Host: r.Host, FullName: r.FullName, WebURL: r.WebUrl},
			WebhookURL: s.WebhookURL(r.ID), Status: statusOf(r.LastDeliveryAt, r.LastErrorAt),
			LastDeliveryAt: r.LastDeliveryAt, LastError: lastError, LastErrorAt: r.LastErrorAt, Projects: projects,
		}
	}
	return out, nil
}

// WebhookURL is where a repository's provider sends its webhooks.
func (s *Service) WebhookURL(id uuid.UUID) string {
	return strings.TrimRight(s.appURL.String(), "/") + WebhookPath + id.String()
}

// statusOf reads a repository's status from its latest accepted and refused deliveries.
func statusOf(lastDelivery, lastErrorAt *time.Time) Status {
	switch {
	case lastErrorAt != nil && (lastDelivery == nil || lastErrorAt.After(*lastDelivery)):
		return StatusFailing
	case lastDelivery != nil:
		return StatusConnected
	}
	return StatusWaiting
}

// newSecret is a webhook secret: 128 random bits, typeable and safe in any settings form.
func newSecret() string { return rand.Text() }

func (s *Service) seal(id uuid.UUID, secret string) []byte { return s.box.Seal([]byte(secret), id[:]) }

// open reads a repository's webhook secret.
func (s *Service) open(id uuid.UUID, sealed []byte) ([]byte, error) { return s.box.Open(sealed, id[:]) }

func uniqueIDs(ids []uuid.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

func sameIDs(a, b []uuid.UUID) bool {
	a, b = uniqueIDs(a), uniqueIDs(b)
	if len(a) != len(b) {
		return false
	}
	for _, id := range a {
		if !slices.Contains(b, id) {
			return false
		}
	}
	return true
}

func errTaken() error {
	return apperr.Conflict(apperr.CodeRepositoryTaken, "This repository is already connected", "")
}

func repositoryError(err error) error {
	if store.IsViolation(err, store.SQLStateUniqueViolation, constraintRepositoryName) {
		return errTaken()
	}
	return err
}
