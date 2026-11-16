package tickets

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/codeflow"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/projects"
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
	Code          []SeedCode
}

// SeedCode is what a stored code event did to the ticket: the link, its activity-log line and the
// Definition of Done items it met. Status moves it caused are SeedSteps with a code cause.
type SeedCode struct {
	At       time.Time
	Event    CodeEvent
	Outcome  codeflow.Outcome
	Items    []dod.Item
	From, To ticket.Status // set when it moved the ticket
}

// SeedStep is a change after creation: an assignee change, a status change, or both. A move
// with CauseUATApproved is the client approving the test, which meets the client items of the
// project's Definition of Done. A move with a code cause is written by a git actor.
type SeedStep struct {
	At       time.Time
	Actor    Ref
	Assignee *Ref          // assign to this person
	Status   ticket.Status // move to this status
	Cause    ticket.Cause  // why the status moved, when it wasn't by hand
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
	git    bool // code activity: a git actor, the member when ID is set
}

// InsertSeed writes the ticket, its messages and its audit history. It reports false and
// writes nothing when a ticket with the key already exists, so seeding can be repeated.
func InsertSeed(ctx context.Context, q *db.Queries, t SeedTicket) (bool, error) {
	status, assignee := ticket.StatusNew, (*Ref)(nil)
	statusChangedAt := t.CreatedAt
	var resolvedAt *time.Time
	var approvals []SeedStep
	entries := []seedEntry{}
	for _, st := range t.Steps {
		if st.Assignee != nil {
			entries = append(entries, seedEntry{at: st.At, actor: st.Actor, action: ActionAssigneeChanged, data: map[string]any{
				dataAssignee: audit.Change{From: refDataPtr(assignee), To: refDataPtr(st.Assignee)},
			}})
			assignee = st.Assignee
			if next := ticket.StatusAfterAssignment(status, true); next != status {
				entries = append(entries, statusEntry(st.At, st.Actor, status, next, ticket.CauseAssignment))
				status, statusChangedAt = next, st.At
			}
		}
		if st.Status != "" && st.Status != status {
			entries = append(entries, statusEntry(st.At, st.Actor, status, st.Status, st.Cause))
			status, statusChangedAt = st.Status, st.At
			if st.Cause == ticket.CauseUATApproved {
				approvals = append(approvals, st)
			}
			if ticket.MarksResolved(status) {
				at := st.At
				resolvedAt = &at
			}
		}
	}
	// Like Create: an agent-logged ticket is answered when it is logged; a portal report by the
	// first public staff reply.
	loggedByStaff := t.Channel.AgentLoggable()
	var firstResponse *time.Time
	if loggedByStaff {
		at := t.CreatedAt
		firstResponse = &at
	}
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
		StatusChangedAt: statusChangedAt, CreatedAt: t.CreatedAt, UpdatedAt: updatedAt,
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
	responded := loggedByStaff
	if loggedByStaff {
		entries = append(entries, seedEntry{at: t.CreatedAt, actor: t.CreatedBy, action: ActionFirstResponded,
			data: map[string]any{dataCause: causeLoggedByStaff}})
	}
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
	for _, c := range t.Code {
		code, err := seedCode(ctx, q, id, params.Key, c)
		if err != nil {
			return false, err
		}
		entries = append(entries, code...)
	}
	for _, st := range approvals {
		items, err := seedClientItems(ctx, q, id, t.ProjectID, st)
		if err != nil {
			return false, err
		}
		if len(items) > 0 {
			entries = append(entries, seedEntry{at: st.At, actor: st.Actor, action: ActionDoDItemsDone,
				data: map[string]any{dataItems: items}})
		}
	}
	// The activity log reads entries by time, then by id (insertion order): at the same time, the
	// code event comes before the move it caused, as ApplyCode writes them.
	slices.SortStableFunc(entries, func(a, b seedEntry) int {
		if c := a.at.Compare(b.at); c != 0 {
			return c
		}
		return seedRank(a) - seedRank(b)
	})
	for _, e := range entries {
		data, err := json.Marshal(e.data)
		if err != nil {
			return false, err
		}
		kind, actorID, actorName, target := audit.ActorUser, &e.actor.ID, e.actor.Name, TargetTicket
		if e.git {
			kind = audit.ActorGit
			if e.actor.ID == uuid.Nil {
				actorID = nil
			}
		}
		if err := q.InsertAuditAt(ctx, db.InsertAuditAtParams{
			At: e.at, ActorKind: kind, ActorUserID: actorID, ActorName: &actorName,
			Action: e.action, TargetType: &target, TargetID: &id, Data: data,
		}); err != nil {
			return false, err
		}
	}
	return true, nil
}

func seedRank(e seedEntry) int {
	if e.action == ActionCodeLinked {
		return 0
	}
	return 1
}

// seedCode writes what a code event did to the ticket, as ApplyCode would, at its own time.
func seedCode(ctx context.Context, q *db.Queries, ticketID uuid.UUID, key string, c SeedCode) ([]seedEntry, error) {
	items := make([]string, len(c.Items))
	for i, it := range c.Items {
		items[i] = string(it)
		if err := q.InsertSeedDoDItem(ctx, db.InsertSeedDoDItemParams{
			TicketID: ticketID, Item: string(it), DoneAt: c.At, DoneBy: c.Event.AuthorID, CodeEventID: &c.Event.ID,
		}); err != nil {
			return nil, err
		}
	}
	link := db.InsertSeedCodeLinkParams{
		CodeEventID: c.Event.ID, TicketID: &ticketID, TicketKey: key, Outcome: string(c.Outcome), Items: items, At: c.At,
	}
	if c.To != "" {
		from, to := string(c.From), string(c.To)
		link.StatusFrom, link.StatusTo = &from, &to
	}
	if err := q.InsertSeedCodeLink(ctx, link); err != nil {
		return nil, err
	}
	actor := Ref{Name: c.Event.AuthorName}
	if c.Event.AuthorID != nil {
		actor.ID = *c.Event.AuthorID
	}
	e := c.Event
	out := []seedEntry{{at: c.At, actor: actor, git: true, action: ActionCodeLinked, data: map[string]any{
		dataCodeEvent: e.ID, dataProvider: e.Provider, dataCodeKind: e.Kind, dataCodeRef: e.Ref,
		dataBranch: e.Branch, dataTarget: e.Target, dataOutcome: c.Outcome, dataItems: items,
	}}}
	if len(items) > 0 {
		out = append(out, seedEntry{at: c.At, actor: actor, git: true, action: ActionDoDItemsDone,
			data: map[string]any{dataItems: items, dataCodeEvent: e.ID}})
	}
	return out, nil
}

// seedClientItems meets the client items of the project's Definition of Done for an approval.
func seedClientItems(ctx context.Context, q *db.Queries, ticketID, projectID uuid.UUID, st SeedStep) ([]string, error) {
	p, err := projects.Load(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	var items []string
	for _, it := range dod.ClientRequired(p.DoD) {
		if err := q.InsertSeedDoDItem(ctx, db.InsertSeedDoDItemParams{
			TicketID: ticketID, Item: string(it), DoneAt: st.At, DoneBy: &st.Actor.ID,
		}); err != nil {
			return nil, err
		}
		items = append(items, string(it))
	}
	return items, nil
}

func statusEntry(at time.Time, actor Ref, from, to ticket.Status, cause ticket.Cause) seedEntry {
	data := map[string]any{dataStatus: audit.Change{From: from, To: to}}
	if cause != "" {
		data[dataCause] = cause
	}
	return seedEntry{at: at, actor: actor, action: ActionStatusChanged, data: data, git: cause.FromCode()}
}

func refDataPtr(r *Ref) any {
	if r == nil {
		return nil
	}
	return refData(r.ID, r.Name)
}
