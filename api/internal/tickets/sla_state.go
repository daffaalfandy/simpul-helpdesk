package tickets

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/slaengine"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// slaActions are the audit actions the clocks replay.
var slaActions = []string{ActionCreated, ActionStatusChanged, ActionFirstResponded}

// clockActions are the changes after which the clocks need a replay: the replayed ones, and a
// priority change, which swaps the targets.
var clockActions = append([]string{ActionPriorityChanged}, slaActions...)

// causeLoggedByStaff marks the first response an agent-logged ticket gets at creation: the
// agent already spoke with the requester (owner, 15 Nov).
const causeLoggedByStaff = "logged_by_staff"

// replaySLA recomputes the ticket's clocks from its history and stores them, in the caller's
// transaction. The scan's flags carry over, except an at-risk flag a change has moved back into
// the future. Flags it sets itself are silent (no alert): a clock that stopped past its
// deadline before the scan saw it, and a ticket replayed for the first time (backfill).
func (s *Service) replaySLA(ctx context.Context, q *db.Queries, ticketID uuid.UUID, now time.Time) error {
	schedules, err := sla.LoadSchedules(ctx, q)
	if err != nil {
		return err
	}
	return ReplaySLA(ctx, q, schedules, ticketID, now)
}

// ReplaySLA is replaySLA with the calendar already loaded, for the scan and the recompute job.
func ReplaySLA(ctx context.Context, q *db.Queries, schedules sla.Schedules, ticketID uuid.UUID, now time.Time) error {
	t, err := q.GetTicketForSLA(ctx, ticketID)
	if err != nil {
		return err
	}
	snap, err := parseSnapshot(t.SlaSnapshot)
	if err != nil {
		return err
	}
	rows, err := q.ListSLAHistory(ctx, db.ListSLAHistoryParams{TargetType: &targetTicket, TargetID: &ticketID, Actions: slaActions})
	if err != nil {
		return err
	}
	state, err := slaengine.Replay(slaengine.Input{
		Snapshot: snap, Type: ticket.Type(t.Type), Events: slaEvents(rows), Schedules: schedules,
		Facts: slaengine.Facts{
			Status: ticket.Status(t.Status), Priority: sla.Priority(t.Priority),
			FirstResponseAt: t.FirstResponseAt, ResolvedAt: t.ResolvedAt,
		},
	})
	if err != nil {
		return err
	}
	prev, err := q.GetTicketSLA(ctx, ticketID)
	backfill := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !backfill {
		return err
	}
	fr := clockColumns(state.FirstResponse, prev.FirstResponseAtRiskFlaggedAt, prev.FirstResponseBreachedAt, backfill, now)
	res := clockColumns(state.Resolution, prev.ResolutionAtRiskFlaggedAt, prev.ResolutionBreachedAt, backfill, now)
	return q.UpsertTicketSLA(ctx, db.UpsertTicketSLAParams{
		TicketID: ticketID, ComputedAt: now,
		FirstResponseBudgetMs: fr.budgetMs, FirstResponseCalendar: fr.calendar, FirstResponseElapsedMs: fr.elapsedMs,
		FirstResponseRunningSince: fr.runningSince, FirstResponseDueAt: fr.dueAt, FirstResponseAtRiskAt: fr.atRiskAt,
		FirstResponseAtRiskFlaggedAt: fr.atRiskFlagged, FirstResponseBreachedAt: fr.breached, FirstResponseMetAt: fr.metAt,
		ResolutionBudgetMs: res.budgetMs, ResolutionCalendar: res.calendar, ResolutionElapsedMs: res.elapsedMs,
		ResolutionRunningSince: res.runningSince, ResolutionDueAt: res.dueAt, ResolutionAtRiskAt: res.atRiskAt,
		ResolutionAtRiskFlaggedAt: res.atRiskFlagged, ResolutionBreachedAt: res.breached, ResolutionMetAt: res.metAt,
	})
}

// storedClock is one clock's columns.
type storedClock struct {
	budgetMs, elapsedMs                  int64
	calendar                             string
	runningSince, dueAt, atRiskAt, metAt *time.Time
	atRiskFlagged, breached              *time.Time
}

func clockColumns(c slaengine.ClockState, atRiskFlagged, breached *time.Time, backfill bool, now time.Time) storedClock {
	col := storedClock{
		budgetMs: c.Target.Budget.Milliseconds(), elapsedMs: c.Elapsed.Milliseconds(),
		calendar: string(c.Target.Calendar), runningSince: c.RunningSince, metAt: c.MetAt,
		atRiskFlagged: atRiskFlagged, breached: breached,
	}
	if c.Running() {
		col.dueAt, col.atRiskAt = c.DueAt, c.AtRiskAt
		if col.atRiskFlagged != nil && c.AtRiskAt != nil && c.AtRiskAt.After(now) {
			col.atRiskFlagged = nil
		}
	}
	end := now
	if c.MetAt != nil {
		end = *c.MetAt
	}
	// Breach is strictly after the due time (calendar: a due time on closing time stays there).
	if col.breached == nil && c.DueAt != nil && (backfill || !c.Running()) && c.DueAt.Before(end) {
		col.breached = c.DueAt
	}
	if backfill && col.atRiskFlagged == nil && c.Running() && c.AtRiskAt != nil && !c.AtRiskAt.After(now) {
		col.atRiskFlagged = c.AtRiskAt
	}
	return col
}

// slaEvents turns the clock-moving audit entries into engine events.
func slaEvents(rows []db.ListSLAHistoryRow) []slaengine.Event {
	out := make([]slaengine.Event, 0, len(rows))
	for _, r := range rows {
		data := map[string]json.RawMessage{}
		_ = json.Unmarshal(r.Data, &data)
		switch r.Action {
		case ActionCreated:
			status := ticket.StatusNew
			if st := decode[ticket.Status](data, dataStatus); st != nil {
				status = *st
			}
			out = append(out, slaengine.Event{At: r.At, Kind: slaengine.EventCreated, Status: status})
		case ActionStatusChanged:
			if st := decode[change[ticket.Status]](data, dataStatus); st != nil {
				out = append(out, slaengine.Event{At: r.At, Kind: slaengine.EventStatus, Status: st.To})
			}
		case ActionFirstResponded:
			out = append(out, slaengine.Event{At: r.At, Kind: slaengine.EventFirstResponse})
		}
	}
	return out
}

// SLAClocks are a ticket's two clocks as of a read.
type SLAClocks struct {
	FirstResponse slaengine.View
	Resolution    slaengine.View
}

// storedClocks reads a ticket_sla row into the engine's stored form.
func storedClocks(r db.TicketSla) (fr, res slaengine.Stored) {
	fr = slaengine.Stored{
		Budget: time.Duration(r.FirstResponseBudgetMs) * time.Millisecond, Calendar: sla.CalendarKind(r.FirstResponseCalendar),
		Elapsed: time.Duration(r.FirstResponseElapsedMs) * time.Millisecond, RunningSince: r.FirstResponseRunningSince,
		MetAt: r.FirstResponseMetAt, DueAt: r.FirstResponseDueAt,
		AtRisk: r.FirstResponseAtRiskFlaggedAt != nil, Breached: r.FirstResponseBreachedAt != nil,
	}
	res = slaengine.Stored{
		Budget: time.Duration(r.ResolutionBudgetMs) * time.Millisecond, Calendar: sla.CalendarKind(r.ResolutionCalendar),
		Elapsed: time.Duration(r.ResolutionElapsedMs) * time.Millisecond, RunningSince: r.ResolutionRunningSince,
		MetAt: r.ResolutionMetAt, DueAt: r.ResolutionDueAt,
		AtRisk: r.ResolutionAtRiskFlaggedAt != nil, Breached: r.ResolutionBreachedAt != nil,
	}
	return fr, res
}

// clocksFor reads a ticket's clocks now; nil until the ticket has been replayed.
func (s *Service) clocksFor(ctx context.Context, q *db.Queries, ticketID uuid.UUID, status ticket.Status) (*SLAClocks, error) {
	row, err := q.GetTicketSLA(ctx, ticketID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	schedules, err := s.sla.Schedules(ctx)
	if err != nil {
		return nil, err
	}
	now := s.now()
	fr, res := storedClocks(row)
	return &SLAClocks{
		FirstResponse: slaengine.ViewAt(fr, schedules, status.IsOpen(), now),
		Resolution:    slaengine.ViewAt(res, schedules, status.IsOpen(), now),
	}, nil
}
