package tickets

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// SeedTicket is a demo ticket with its history, for the dev seed only. History is written
// the way the service writes it, so the activity log and progress tracker look real.
type SeedTicket struct {
	Number        int64
	Prefix        string
	Type          ticket.Type
	Priority      sla.Priority
	Channel       ticket.Channel
	ClientID      uuid.UUID
	Requester     Ref
	CreatedBy     Ref
	ProjectID     uuid.UUID
	CatalogItemID *uuid.UUID
	Subject       string
	Description   string
	CreatedAt     time.Time
	Steps         []SeedStep
	Messages      []SeedMessage
}

// SeedStep is a change after creation: an assignee change, a status change, or both.
type SeedStep struct {
	At       time.Time
	Actor    Ref
	Assignee *Ref          // assign to this person
	Status   ticket.Status // move to this status
}

type SeedMessage struct {
	At          time.Time
	Author      Ref
	FromSupport bool
	Visibility  ticket.Visibility
	Body        string
}

type seedEntry struct {
	at     time.Time
	actor  Ref
	action string
	data   map[string]any
}

// InsertSeed writes the ticket, its messages and its audit history. It reports false and
// writes nothing when a ticket with the key already exists, so seeding can be repeated.
func InsertSeed(ctx context.Context, q *db.Queries, t SeedTicket) (bool, error) {
	status, assignee := ticket.StatusNew, (*Ref)(nil)
	var resolvedAt *time.Time
	entries := []seedEntry{}
	for _, st := range t.Steps {
		if st.Assignee != nil {
			entries = append(entries, seedEntry{at: st.At, actor: st.Actor, action: ActionAssigneeChanged, data: map[string]any{
				dataAssignee: audit.Change{From: refDataPtr(assignee), To: refDataPtr(st.Assignee)},
			}})
			assignee = st.Assignee
			if next := ticket.StatusAfterAssignment(status, true); next != status {
				entries = append(entries, statusEntry(st.At, st.Actor, status, next, ticket.CauseAssignment))
				status = next
			}
		}
		if st.Status != "" && st.Status != status {
			entries = append(entries, statusEntry(st.At, st.Actor, status, st.Status, ""))
			status = st.Status
			if ticket.MarksResolved(status) {
				at := st.At
				resolvedAt = &at
			}
		}
	}
	var firstResponse *time.Time
	for _, m := range t.Messages {
		if m.FromSupport && m.Visibility == ticket.VisibilityPublic && firstResponse == nil {
			at := m.At
			firstResponse = &at
		}
	}
	snap, err := SnapshotFor(ctx, q, t.ProjectID)
	if err != nil {
		return false, err
	}
	snapJSON, err := json.Marshal(snap)
	if err != nil {
		return false, err
	}
	updatedAt := t.CreatedAt
	if n := len(t.Steps); n > 0 {
		updatedAt = t.Steps[n-1].At
	}
	params := db.InsertSeedTicketParams{
		Number: t.Number, Key: ticket.FormatKey(t.Prefix, t.Number), Type: string(t.Type), Status: string(status),
		Priority: string(t.Priority), Channel: string(t.Channel), ClientID: t.ClientID, RequesterID: t.Requester.ID,
		CreatedBy: t.CreatedBy.ID, ProjectID: t.ProjectID, CatalogItemID: t.CatalogItemID, Subject: t.Subject,
		Description: t.Description, SlaSnapshot: snapJSON, FirstResponseAt: firstResponse, ResolvedAt: resolvedAt,
		CreatedAt: t.CreatedAt, UpdatedAt: updatedAt,
	}
	if assignee != nil {
		params.AssigneeID = &assignee.ID
	}
	id, err := q.InsertSeedTicket(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	created := seedEntry{at: t.CreatedAt, actor: t.CreatedBy, action: ActionCreated, data: map[string]any{
		dataKey: params.Key, dataType: t.Type, dataPriority: t.Priority, dataStatus: ticket.StatusNew,
		dataChannel: t.Channel, dataClient: t.ClientID, dataRequester: t.Requester.ID, dataProject: t.ProjectID,
		dataAttachmentIDs: []uuid.UUID{},
	}}
	entries = append([]seedEntry{created}, entries...)
	responded := false
	for _, m := range t.Messages {
		msgID := uuid.Must(uuid.NewV7())
		if err := q.InsertSeedMessage(ctx, db.InsertSeedMessageParams{
			ID: msgID, TicketID: id, AuthorID: m.Author.ID, Visibility: string(m.Visibility), Body: m.Body, CreatedAt: m.At,
		}); err != nil {
			return false, err
		}
		entries = append(entries, seedEntry{at: m.At, actor: m.Author, action: ActionMessageAdded, data: map[string]any{
			dataMessageID: msgID, dataVisibility: m.Visibility, dataAttachmentIDs: []uuid.UUID{},
		}})
		if m.FromSupport && m.Visibility == ticket.VisibilityPublic && !responded {
			responded = true
			entries = append(entries, seedEntry{at: m.At, actor: m.Author, action: ActionFirstResponded,
				data: map[string]any{dataMessageID: msgID}})
		}
	}
	for _, e := range entries {
		data, err := json.Marshal(e.data)
		if err != nil {
			return false, err
		}
		actorID, actorName, target := e.actor.ID, e.actor.Name, TargetTicket
		if err := q.InsertAuditAt(ctx, db.InsertAuditAtParams{
			At: e.at, ActorKind: audit.ActorUser, ActorUserID: &actorID, ActorName: &actorName,
			Action: e.action, TargetType: &target, TargetID: &id, Data: data,
		}); err != nil {
			return false, err
		}
	}
	return true, nil
}

func statusEntry(at time.Time, actor Ref, from, to ticket.Status, cause ticket.Cause) seedEntry {
	data := map[string]any{dataStatus: audit.Change{From: from, To: to}}
	if cause != "" {
		data[dataCause] = cause
	}
	return seedEntry{at: at, actor: actor, action: ActionStatusChanged, data: data}
}

func refDataPtr(r *Ref) any {
	if r == nil {
		return nil
	}
	return refData(r.ID, r.Name)
}
