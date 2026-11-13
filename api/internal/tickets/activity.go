package tickets

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// ActivityAction is a kind of activity-log entry, as the agent app shows it.
type ActivityAction string

// Activity actions. Message entries split by visibility so the log can say "reply" or "note".
const (
	ActivityCreated         ActivityAction = "created"
	ActivityStatusChanged   ActivityAction = "status_changed"
	ActivityAssigneeChanged ActivityAction = "assignee_changed"
	ActivityPriorityChanged ActivityAction = "priority_changed"
	ActivityReplyAdded      ActivityAction = "reply_added"
	ActivityNoteAdded       ActivityAction = "note_added"
	ActivityFirstResponded  ActivityAction = "first_responded"
)

// ActivityEntry is one line of a ticket's activity log. From and To hold statuses,
// priorities or assignee names, depending on the action.
type ActivityEntry struct {
	ID        uuid.UUID
	At        time.Time
	ActorName *string // nil for the system
	Action    ActivityAction
	From      *string
	To        *string
	Reason    *string
	Cause     *ticket.Cause
	Channel   *ticket.Channel
}

// refJSON is the stored form of refData.
type refJSON struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// Activity returns the ticket's audit trail, oldest first (staff only).
func (s *Service) Activity(ctx context.Context, p *auth.Principal, key string) ([]ActivityEntry, error) {
	a, err := staffAccess(p)
	if err != nil {
		return nil, err
	}
	q := s.store.Q()
	t, err := s.getRow(ctx, q, a, key)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListAuditByTarget(ctx, db.ListAuditByTargetParams{TargetType: &targetTicket, TargetID: &t.ID})
	if err != nil {
		return nil, err
	}
	out := make([]ActivityEntry, 0, len(rows))
	for _, r := range rows {
		if e, ok := activityFromRow(r); ok {
			out = append(out, e)
		}
	}
	return out, nil
}

func activityFromRow(r db.ListAuditByTargetRow) (ActivityEntry, bool) {
	data := map[string]json.RawMessage{}
	_ = json.Unmarshal(r.Data, &data)
	e := ActivityEntry{ID: r.ID, At: r.At, ActorName: r.ActorName, Reason: r.Reason}
	switch r.Action {
	case ActionCreated:
		e.Action = ActivityCreated
		e.Channel = decode[ticket.Channel](data, dataChannel)
	case ActionStatusChanged:
		e.Action = ActivityStatusChanged
		if c := decode[change[string]](data, dataStatus); c != nil {
			e.From, e.To = &c.From, &c.To
		}
		e.Cause = decode[ticket.Cause](data, dataCause)
	case ActionAssigneeChanged:
		e.Action = ActivityAssigneeChanged
		if c := decode[change[*refJSON]](data, dataAssignee); c != nil {
			e.From, e.To = refName(c.From), refName(c.To)
		}
	case ActionPriorityChanged:
		e.Action = ActivityPriorityChanged
		if c := decode[change[string]](data, dataPriority); c != nil {
			e.From, e.To = &c.From, &c.To
		}
	case ActionMessageAdded:
		e.Action = ActivityReplyAdded
		if v := decode[ticket.Visibility](data, dataVisibility); v != nil && *v == ticket.VisibilityInternal {
			e.Action = ActivityNoteAdded
		}
	case ActionFirstResponded:
		e.Action = ActivityFirstResponded
	default:
		return e, false
	}
	return e, true
}

func refName(r *refJSON) *string {
	if r == nil {
		return nil
	}
	return &r.Name
}
