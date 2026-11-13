// Package priorities holds the priority names and descriptions requesters see. They are shared by
// every project (follow-up #1); SLA targets live on each project.
package priorities

import (
	"context"
	"strings"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
)

// Audit action and target type. Priority levels have no UUID, so entries carry the code in data.
const (
	ActionUpdated       = "priority_level.updated"
	TargetPriorityLevel = "priority_level"
)

const (
	dataCode        = "code"
	dataName        = "name"
	dataDescription = "description"
)

// Field limits, matching the database CHECK constraints.
const (
	MaxNameLen        = 40
	MaxDescriptionLen = 200
)

// Level is one priority's name and description.
type Level struct {
	Code        sla.Priority
	Name        string
	Description string
}

type Service struct {
	store *store.Store
}

func NewService(st *store.Store) *Service { return &Service{store: st} }

// List returns the four levels, P1 first.
func (s *Service) List(ctx context.Context) ([]Level, error) {
	rows, err := s.store.Q().ListPriorityLevels(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Level, 0, len(rows))
	for _, r := range rows {
		out = append(out, Level{Code: sla.Priority(r.Code), Name: r.Name, Description: r.Description})
	}
	return out, nil
}

// Update replaces every level's name and description. Each changed level gets an audit entry.
func (s *Service) Update(ctx context.Context, actor audit.Actor, in []Level) ([]Level, error) {
	levels, err := normalize(in)
	if err != nil {
		return nil, err
	}
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		old, err := tx.Q.ListPriorityLevelsForUpdate(ctx)
		if err != nil {
			return err
		}
		before := make(map[string]db.ListPriorityLevelsForUpdateRow, len(old))
		for _, r := range old {
			before[r.Code] = r
		}
		for _, l := range levels {
			prev := before[string(l.Code)]
			changes := audit.Changes{}
			audit.Track(changes, dataName, prev.Name, l.Name)
			audit.Track(changes, dataDescription, prev.Description, l.Description)
			if len(changes) == 0 {
				continue
			}
			if err := tx.Q.UpdatePriorityLevel(ctx, db.UpdatePriorityLevelParams{
				Code: string(l.Code), Name: l.Name, Description: l.Description,
			}); err != nil {
				return err
			}
			changes[dataCode] = l.Code
			if err := audit.Write(ctx, tx.Q, actor.Entry(ActionUpdated, TargetPriorityLevel, nil, changes)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.List(ctx)
}

// normalize trims the text and checks there is exactly one level per priority, ordered P1–P4.
func normalize(in []Level) ([]Level, error) {
	invalid := apperr.Invalid(apperr.CodeInvalidPriorityLevels,
		"Give every priority a name and a description", "Names are up to 40 characters, descriptions up to 200.")
	if len(in) != len(sla.Priorities) {
		return nil, invalid
	}
	byCode := make(map[sla.Priority]Level, len(in))
	for _, l := range in {
		l.Name = strings.TrimSpace(l.Name)
		l.Description = strings.TrimSpace(l.Description)
		_, dup := byCode[l.Code]
		if !l.Code.Valid() || dup ||
			l.Name == "" || len([]rune(l.Name)) > MaxNameLen ||
			l.Description == "" || len([]rune(l.Description)) > MaxDescriptionLen {
			return nil, invalid
		}
		byCode[l.Code] = l
	}
	out := make([]Level, 0, len(sla.Priorities))
	for _, p := range sla.Priorities {
		out = append(out, byCode[p])
	}
	return out, nil
}
