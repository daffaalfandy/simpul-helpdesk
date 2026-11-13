// Package clients manages the organisations the vendor supports: contract details, SLA
// plan, PIC, and which agents work them. It is the one place that decides which clients a
// user may see.
package clients

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
)

// Audit actions and target type written by this package.
const (
	ActionCreated       = "client.created"
	ActionUpdated       = "client.updated"
	ActionDeleted       = "client.deleted"
	ActionAgentsUpdated = "client.agents_updated"

	TargetClient = "client"
)

// Audit data keys.
const (
	dataName          = "name"
	dataContractNo    = "contract_no"
	dataContractStart = "contract_start"
	dataContractEnd   = "contract_end"
	dataPicName       = "pic_name"
	dataPicEmail      = "pic_email"
	dataSlaPlan       = "sla_plan_id"
	dataAdded         = "added"
	dataRemoved       = "removed"
)

// Field limits, matching the database CHECK constraints.
const (
	MaxNameLen       = 200
	MaxContractNoLen = 100
	emailAt          = "@"
)

const (
	constraintName     = "clients_name_key"
	constraintPlanFK   = "clients_sla_plan_id_fkey"
	constraintUserFK   = "users_client_id_fkey"
	constraintTicketFK = "tickets_client_id_fkey"
)

// Ref names a related record.
type Ref struct {
	ID   uuid.UUID
	Name string
}

// Client is a supported organisation.
type Client struct {
	ID             uuid.UUID
	Name           string
	ContractNo     *string
	ContractStart  *calendar.Date
	ContractEnd    *calendar.Date
	PicName        *string
	PicEmail       *string
	SlaPlan        Ref
	RequesterCount int
	Agents         []Ref
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Input creates or replaces a client's details.
type Input struct {
	Name          string
	ContractNo    *string
	ContractStart *calendar.Date
	ContractEnd   *calendar.Date
	PicName       *string
	PicEmail      *string
	SlaPlanID     uuid.UUID
}

// Service manages clients.
type Service struct {
	store *store.Store
}

// NewService returns a Service.
func NewService(st *store.Store) *Service { return &Service{store: st} }

// scope returns the agent filter for p: nil when p sees every client.
func scope(p *auth.Principal) *uuid.UUID {
	if auth.Can(p.Role, auth.CapClientsViewAll) {
		return nil
	}
	id := p.UserID
	return &id
}

// List returns the clients p may see, by name.
func (s *Service) List(ctx context.Context, p *auth.Principal) ([]Client, error) {
	q := s.store.Q()
	rows, err := q.ListClients(ctx, scope(p))
	if err != nil {
		return nil, err
	}
	agents, err := agentsByClient(ctx, q, nil)
	if err != nil {
		return nil, err
	}
	out := make([]Client, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromRow(db.GetClientRow(r), agents[r.ID]))
	}
	return out, nil
}

// Get returns one client, or 404 if p may not see it.
func (s *Service) Get(ctx context.Context, p *auth.Principal, id uuid.UUID) (Client, error) {
	return get(ctx, s.store.Q(), id, scope(p))
}

// Create adds a client.
func (s *Service) Create(ctx context.Context, actor audit.Actor, in Input) (Client, error) {
	in, err := normalize(in)
	if err != nil {
		return Client{}, err
	}
	var id uuid.UUID
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		if err := requirePlan(ctx, tx.Q, in.SlaPlanID); err != nil {
			return err
		}
		if id, err = tx.Q.CreateClient(ctx, db.CreateClientParams{
			Name: in.Name, ContractNo: in.ContractNo, ContractStart: in.ContractStart,
			ContractEnd: in.ContractEnd, PicName: in.PicName, PicEmail: in.PicEmail, SlaPlanID: in.SlaPlanID,
		}); err != nil {
			return err
		}
		return audit.Write(ctx, tx.Q, actor.Entry(ActionCreated, TargetClient, &id, map[string]any{
			dataName: in.Name, dataSlaPlan: in.SlaPlanID,
		}))
	})
	if err != nil {
		return Client{}, clientError(err)
	}
	return get(ctx, s.store.Q(), id, nil)
}

// Update replaces a client's details.
func (s *Service) Update(ctx context.Context, actor audit.Actor, id uuid.UUID, in Input) (Client, error) {
	in, err := normalize(in)
	if err != nil {
		return Client{}, err
	}
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		old, err := tx.Q.GetClientForUpdate(ctx, id)
		if err != nil {
			return notFound(err)
		}
		if err := requirePlan(ctx, tx.Q, in.SlaPlanID); err != nil {
			return err
		}
		if err := tx.Q.UpdateClient(ctx, db.UpdateClientParams{
			ID: id, Name: in.Name, ContractNo: in.ContractNo, ContractStart: in.ContractStart,
			ContractEnd: in.ContractEnd, PicName: in.PicName, PicEmail: in.PicEmail, SlaPlanID: in.SlaPlanID,
		}); err != nil {
			return err
		}
		changes := audit.Changes{}
		audit.Track(changes, dataName, old.Name, in.Name)
		audit.TrackPtr(changes, dataContractNo, old.ContractNo, in.ContractNo)
		audit.TrackPtr(changes, dataContractStart, old.ContractStart, in.ContractStart)
		audit.TrackPtr(changes, dataContractEnd, old.ContractEnd, in.ContractEnd)
		audit.TrackPtr(changes, dataPicName, old.PicName, in.PicName)
		audit.TrackPtr(changes, dataPicEmail, old.PicEmail, in.PicEmail)
		audit.Track(changes, dataSlaPlan, old.SlaPlanID, in.SlaPlanID)
		return audit.Write(ctx, tx.Q, actor.Entry(ActionUpdated, TargetClient, &id, changes))
	})
	if err != nil {
		return Client{}, clientError(err)
	}
	return get(ctx, s.store.Q(), id, nil)
}

// Delete removes a client that has no requesters and no tickets.
func (s *Service) Delete(ctx context.Context, actor audit.Actor, id uuid.UUID) error {
	err := s.store.Tx(ctx, func(tx store.Tx) error {
		old, err := tx.Q.GetClientForUpdate(ctx, id)
		if err != nil {
			return notFound(err)
		}
		if n, err := tx.Q.CountClientTickets(ctx, id); err != nil {
			return err
		} else if n > 0 {
			return errHasTickets()
		}
		if n, err := tx.Q.CountClientUsers(ctx, &id); err != nil {
			return err
		} else if n > 0 {
			return errHasRequesters()
		}
		if err := tx.Q.DeleteClient(ctx, id); err != nil {
			return err
		}
		return audit.Write(ctx, tx.Q, actor.Entry(ActionDeleted, TargetClient, &id, map[string]any{dataName: old.Name}))
	})
	return clientError(err)
}

// SetAgents replaces the agents assigned to a client. Every user must be an active agent.
func (s *Service) SetAgents(ctx context.Context, actor audit.Actor, id uuid.UUID, userIDs []uuid.UUID) (Client, error) {
	ids := slices.Clone(userIDs)
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	ids = slices.Compact(ids)
	err := s.store.Tx(ctx, func(tx store.Tx) error {
		if _, err := tx.Q.GetClientForUpdate(ctx, id); err != nil {
			return notFound(err)
		}
		n, err := tx.Q.CountUsersWithRoleAndStatus(ctx, db.CountUsersWithRoleAndStatusParams{
			Ids: ids, Role: string(auth.RoleAgent), Status: string(auth.UserStatusActive),
		})
		if err != nil {
			return err
		}
		if int(n) != len(ids) {
			return apperr.Invalid(apperr.CodeInvalidAgent, "Only active agents can be assigned to a client", "")
		}
		old, err := tx.Q.ListClientAgentIDs(ctx, id)
		if err != nil {
			return err
		}
		if err := tx.Q.DeleteClientAgents(ctx, id); err != nil {
			return err
		}
		if err := tx.Q.InsertClientAgents(ctx, db.InsertClientAgentsParams{ClientID: id, UserIds: ids}); err != nil {
			return err
		}
		added, removed := diff(old, ids)
		if len(added) == 0 && len(removed) == 0 {
			return nil
		}
		return audit.Write(ctx, tx.Q, actor.Entry(ActionAgentsUpdated, TargetClient, &id, map[string]any{
			dataAdded: added, dataRemoved: removed,
		}))
	})
	if err != nil {
		return Client{}, err
	}
	return get(ctx, s.store.Q(), id, nil)
}

func get(ctx context.Context, q *db.Queries, id uuid.UUID, agentID *uuid.UUID) (Client, error) {
	r, err := q.GetClient(ctx, db.GetClientParams{ID: id, AgentID: agentID})
	if err != nil {
		return Client{}, notFound(err)
	}
	agents, err := agentsByClient(ctx, q, &id)
	if err != nil {
		return Client{}, err
	}
	return fromRow(r, agents[id]), nil
}

func agentsByClient(ctx context.Context, q *db.Queries, clientID *uuid.UUID) (map[uuid.UUID][]Ref, error) {
	rows, err := q.ListClientAgents(ctx, clientID)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID][]Ref)
	for _, r := range rows {
		out[r.ClientID] = append(out[r.ClientID], Ref{ID: r.ID, Name: r.Name})
	}
	return out, nil
}

func fromRow(r db.GetClientRow, agents []Ref) Client {
	if agents == nil {
		agents = []Ref{}
	}
	return Client{
		ID: r.ID, Name: r.Name, ContractNo: r.ContractNo, ContractStart: r.ContractStart,
		ContractEnd: r.ContractEnd, PicName: r.PicName, PicEmail: r.PicEmail,
		SlaPlan: Ref{ID: r.SlaPlanID, Name: r.SlaPlanName}, RequesterCount: int(r.RequesterCount),
		Agents: agents, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func requirePlan(ctx context.Context, q *db.Queries, planID uuid.UUID) error {
	ok, err := q.SlaPlanExists(ctx, planID)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.Invalid(apperr.CodeSlaPlanNotFound, "Choose an SLA plan", "")
	}
	return nil
}

// normalize trims text, turns blanks into "not set" and checks the fields.
func normalize(in Input) (Input, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.ContractNo = trimmed(in.ContractNo)
	in.PicName = trimmed(in.PicName)
	if e := trimmed(in.PicEmail); e != nil {
		lower := strings.ToLower(*e)
		in.PicEmail = &lower
	} else {
		in.PicEmail = nil
	}
	switch {
	case in.Name == "" || len([]rune(in.Name)) > MaxNameLen:
		return in, apperr.Invalid(apperr.CodeInvalidName, "Enter the client's name", "")
	case in.ContractNo != nil && len([]rune(*in.ContractNo)) > MaxContractNoLen:
		return in, apperr.Invalid(apperr.CodeInvalidRequest, "The contract number is too long", "")
	case in.PicName != nil && len([]rune(*in.PicName)) > MaxNameLen:
		return in, apperr.Invalid(apperr.CodeInvalidName, "The PIC name is too long", "")
	case in.PicEmail != nil && strings.Index(*in.PicEmail, emailAt) < 1:
		return in, apperr.Invalid(apperr.CodeInvalidEmail, "Enter a valid PIC email address", "")
	case in.ContractStart != nil && in.ContractEnd != nil && in.ContractEnd.Before(*in.ContractStart):
		return in, apperr.Invalid(apperr.CodeInvalidContractPeriod, "The contract can't end before it starts", "")
	}
	return in, nil
}

func trimmed(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

// diff returns the IDs only in next (added) and only in prev (removed).
func diff(prev, next []uuid.UUID) (added, removed []uuid.UUID) {
	added, removed = []uuid.UUID{}, []uuid.UUID{}
	for _, id := range next {
		if !slices.Contains(prev, id) {
			added = append(added, id)
		}
	}
	for _, id := range prev {
		if !slices.Contains(next, id) {
			removed = append(removed, id)
		}
	}
	return added, removed
}

func clientError(err error) error {
	switch {
	case err == nil:
		return nil
	case store.IsViolation(err, store.SQLStateUniqueViolation, constraintName):
		return apperr.Conflict(apperr.CodeClientNameTaken, "A client with this name already exists", "")
	case store.IsViolation(err, store.SQLStateForeignKeyViolation, constraintPlanFK):
		return apperr.Invalid(apperr.CodeSlaPlanNotFound, "Choose an SLA plan", "")
	case store.IsViolation(err, store.SQLStateRestrictViolation, constraintTicketFK):
		return errHasTickets()
	case store.IsViolation(err, store.SQLStateRestrictViolation, constraintUserFK):
		return errHasRequesters()
	}
	return err
}

func errHasRequesters() error {
	return apperr.Conflict(apperr.CodeClientInUse, "This client still has requesters", "Remove or move them first.")
}

func errHasTickets() error {
	return apperr.Conflict(apperr.CodeClientInUse, "This client has tickets", "Clients with tickets are kept for the record.")
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound()
	}
	return err
}
