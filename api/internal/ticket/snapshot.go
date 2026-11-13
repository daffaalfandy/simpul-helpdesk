package ticket

import (
	"errors"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
)

// SnapshotVersion is the shape of SLASnapshot stored in tickets.sla_snapshot. Version 1 held a
// client SLA plan; migration 00005 rewrote every ticket to version 2, the project format. Bump
// it, and convert or keep reading older versions, if the shape changes again.
const SnapshotVersion = 2

var (
	// ErrNoTarget means the snapshot has no target for the priority.
	ErrNoTarget = errors.New("no SLA target for this priority")
	// ErrSnapshotVersion means the stored snapshot is in a shape this build can't read.
	ErrSnapshotVersion = errors.New("unknown SLA snapshot version")
)

// SLASnapshot is the project's SLA copied onto a ticket at creation, every priority included,
// so SLA edits never change an open ticket. Its JSON is stored: don't rename the tags.
type SLASnapshot struct {
	Version     int       `json:"version"`
	ProjectID   uuid.UUID `json:"projectId"`
	ProjectName string    `json:"projectName"`
	sla.ProjectSLA
}

// EffectiveSLA is what applies to a ticket's current priority and type.
type EffectiveSLA struct {
	Priority             sla.Priority
	FirstResponse        sla.Duration
	Resolution           sla.Duration // resolve for incidents, deliver for service requests
	Calendar             sla.CalendarKind
	PauseOnWaitingClient bool
	PauseOnUAT           bool
}

// Effective resolves the targets for priority p and ticket type t.
func (s SLASnapshot) Effective(p sla.Priority, t Type) (EffectiveSLA, error) {
	target, ok := s.Target(p)
	if !ok {
		return EffectiveSLA{}, ErrNoTarget
	}
	e := EffectiveSLA{
		Priority: p, FirstResponse: target.FirstResponse, Resolution: target.Resolve, Calendar: target.Calendar,
		PauseOnWaitingClient: s.PauseOnWaitingClient, PauseOnUAT: s.PauseOnUAT,
	}
	if t == TypeRequest {
		e.Resolution = target.Deliver
	}
	return e, nil
}
