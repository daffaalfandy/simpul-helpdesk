package tickets

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/slaengine"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// BoardColumnLimit caps the cards in one column; the column still reports its full size.
const BoardColumnLimit = 100

// BoardFilter narrows the board's cards. The stats ignore it.
type BoardFilter struct {
	Assignee  AssigneeFilter
	Query     string // a ticket key, or words in the subject or client name
	SLAAtRisk bool   // only open tickets with a clock at risk or breached
}

// BoardCard is a ticket on the board, with the moves the viewer may make.
type BoardCard struct {
	ID              uuid.UUID
	Key             string
	Type            ticket.Type
	Status          ticket.Status
	Priority        sla.Priority
	Subject         string
	Client          Ref
	Assignee        *Ref
	AllowedStatuses []ticket.Status
	SLA             *CardSLA // nil until the SLA engine has run for the ticket
	// DoDPending is what an open ticket's Definition of Done still needs; resolving before then
	// needs a reason. Empty for done tickets.
	DoDPending []dod.Item
	UAT        *CardUAT  // set while the client tests
	Code       *CardCode // set once code reached the ticket
}

// CardUAT is how far into the client's test a ticket is: day Day of Days working days, or of
// no limit when Days is nil.
type CardUAT struct {
	Day  int
	Days *int
}

// CardSLA is the clock a card shows: an open ticket's first response until it is sent, then
// resolution.
type CardSLA struct {
	Clock slaengine.Clock
	View  slaengine.View
}

type BoardColumn struct {
	ID    ticket.Column
	Total int
	Cards []BoardCard
}

type BoardStats struct {
	OpenTickets      int
	ResolvedThisWeek int
	SLABreached      int // open tickets with an unmet clock past its deadline
	SLAAtRisk        int // open tickets at risk, not yet breached
	SLAOldest        *SLAProblem
	// AvgFirstResponse is the mean first-response time of portal reports answered this month,
	// on their SLA clocks; nil when there were none. Agent-logged tickets are answered at 0.
	AvgFirstResponse *time.Duration
	// WithinSLAPercent is the share of tickets resolved this month that never breached; nil
	// when none were resolved.
	WithinSLAPercent *float64
}

// SLAProblem names the ticket the board's SLA banner points at.
type SLAProblem struct {
	Key        string
	ClientName string
}

// percentScale turns a share into a percentage.
const percentScale = 100

// Board is every column, left to right, even when empty.
type Board struct {
	Columns       []BoardColumn
	ResolvedSince time.Time // local midnight: the resolved column starts here
	Stats         BoardStats
}

// Board returns the agent board for staff in their scope.
func (s *Service) Board(ctx context.Context, p *auth.Principal, f BoardFilter) (Board, error) {
	a, err := staffAccess(p)
	if err != nil {
		return Board{}, err
	}
	loc, err := s.location(ctx, a.org)
	if err != nil {
		return Board{}, err
	}
	today := calendar.DateOf(s.now().In(loc))
	midnight := today.In(loc)
	weekStart := today.AddDays(-daysSince(today.Weekday(), ticket.BoardWeekStart)).In(loc)
	monthStart := calendar.NewDate(today.Year, today.Month, 1).In(loc)

	statuses := ticket.BoardStatuses()
	columns := make([]string, len(statuses))
	for i, st := range statuses {
		col, _ := ticket.ColumnOf(st)
		columns[i] = string(col)
	}
	q := s.store.Q()
	filter, err := projectFilter(ctx, q, a)
	if err != nil {
		return Board{}, err
	}
	params := db.ListBoardCardsParams{
		OrganizationID: a.org, ProjectIds: filter, ColumnLimit: BoardColumnLimit, Statuses: statusStrings(statuses), Columns: columns,
		ScopeAgentID: a.scopeAgentID, ResolvedSince: midnight, Unassigned: f.Assignee == AssigneeUnassigned,
		SlaAtRisk: f.SLAAtRisk, OpenStatuses: statusStrings(ticket.OpenStatuses),
	}
	if f.Assignee == AssigneeMe {
		me := p.UserID
		params.AssigneeID = &me
	}
	params.SearchKey, params.SearchPattern = searchParams(f.Query)
	rows, err := q.ListBoardCards(ctx, params)
	if err != nil {
		return Board{}, err
	}
	stats, err := s.boardStats(ctx, q, a, filter, weekStart, monthStart)
	if err != nil {
		return Board{}, err
	}
	schedules, err := s.sla.Schedules(ctx, a.org)
	if err != nil {
		return Board{}, err
	}
	now := s.now()

	board := Board{
		Columns:       make([]BoardColumn, len(ticket.Columns)),
		ResolvedSince: midnight,
		Stats:         stats,
	}
	index := map[ticket.Column]int{}
	for i, c := range ticket.Columns {
		board.Columns[i] = BoardColumn{ID: c, Cards: []BoardCard{}}
		index[c] = i
	}
	for _, r := range rows {
		i, ok := index[ticket.Column(r.ColumnID)]
		if !ok {
			return Board{}, fmt.Errorf("board: unknown column %q", r.ColumnID)
		}
		col := &board.Columns[i]
		col.Total = int(r.ColumnTotal)
		status := ticket.Status(r.Status)
		pending := []dod.Item{}
		var uat *CardUAT
		if status.IsOpen() {
			sd, err := parseDoD(r.DodConfig, r.DodOverride)
			if err != nil {
				return Board{}, fmt.Errorf("board: %s: %w", r.Key, err)
			}
			pending = pendingFor(sd, r.DodDone)
			if status == ticket.StatusInUAT {
				uat = cardUAT(sd, r.StatusChangedAt, schedules, now)
			}
		}
		col.Cards = append(col.Cards, BoardCard{
			ID: r.ID, Key: r.Key, Type: ticket.Type(r.Type), Status: status, Priority: sla.Priority(r.Priority),
			Subject: r.Subject, Client: Ref{r.ClientID, r.ClientName}, Assignee: optionalRef(r.AssigneeID, r.AssigneeName),
			AllowedStatuses: ticket.AllowedMoves(status, r.AssigneeID != nil),
			SLA:             cardSLA(r, status, schedules, now),
			DoDPending:      pending,
			UAT:             uat,
		})
	}
	return board, s.addCardCodes(ctx, q, &board)
}

// addCardCodes sets the code chip of every card that code reached.
func (s *Service) addCardCodes(ctx context.Context, q *db.Queries, b *Board) error {
	var ids []uuid.UUID
	for _, col := range b.Columns {
		for _, c := range col.Cards {
			ids = append(ids, c.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	codes, err := cardCodes(ctx, q, ids)
	if err != nil {
		return err
	}
	for i := range b.Columns {
		for j := range b.Columns[i].Cards {
			b.Columns[i].Cards[j].Code = codes[b.Columns[i].Cards[j].ID]
		}
	}
	return nil
}

func (s *Service) boardStats(ctx context.Context, q *db.Queries, a access, filter []uuid.UUID, weekStart, monthStart time.Time) (BoardStats, error) {
	open := statusStrings(ticket.OpenStatuses)
	r, err := q.CountBoardStats(ctx, db.CountBoardStatsParams{
		OpenStatuses: open, WeekStart: weekStart, MonthStart: monthStart, PortalChannel: string(ticket.ChannelPortal),
		OrganizationID: a.org, ScopeAgentID: a.scopeAgentID, ProjectIds: filter,
	})
	if err != nil {
		return BoardStats{}, err
	}
	stats := BoardStats{
		OpenTickets: int(r.OpenTickets), ResolvedThisWeek: int(r.ResolvedThisWeek),
		SLABreached: int(r.SlaBreached), SLAAtRisk: int(r.SlaAtRisk),
	}
	if r.FirstResponses > 0 {
		avg := time.Duration(r.FirstResponseMsTotal/r.FirstResponses) * time.Millisecond
		stats.AvgFirstResponse = &avg
	}
	if r.ResolvedThisMonth > 0 {
		pct := float64(r.ResolvedWithinSla) * percentScale / float64(r.ResolvedThisMonth)
		stats.WithinSLAPercent = &pct
	}
	if stats.SLABreached+stats.SLAAtRisk > 0 {
		oldest, err := q.GetOldestSLAProblem(ctx, db.GetOldestSLAProblemParams{OrganizationID: a.org, OpenStatuses: open, ScopeAgentID: a.scopeAgentID, ProjectIds: filter})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return BoardStats{}, err
		}
		if err == nil {
			stats.SLAOldest = &SLAProblem{Key: oldest.Key, ClientName: oldest.ClientName}
		}
	}
	return stats, nil
}

func cardUAT(sd storedDoD, enteredAt time.Time, s sla.Schedules, now time.Time) *CardUAT {
	eff, _ := sd.effective()
	days, ok := dod.AutoApproveDays(eff)
	if !ok {
		return &CardUAT{Day: dod.UATDay(enteredAt, now, s, dod.AutoApproveNever)}
	}
	return &CardUAT{Day: dod.UATDay(enteredAt, now, s, days), Days: &days}
}

// cardSLA picks the clock a card shows and reads it now.
func cardSLA(r db.ListBoardCardsRow, status ticket.Status, s sla.Schedules, now time.Time) *CardSLA {
	if r.SlaTicketID == nil {
		return nil
	}
	row := db.TicketSla{
		FirstResponseBudgetMs: deref(r.FirstResponseBudgetMs), FirstResponseCalendar: deref(r.FirstResponseCalendar),
		FirstResponseElapsedMs: deref(r.FirstResponseElapsedMs), FirstResponseRunningSince: r.FirstResponseRunningSince,
		FirstResponseDueAt: r.FirstResponseDueAt, FirstResponseAtRiskFlaggedAt: r.FirstResponseAtRiskFlaggedAt,
		FirstResponseBreachedAt: r.FirstResponseBreachedAt, FirstResponseMetAt: r.FirstResponseMetAt,
		ResolutionBudgetMs: deref(r.ResolutionBudgetMs), ResolutionCalendar: deref(r.ResolutionCalendar),
		ResolutionElapsedMs: deref(r.ResolutionElapsedMs), ResolutionRunningSince: r.ResolutionRunningSince,
		ResolutionDueAt: r.ResolutionDueAt, ResolutionAtRiskFlaggedAt: r.ResolutionAtRiskFlaggedAt,
		ResolutionBreachedAt: r.ResolutionBreachedAt, ResolutionMetAt: r.ResolutionMetAt,
	}
	fr, res := storedClocks(row)
	if status.IsOpen() && fr.MetAt == nil {
		return &CardSLA{Clock: slaengine.ClockFirstResponse, View: slaengine.ViewAt(fr, s, true, now)}
	}
	return &CardSLA{Clock: slaengine.ClockResolution, View: slaengine.ViewAt(res, s, status.IsOpen(), now)}
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

// location is the organization's time zone, which decides where "today" starts.
func (s *Service) location(ctx context.Context, orgID uuid.UUID) (*time.Location, error) {
	return sla.Location(ctx, s.store.Q(), orgID)
}

// daysSince counts the days back from day to the last start (0 when day is start).
func daysSince(day, start time.Weekday) int {
	const daysPerWeek = 7
	return (int(day) - int(start) + daysPerWeek) % daysPerWeek
}
