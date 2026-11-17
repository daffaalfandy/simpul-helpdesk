package projects

import (
	"bytes"
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/clients"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
)

// The saved project filter (owner, 17 Nov): the projects a user sees on the board, its counts
// and the staff ticket list, per organization. No projects means every project. Notifications
// and reports ignore it.
const (
	ActionFilterChanged = "project.filter_changed"
	// MaxFilterProjects caps one filter: the API's ProjectFilter.projectIds maxItems.
	MaxFilterProjects = 200
	dataProjects      = "projects"
)

// FilterOf is a user's saved filter in an organization, without the projects they can no
// longer see (agentID: an agent's clients only). Empty, never nil, means every project.
func FilterOf(ctx context.Context, q *db.Queries, userID, orgID uuid.UUID, agentID *uuid.UUID) ([]uuid.UUID, error) {
	ids, err := q.ListProjectFilter(ctx, db.ListProjectFilterParams{UserID: userID, OrganizationID: orgID, AgentID: agentID})
	if err != nil {
		return nil, err
	}
	if ids == nil {
		ids = []uuid.UUID{}
	}
	return ids, nil
}

// Filter returns p's saved filter in the organization they work in.
func (s *Service) Filter(ctx context.Context, p *auth.Principal) ([]uuid.UUID, error) {
	return FilterOf(ctx, s.store.Q(), p.UserID, p.OrganizationID, clients.AgentScope(p))
}

// SetFilter replaces p's filter in the organization they work in. Every project must be one
// p can see there.
func (s *Service) SetFilter(ctx context.Context, p *auth.Principal, ids []uuid.UUID) ([]uuid.UUID, error) {
	ids = sortedUnique(ids)
	if len(ids) > MaxFilterProjects {
		return nil, invalidFilter()
	}
	agentID := clients.AgentScope(p)
	err := s.store.Tx(ctx, func(tx store.Tx) error {
		visible, err := tx.Q.CountVisibleProjects(ctx, db.CountVisibleProjectsParams{OrganizationID: p.OrganizationID, Ids: ids, AgentID: agentID})
		if err != nil {
			return err
		}
		if int(visible) != len(ids) {
			return invalidFilter()
		}
		old, err := FilterOf(ctx, tx.Q, p.UserID, p.OrganizationID, agentID)
		if err != nil {
			return err
		}
		if err := tx.Q.ClearProjectFilter(ctx, db.ClearProjectFilterParams{UserID: p.UserID, OrganizationID: p.OrganizationID}); err != nil {
			return err
		}
		if len(ids) > 0 {
			if err := tx.Q.AddToProjectFilter(ctx, db.AddToProjectFilterParams{UserID: p.UserID, OrganizationID: p.OrganizationID, ProjectIds: ids}); err != nil {
				return err
			}
		}
		if slices.Equal(old, ids) {
			return nil
		}
		changes := audit.Changes{dataProjects: audit.Change{From: old, To: ids}}
		return audit.Write(ctx, tx.Q, p.AuditActor().Entry(ActionFilterChanged, auth.TargetUser, &p.UserID, changes))
	})
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func invalidFilter() error {
	return apperr.Invalid(apperr.CodeInvalidProject, "Choose projects you can see", "")
}

// sortedUnique orders ids as the database compares uuids (byte by byte), without repeats.
func sortedUnique(ids []uuid.UUID) []uuid.UUID {
	out := slices.Clone(ids)
	slices.SortFunc(out, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })
	out = slices.Compact(out)
	if out == nil {
		out = []uuid.UUID{}
	}
	return out
}
