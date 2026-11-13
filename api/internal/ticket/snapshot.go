package ticket

import (
	"errors"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
)

// SnapshotVersion is the shape of SLASnapshot stored in tickets.sla_snapshot. Bump it, and
// keep reading older versions, if the shape ever changes.
const SnapshotVersion = 1

// ErrNoTarget means the snapshot has no target for the priority.
var ErrNoTarget = errors.New("no SLA target for this priority")

// SLASnapshot is the client's SLA plan copied onto a ticket when it is created, so later
// contract or plan edits never change an open ticket. It keeps every priority because an
// agent may change the priority later. Its JSON form is stored; don't rename the tags.
type SLASnapshot struct {
	Version  int          `json:"version"`
	PlanID   uuid.UUID    `json:"planId"`
	PlanName string       `json:"planName"`
	Targets  []sla.Target `json:"targets"`
	// CatalogDeliverMinutes is the catalogue item's delivery time, when the ticket has one.
	CatalogDeliverMinutes *int `json:"catalogDeliverMinutes"`
}

// EffectiveSLA is the target that applies to a ticket's current priority and type.
type EffectiveSLA struct {
	Priority             sla.Priority
	FirstResponseMinutes int
	ResolutionMinutes    int // resolve time for incidents, delivery time for requests
	Calendar             sla.CalendarKind
	FromCatalog          bool // ResolutionMinutes came from the catalogue item
}

// Effective resolves the target for priority p and ticket type t. Service requests take the
// catalogue item's delivery time when the target says so and the ticket has an item.
func (s SLASnapshot) Effective(p sla.Priority, t Type) (EffectiveSLA, error) {
	for _, target := range s.Targets {
		if target.Priority != p {
			continue
		}
		e := EffectiveSLA{
			Priority: p, FirstResponseMinutes: target.FirstResponseMinutes,
			ResolutionMinutes: target.ResolveMinutes, Calendar: target.Calendar,
		}
		if t == TypeRequest {
			e.ResolutionMinutes = target.DeliverMinutes
			if target.DeliverFromCatalog && s.CatalogDeliverMinutes != nil {
				e.ResolutionMinutes, e.FromCatalog = *s.CatalogDeliverMinutes, true
			}
		}
		return e, nil
	}
	return EffectiveSLA{}, ErrNoTarget
}
