package tickets

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// BoardColumnLimit caps the cards in one column; the column still reports its full size.
const BoardColumnLimit = 100

// BoardFilter narrows the board's cards. The stats ignore it.
type BoardFilter struct {
	Assignee AssigneeFilter
	Query    string // a ticket key, or words in the subject or client name
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
}

type BoardColumn struct {
	ID    ticket.Column
	Total int
	Cards []BoardCard
}

type BoardStats struct {
	OpenTickets      int
	ResolvedThisWeek int
}

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
	loc, err := s.location(ctx)
	if err != nil {
		return Board{}, err
	}
	today := calendar.DateOf(s.now().In(loc))
	midnight := today.In(loc)
	weekStart := today.AddDays(-daysSince(today.Weekday(), ticket.BoardWeekStart)).In(loc)

	statuses := ticket.BoardStatuses()
	columns := make([]string, len(statuses))
	for i, st := range statuses {
		col, _ := ticket.ColumnOf(st)
		columns[i] = string(col)
	}
	params := db.ListBoardCardsParams{
		ColumnLimit: BoardColumnLimit, Statuses: statusStrings(statuses), Columns: columns,
		ScopeAgentID: a.scopeAgentID, ResolvedSince: midnight, Unassigned: f.Assignee == AssigneeUnassigned,
	}
	if f.Assignee == AssigneeMe {
		me := p.UserID
		params.AssigneeID = &me
	}
	params.SearchKey, params.SearchPattern = searchParams(f.Query)
	q := s.store.Q()
	rows, err := q.ListBoardCards(ctx, params)
	if err != nil {
		return Board{}, err
	}
	stats, err := q.CountBoardStats(ctx, db.CountBoardStatsParams{
		OpenStatuses: statusStrings(ticket.OpenStatuses), WeekStart: weekStart, ScopeAgentID: a.scopeAgentID,
	})
	if err != nil {
		return Board{}, err
	}

	board := Board{
		Columns:       make([]BoardColumn, len(ticket.Columns)),
		ResolvedSince: midnight,
		Stats:         BoardStats{OpenTickets: int(stats.OpenTickets), ResolvedThisWeek: int(stats.ResolvedThisWeek)},
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
		col.Cards = append(col.Cards, BoardCard{
			ID: r.ID, Key: r.Key, Type: ticket.Type(r.Type), Status: status, Priority: sla.Priority(r.Priority),
			Subject: r.Subject, Client: Ref{r.ClientID, r.ClientName}, Assignee: optionalRef(r.AssigneeID, r.AssigneeName),
			AllowedStatuses: ticket.AllowedMoves(status, r.AssigneeID != nil),
		})
	}
	return board, nil
}

// location is the organisation's time zone, which decides where "today" starts.
func (s *Service) location(ctx context.Context) (*time.Location, error) {
	tz, err := s.store.Q().GetOrganizationTimezone(ctx)
	if err != nil {
		return nil, err
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("organisation time zone: %w", err)
	}
	return loc, nil
}

// daysSince counts the days back from day to the last start (0 when day is start).
func daysSince(day, start time.Weekday) int {
	const daysPerWeek = 7
	return (int(day) - int(start) + daysPerWeek) % daysPerWeek
}
