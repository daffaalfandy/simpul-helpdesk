package tickets

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// AssigneeFilter narrows a list by assignee.
type AssigneeFilter string

const (
	AssigneeAny        AssigneeFilter = ""
	AssigneeMe         AssigneeFilter = "me"
	AssigneeUnassigned AssigneeFilter = "unassigned"
)

// ILIKE pattern pieces for subject search.
const (
	likeAny    = "%"
	likeOne    = "_"
	likeEscape = `\`
)

var likeEscaper = strings.NewReplacer(likeEscape, likeEscape+likeEscape, likeAny, likeEscape+likeAny, likeOne, likeEscape+likeOne)

// targetTicket is TargetTicket as a query parameter.
var targetTicket = TargetTicket

// ListFilter narrows and pages a ticket list.
type ListFilter struct {
	Statuses []ticket.Status // empty means every status
	ClientID *uuid.UUID
	Assignee AssigneeFilter
	Query    string     // a ticket key, or words in the subject
	Before   *uuid.UUID // the last ID of the previous page
	Limit    int
}

// List returns the tickets p may see, newest first.
func (s *Service) List(ctx context.Context, p *auth.Principal, f ListFilter) ([]Summary, error) {
	a, err := accessFor(p)
	if err != nil {
		return nil, err
	}
	statuses := f.Statuses
	if len(statuses) == 0 {
		statuses = ticket.Statuses
	}
	params := db.ListTicketsParams{
		ScopeClientID: a.scopeClientID, ScopeAgentID: a.scopeAgentID, Statuses: statusStrings(statuses),
		ClientID: f.ClientID, Unassigned: f.Assignee == AssigneeUnassigned, Before: f.Before,
		RowLimit: int32(pageSize(f.Limit)),
	}
	if f.Assignee == AssigneeMe {
		me := p.UserID
		params.AssigneeID = &me
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		if len([]rune(q)) > MaxSearchLen {
			q = string([]rune(q)[:MaxSearchLen])
		}
		key := ticket.NormalizeKey(q)
		pattern := likeAny + likeEscaper.Replace(q) + likeAny
		params.SearchKey, params.SearchPattern = &key, &pattern
	}
	rows, err := s.store.Q().ListTickets(ctx, params)
	if err != nil {
		return nil, err
	}
	out := make([]Summary, 0, len(rows))
	for _, r := range rows {
		out = append(out, Summary{
			ID: r.ID, Key: r.Key, Type: ticket.Type(r.Type), Status: ticket.Status(r.Status),
			Priority: sla.Priority(r.Priority), Subject: r.Subject,
			Client: Ref{r.ClientID, r.ClientName}, Requester: Ref{r.RequesterID, r.RequesterName},
			Assignee: optionalRef(r.AssigneeID, r.AssigneeName), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			ResolvedAt: r.ResolvedAt,
		})
	}
	return out, nil
}

func pageSize(n int) int {
	switch {
	case n <= 0:
		return DefaultPageSize
	case n > MaxPageSize:
		return MaxPageSize
	}
	return n
}

// Get returns one ticket, or 404 if p may not see it.
func (s *Service) Get(ctx context.Context, p *auth.Principal, key string) (Ticket, error) {
	a, err := accessFor(p)
	if err != nil {
		return Ticket{}, err
	}
	q := s.store.Q()
	r, err := s.getRow(ctx, q, a, key)
	if err != nil {
		return Ticket{}, err
	}
	snap, err := parseSnapshot(r.SlaSnapshot)
	if err != nil {
		return Ticket{}, err
	}
	t := Ticket{
		Summary: Summary{
			ID: r.ID, Key: r.Key, Type: ticket.Type(r.Type), Status: ticket.Status(r.Status),
			Priority: sla.Priority(r.Priority), Subject: r.Subject,
			Client: Ref{r.ClientID, r.ClientName}, Requester: Ref{r.RequesterID, r.RequesterName},
			Assignee: optionalRef(r.AssigneeID, r.AssigneeName), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			ResolvedAt: r.ResolvedAt,
		},
		Channel: ticket.Channel(r.Channel), Description: r.Description, RequesterJobTitle: r.RequesterJobTitle,
		Project: Ref{r.ProjectID, r.ProjectName}, CatalogItem: optionalRef(r.CatalogItemID, r.CatalogItemName),
		CreatedBy: Ref{r.CreatedBy, r.CreatedByName}, LoggedByStaff: auth.Role(r.CreatedByRole) != auth.RoleRequester,
		SLATargets: snap.Targets, FirstResponseAt: r.FirstResponseAt, ClosedAt: r.ClosedAt,
		AllowedStatuses: []ticket.Status{},
	}
	if t.SLA, err = snap.Effective(t.Priority, t.Type); err != nil {
		return Ticket{}, err
	}
	files, err := q.ListTicketAttachments(ctx, db.ListTicketAttachmentsParams{TicketID: &r.ID, Visibility: a.visibility})
	if err != nil {
		return Ticket{}, err
	}
	t.Attachments = []Attachment{}
	for _, f := range files {
		if f.MessageID == nil {
			t.Attachments = append(t.Attachments, attachmentFromRow(f))
		}
	}
	history, err := s.statusHistory(ctx, q, r.ID)
	if err != nil {
		return Ticket{}, err
	}
	t.Progress = ticket.Progress(t.Status, t.CreatedAt, history)
	if a.staff {
		t.AllowedStatuses = ticket.AllowedMoves(t.Status, t.Assignee != nil)
	}
	return t, nil
}

func (s *Service) getRow(ctx context.Context, q *db.Queries, a access, key string) (db.GetTicketRow, error) {
	key = ticket.NormalizeKey(key)
	if !ticket.ValidKey(key) {
		return db.GetTicketRow{}, notFound(pgx.ErrNoRows)
	}
	r, err := q.GetTicket(ctx, db.GetTicketParams{Key: key, ScopeClientID: a.scopeClientID, ScopeAgentID: a.scopeAgentID})
	return r, notFound(err)
}

// statusHistory reads the ticket's status changes from its audit entries: the status it was
// created with, then each change.
func (s *Service) statusHistory(ctx context.Context, q *db.Queries, ticketID uuid.UUID) ([]ticket.StatusChange, error) {
	rows, err := q.ListAuditByTarget(ctx, db.ListAuditByTargetParams{TargetType: &targetTicket, TargetID: &ticketID})
	if err != nil {
		return nil, err
	}
	out := []ticket.StatusChange{}
	for _, r := range rows {
		data := map[string]json.RawMessage{}
		_ = json.Unmarshal(r.Data, &data)
		switch r.Action {
		case ActionCreated:
			if st := decode[ticket.Status](data, dataStatus); st != nil {
				out = append(out, ticket.StatusChange{To: *st, At: r.At})
			}
		case ActionStatusChanged:
			if c := decode[change[ticket.Status]](data, dataStatus); c != nil {
				out = append(out, ticket.StatusChange{To: c.To, At: r.At})
			}
		}
	}
	return out, nil
}

// Messages returns the ticket's conversation, oldest first. Requesters get public messages
// only, and only files on those.
func (s *Service) Messages(ctx context.Context, p *auth.Principal, key string) ([]Message, error) {
	a, err := accessFor(p)
	if err != nil {
		return nil, err
	}
	q := s.store.Q()
	r, err := s.getRow(ctx, q, a, key)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListTicketMessages(ctx, db.ListTicketMessagesParams{TicketID: r.ID, Visibility: a.visibility})
	if err != nil {
		return nil, err
	}
	files, err := q.ListTicketAttachments(ctx, db.ListTicketAttachmentsParams{TicketID: &r.ID, Visibility: a.visibility})
	if err != nil {
		return nil, err
	}
	byMessage := map[uuid.UUID][]Attachment{}
	for _, f := range files {
		if f.MessageID != nil {
			byMessage[*f.MessageID] = append(byMessage[*f.MessageID], attachmentFromRow(f))
		}
	}
	out := make([]Message, 0, len(rows))
	for _, m := range rows {
		attachments := byMessage[m.ID]
		if attachments == nil {
			attachments = []Attachment{}
		}
		out = append(out, Message{
			ID: m.ID, Author: Ref{m.AuthorID, m.AuthorName}, AuthorJobTitle: m.AuthorJobTitle,
			FromSupport: auth.Role(m.AuthorRole) != auth.RoleRequester, Visibility: ticket.Visibility(m.Visibility),
			Body: m.Body, Attachments: attachments, CreatedAt: m.CreatedAt,
		})
	}
	return out, nil
}

func attachmentFromRow(r db.ListTicketAttachmentsRow) Attachment {
	return Attachment{ID: r.ID, Filename: r.Filename, ContentType: r.ContentType, SizeBytes: r.SizeBytes, CreatedAt: r.CreatedAt}
}

func optionalRef(id *uuid.UUID, name *string) *Ref {
	if id == nil || name == nil {
		return nil
	}
	return &Ref{*id, *name}
}

func statusStrings(statuses []ticket.Status) []string {
	out := make([]string, len(statuses))
	for i, st := range statuses {
		out[i] = string(st)
	}
	return out
}

// change is the stored form of audit.Change, typed for reading back.
type change[T any] struct {
	From T `json:"from"`
	To   T `json:"to"`
}

// decode reads one audit data field; nil if missing or malformed.
func decode[T any](data map[string]json.RawMessage, key string) *T {
	raw, ok := data[key]
	if !ok {
		return nil
	}
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	return &v
}
