package httpapi

import (
	"context"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

// signedIn returns the principal for operations any role may call; the tickets service
// scopes what each role sees.
func signedIn(ctx context.Context) (*auth.Principal, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, apperr.Unauthenticated()
	}
	return p, nil
}

func (s *server) GetIntake(ctx context.Context, req gen.GetIntakeRequestObject) (gen.GetIntakeResponseObject, error) {
	p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	in, err := s.tickets.Intake(ctx, p, req.Params.ClientId)
	if err != nil {
		return nil, err
	}
	out := gen.GetIntake200JSONResponse{
		ClientId: in.ClientID, Targets: apiTargets(in.Targets), BusinessHours: apiHours(in.Hours),
		Systems: make([]gen.IntakeSystem, len(in.Systems)), CatalogItems: make([]gen.IntakeCatalogItem, len(in.CatalogItems)),
	}
	for i, sys := range in.Systems {
		out.Systems[i] = gen.IntakeSystem{Id: sys.ID, Name: sys.Name, Description: sys.Description}
	}
	for i, it := range in.CatalogItems {
		out.CatalogItems[i] = gen.IntakeCatalogItem{
			Id: it.ID, Name: it.Name, Description: it.Description, DeliveryMinutes: it.DeliveryMinutes,
		}
	}
	return out, nil
}

func (s *server) ListTickets(ctx context.Context, req gen.ListTicketsRequestObject) (gen.ListTicketsResponseObject, error) {
	p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	f := tickets.ListFilter{ClientID: req.Params.ClientId, Before: req.Params.Before}
	if req.Params.Status != nil {
		for _, st := range *req.Params.Status {
			f.Statuses = append(f.Statuses, ticket.Status(st))
		}
	}
	if req.Params.Assignee != nil {
		f.Assignee = tickets.AssigneeFilter(*req.Params.Assignee)
	}
	if req.Params.Q != nil {
		f.Query = *req.Params.Q
	}
	if req.Params.Limit != nil {
		f.Limit = *req.Params.Limit
	}
	list, err := s.tickets.List(ctx, p, f)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListTickets200JSONResponse, len(list))
	for i, t := range list {
		out[i] = apiSummary(t)
	}
	return out, nil
}

func (s *server) CreateTicket(ctx context.Context, req gen.CreateTicketRequestObject) (gen.CreateTicketResponseObject, error) {
	p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	b := req.Body
	in := tickets.CreateInput{
		Type: ticket.Type(b.Type), Priority: sla.Priority(b.Priority), Subject: b.Subject, Description: b.Description,
		SystemID: b.SystemId, CatalogItemID: b.CatalogItemId, ClientID: b.ClientId, RequesterID: b.RequesterId,
	}
	if b.AttachmentIds != nil {
		in.AttachmentIDs = *b.AttachmentIds
	}
	if b.Channel != nil {
		in.Channel = ticket.Channel(*b.Channel)
	}
	if b.AssignToMe != nil {
		in.AssignToMe = *b.AssignToMe
	}
	t, err := s.tickets.Create(ctx, p, in)
	if err != nil {
		return nil, err
	}
	return gen.CreateTicket201JSONResponse(apiTicket(t)), nil
}

func (s *server) GetTicket(ctx context.Context, req gen.GetTicketRequestObject) (gen.GetTicketResponseObject, error) {
	p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	t, err := s.tickets.Get(ctx, p, req.TicketKey)
	if err != nil {
		return nil, err
	}
	return gen.GetTicket200JSONResponse(apiTicket(t)), nil
}

func (s *server) ListTicketMessages(ctx context.Context, req gen.ListTicketMessagesRequestObject) (gen.ListTicketMessagesResponseObject, error) {
	p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	list, err := s.tickets.Messages(ctx, p, req.TicketKey)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListTicketMessages200JSONResponse, len(list))
	for i, m := range list {
		out[i] = apiMessage(m)
	}
	return out, nil
}

func (s *server) CreateTicketMessage(ctx context.Context, req gen.CreateTicketMessageRequestObject) (gen.CreateTicketMessageResponseObject, error) {
	p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	in := tickets.MessageInput{Visibility: ticket.Visibility(req.Body.Visibility), Body: req.Body.Body}
	if req.Body.AttachmentIds != nil {
		in.AttachmentIDs = *req.Body.AttachmentIds
	}
	m, err := s.tickets.AddMessage(ctx, p, req.TicketKey, in)
	if err != nil {
		return nil, err
	}
	return gen.CreateTicketMessage201JSONResponse(apiMessage(m)), nil
}

func (s *server) SetTicketStatus(ctx context.Context, req gen.SetTicketStatusRequestObject) (gen.SetTicketStatusResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapAgentApp)
	if err != nil {
		return nil, err
	}
	t, err := s.tickets.SetStatus(ctx, p, req.TicketKey, ticket.Status(req.Body.Status))
	if err != nil {
		return nil, err
	}
	return gen.SetTicketStatus200JSONResponse(apiTicket(t)), nil
}

func (s *server) SetTicketAssignee(ctx context.Context, req gen.SetTicketAssigneeRequestObject) (gen.SetTicketAssigneeResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapAgentApp)
	if err != nil {
		return nil, err
	}
	t, err := s.tickets.SetAssignee(ctx, p, req.TicketKey, req.Body.AssigneeId)
	if err != nil {
		return nil, err
	}
	return gen.SetTicketAssignee200JSONResponse(apiTicket(t)), nil
}

func (s *server) SetTicketPriority(ctx context.Context, req gen.SetTicketPriorityRequestObject) (gen.SetTicketPriorityResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapAgentApp)
	if err != nil {
		return nil, err
	}
	t, err := s.tickets.SetPriority(ctx, p, req.TicketKey, sla.Priority(req.Body.Priority), req.Body.Reason)
	if err != nil {
		return nil, err
	}
	return gen.SetTicketPriority200JSONResponse(apiTicket(t)), nil
}

func (s *server) ListTicketActivity(ctx context.Context, req gen.ListTicketActivityRequestObject) (gen.ListTicketActivityResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapAgentApp)
	if err != nil {
		return nil, err
	}
	list, err := s.tickets.Activity(ctx, p, req.TicketKey)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListTicketActivity200JSONResponse, len(list))
	for i, e := range list {
		a := gen.TicketActivity{
			Id: e.ID, At: e.At, ActorName: e.ActorName, Action: gen.TicketActivityAction(e.Action),
			From: e.From, To: e.To, Reason: e.Reason,
		}
		if e.Cause != nil {
			c := gen.TicketStatusCause(*e.Cause)
			a.Cause = &c
		}
		if e.Channel != nil {
			c := gen.TicketChannel(*e.Channel)
			a.Channel = &c
		}
		out[i] = a
	}
	return out, nil
}

func apiRef(r tickets.Ref) gen.Ref { return gen.Ref{Id: r.ID, Name: r.Name} }

func apiRefPtr(r *tickets.Ref) *gen.Ref {
	if r == nil {
		return nil
	}
	v := apiRef(*r)
	return &v
}

func apiSummary(t tickets.Summary) gen.TicketSummary {
	return gen.TicketSummary{
		Id: t.ID, Key: t.Key, Type: gen.TicketType(t.Type), Status: gen.TicketStatus(t.Status),
		Priority: gen.Priority(t.Priority), Subject: t.Subject, Client: apiRef(t.Client),
		Requester: apiRef(t.Requester), Assignee: apiRefPtr(t.Assignee), CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt, ResolvedAt: t.ResolvedAt,
	}
}

func apiTicket(t tickets.Ticket) gen.Ticket {
	out := gen.Ticket{
		Id: t.ID, Key: t.Key, Type: gen.TicketType(t.Type), Status: gen.TicketStatus(t.Status),
		Priority: gen.Priority(t.Priority), Subject: t.Subject, Client: apiRef(t.Client),
		Requester: apiRef(t.Requester), Assignee: apiRefPtr(t.Assignee), CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt, ResolvedAt: t.ResolvedAt, Channel: gen.TicketChannel(t.Channel),
		Description: t.Description, RequesterJobTitle: t.RequesterJobTitle, System: apiRefPtr(t.System),
		CatalogItem: apiRefPtr(t.CatalogItem), CreatedBy: apiRef(t.CreatedBy), LoggedByStaff: t.LoggedByStaff,
		Attachments: apiAttachments(t.Attachments), SlaTargets: apiTargets(t.SLATargets),
		FirstResponseAt: t.FirstResponseAt, ClosedAt: t.ClosedAt,
		Sla: gen.TicketSla{
			Priority: gen.Priority(t.SLA.Priority), FirstResponseMinutes: t.SLA.FirstResponseMinutes,
			ResolutionMinutes: t.SLA.ResolutionMinutes, Calendar: gen.SlaCalendar(t.SLA.Calendar),
			FromCatalog: t.SLA.FromCatalog,
		},
		Progress:        make([]gen.TicketProgress, len(t.Progress)),
		AllowedStatuses: make([]gen.TicketStatus, len(t.AllowedStatuses)),
	}
	for i, st := range t.Progress {
		out.Progress[i] = gen.TicketProgress{
			Step: gen.TicketProgressStep(st.Step), State: gen.TicketProgressState(st.State), At: st.At,
		}
	}
	for i, st := range t.AllowedStatuses {
		out.AllowedStatuses[i] = gen.TicketStatus(st)
	}
	return out
}

func apiMessage(m tickets.Message) gen.TicketMessage {
	return gen.TicketMessage{
		Id: m.ID, Author: apiRef(m.Author), AuthorJobTitle: m.AuthorJobTitle, FromSupport: m.FromSupport,
		Visibility: gen.MessageVisibility(m.Visibility), Body: m.Body, Attachments: apiAttachments(m.Attachments),
		CreatedAt: m.CreatedAt,
	}
}

func apiAttachments(list []tickets.Attachment) []gen.Attachment {
	out := make([]gen.Attachment, len(list))
	for i, a := range list {
		out[i] = apiAttachment(a)
	}
	return out
}

func apiAttachment(a tickets.Attachment) gen.Attachment {
	return gen.Attachment{
		Id: a.ID, Filename: a.Filename, ContentType: a.ContentType, SizeBytes: a.SizeBytes, CreatedAt: a.CreatedAt,
	}
}
