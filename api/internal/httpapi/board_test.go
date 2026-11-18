package httpapi_test

import (
	"context"
	"net/http"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

func (s session) board(t *testing.T, query string) gen.Board {
	t.Helper()
	return decode[gen.Board](t, s.do(http.MethodGet, "/board"+query, nil), http.StatusOK)
}

// column returns the keys in one column, top to bottom.
func column(b gen.Board, id gen.BoardColumnId) []string {
	for _, c := range b.Columns {
		if c.Id == id {
			out := []string{}
			for _, card := range c.Cards {
				out = append(out, card.Key)
			}
			return out
		}
	}
	return nil
}

func card(b gen.Board, key string) gen.BoardCard {
	for _, c := range b.Columns {
		for _, card := range c.Cards {
			if card.Key == key {
				return card
			}
		}
	}
	return gen.BoardCard{}
}

func (te *ticketEnv) move(t *testing.T, key string, to gen.TicketStatus) {
	t.Helper()
	decode[gen.Ticket](t, te.admin.do(http.MethodPut, "/tickets/"+key+"/status", gen.TicketStatusInput{Status: to}), http.StatusOK)
}

func (te *ticketEnv) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := te.st.Pool().Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func TestBoard(t *testing.T) {
	te := newTicketEnv(t)
	open := func(subject string, p gen.Priority) gen.Ticket {
		in := incident(te.projectA, subject)
		in.Priority = p
		return te.requester.report(t, in)
	}
	fresh := open("Printer offline", gen.PriorityP3)
	owned := open("Scanner jams", gen.PriorityP3)
	decode[gen.Ticket](t, te.admin.do(http.MethodPut, "/tickets/"+owned.Key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &te.agentID}), http.StatusOK)
	slow := open("VPN slow", gen.PriorityP3)
	te.move(t, slow.Key, gen.TicketStatusInProgress)
	urgent := open("e-Office down", gen.PriorityP1)
	te.move(t, urgent.Key, gen.TicketStatusInProgress)
	later := open("Mail quota", gen.PriorityP3)
	te.move(t, later.Key, gen.TicketStatusInProgress)
	waiting := open("Need a screenshot", gen.PriorityP2)
	te.move(t, waiting.Key, gen.TicketStatusWaitingOnClient)
	inUAT := open("Fix deployed", gen.PriorityP2)
	te.move(t, inUAT.Key, gen.TicketStatusInProgress)
	te.move(t, inUAT.Key, gen.TicketStatusInUat)
	today, yesterday, lastWeek := open("Fixed today", gen.PriorityP4), open("Fixed yesterday", gen.PriorityP4), open("Closed", gen.PriorityP4)
	for _, k := range []string{today.Key, yesterday.Key, lastWeek.Key} {
		te.move(t, k, gen.TicketStatusResolved)
	}
	other := te.otherRequester.report(t, incident(te.projectB, "Batam router"))

	// "Resolved this week" counts from Monday's local midnight in the organisation's time zone.
	jakarta, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(jakarta)
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, jakarta)
	weekStart := midnight.AddDate(0, 0, -int((midnight.Weekday()-ticket.BoardWeekStart+7)%7))
	te.exec(t, `UPDATE tickets SET resolved_at = $2 WHERE key = $1`, today.Key, midnight)
	te.exec(t, `UPDATE tickets SET resolved_at = $2 WHERE key = $1`, yesterday.Key, midnight.Add(-time.Minute))
	te.exec(t, `UPDATE tickets SET status = 'closed', resolved_at = $2, closed_at = $2 WHERE key = $1`,
		lastWeek.Key, weekStart.Add(-time.Minute))

	b := te.admin.board(t, "")
	ids := make([]gen.BoardColumnId, len(b.Columns))
	for i, c := range b.Columns {
		ids[i] = c.Id
	}
	if want := []gen.BoardColumnId{gen.BoardColumnIdNew, gen.BoardColumnIdInProgress, gen.BoardColumnIdWaitingOnClient,
		gen.BoardColumnIdInUat, gen.BoardColumnIdResolved}; !slices.Equal(ids, want) {
		t.Fatalf("columns = %v", ids)
	}
	want := map[gen.BoardColumnId][]string{
		// Assigned tickets wait in New with the unassigned ones, longest waiting first.
		gen.BoardColumnIdNew: {fresh.Key, owned.Key, other.Key},
		// P1 first, then the longest in the column.
		gen.BoardColumnIdInProgress:      {urgent.Key, slow.Key, later.Key},
		gen.BoardColumnIdWaitingOnClient: {waiting.Key},
		gen.BoardColumnIdInUat:           {inUAT.Key},
		// Resolved and closed tickets reported this month, whenever they were resolved.
		gen.BoardColumnIdResolved: {today.Key, yesterday.Key, lastWeek.Key},
	}
	for _, c := range b.Columns {
		if got := column(b, c.Id); !slices.Equal(got, want[c.Id]) || c.Total != len(want[c.Id]) {
			t.Errorf("%s = %v (total %d), want %v", c.Id, got, c.Total, want[c.Id])
		}
	}
	if b.Stats.OpenTickets != 8 {
		t.Errorf("open tickets = %d, want 8", b.Stats.OpenTickets)
	}
	// Yesterday counts unless the week started today; last week's closed ticket never does.
	wantResolved := 2
	if midnight.Equal(weekStart) {
		wantResolved = 1
	}
	if b.Stats.ResolvedThisWeek != wantResolved {
		t.Errorf("resolved this week = %d, want %d", b.Stats.ResolvedThisWeek, wantResolved)
	}

	// Each card carries the moves the viewer may make.
	if got := card(b, fresh.Key).AllowedStatuses; slices.Contains(got, gen.TicketStatusInUat) ||
		!slices.Contains(got, gen.TicketStatusInProgress) {
		t.Errorf("new ticket allowed = %v", got)
	}
	if got := card(b, urgent.Key).AllowedStatuses; !slices.Contains(got, gen.TicketStatusNew) || slices.Contains(got, gen.TicketStatusAssigned) {
		t.Errorf("unowned in-progress ticket allowed = %v", got)
	}
	if got := card(b, owned.Key); got.Assignee == nil || got.Assignee.Id != te.agentID || got.Status != gen.TicketStatusAssigned {
		t.Errorf("owned card = %+v", got)
	}

	// Filters and search narrow the cards, never the stats.
	filtered := map[string]struct {
		s     session
		query string
		want  []string
	}{
		"mine":          {te.agent, "?assignee=me", []string{owned.Key}},
		"unassigned":    {te.admin, "?assignee=unassigned&q=printer", []string{fresh.Key}},
		"client search": {te.admin, "?q=batam", []string{other.Key}},
		"key search":    {te.admin, "?q=" + urgent.Key, []string{urgent.Key}},
		"agent scope":   {te.agent, "?q=router", []string{}},
	}
	for name, f := range filtered {
		fb := f.s.board(t, f.query)
		got := []string{}
		for _, c := range fb.Columns {
			got = append(got, column(fb, c.Id)...)
		}
		if !slices.Equal(got, f.want) {
			t.Errorf("%s: %v, want %v", name, got, f.want)
		}
		if f.s == te.admin && !reflect.DeepEqual(fb.Stats, b.Stats) {
			t.Errorf("%s: stats = %+v, want %+v", name, fb.Stats, b.Stats)
		}
	}

	// Agents see their clients only; requesters have no board.
	agentBoard := te.agent.board(t, "")
	if got := column(agentBoard, gen.BoardColumnIdNew); !slices.Equal(got, []string{fresh.Key, owned.Key}) {
		t.Errorf("agent new column = %v", got)
	}
	if agentBoard.Stats.OpenTickets != 7 {
		t.Errorf("agent open tickets = %d, want 7", agentBoard.Stats.OpenTickets)
	}
	empty := te.otherAgent.board(t, "")
	for _, c := range empty.Columns {
		if len(c.Cards) != 0 || c.Total != 0 {
			t.Errorf("unlinked agent sees %s: %v", c.Id, column(empty, c.Id))
		}
	}
	if empty.Stats != (gen.BoardStats{}) {
		t.Errorf("unlinked agent stats = %+v", empty.Stats)
	}
	expectProblem(t, te.requester.do(http.MethodGet, "/board", nil), http.StatusForbidden, "forbidden")
}

// Open tickets show whenever they were reported; the resolved column holds the resolved and
// closed tickets reported in the range, the current month by default (owner, 18 Nov).
func TestBoardDateRange(t *testing.T) {
	te := newTicketEnv(t)
	jakarta, _ := time.LoadLocation("Asia/Jakarta")
	open := func(subject string) gen.Ticket { return te.requester.report(t, incident(te.projectA, subject)) }
	resolve := func(subject string) gen.Ticket {
		tk := open(subject)
		te.move(t, tk.Key, gen.TicketStatusResolved)
		return tk
	}

	// The default range is a whole month; read it from the answer, so a run across midnight at
	// the end of a month can't fail.
	b := te.admin.board(t, "")
	from, to := b.DoneFrom.Time, b.DoneTo.Time
	if from.Day() != 1 || to.AddDate(0, 0, 1).Day() != 1 || from.Month() != to.Month() || from.Year() != to.Year() {
		t.Fatalf("default range = %s..%s, want a whole month", b.DoneFrom, b.DoneTo)
	}
	lastFrom, lastTo := from.AddDate(0, -1, 0), from.AddDate(0, 0, -1)
	// 23:30 on the last day of the month before, and 00:30 on the 1st, in Jakarta (UTC+7).
	lateLastMonth := time.Date(lastTo.Year(), lastTo.Month(), lastTo.Day(), 23, 30, 0, 0, jakarta)
	earlyFirst := time.Date(from.Year(), from.Month(), from.Day(), 0, 30, 0, 0, jakarta)

	done := resolve("Fixed this month")
	closed := resolve("Closed this month")
	te.exec(t, `UPDATE tickets SET status = 'closed', closed_at = now() WHERE key = $1`, closed.Key)
	first := resolve("Fixed on the 1st")
	te.exec(t, `UPDATE tickets SET created_at = $2 WHERE key = $1`, first.Key, earlyFirst)
	old := resolve("Fixed last month")
	stillOpen := open("Still open from last month")
	for _, k := range []string{old.Key, stillOpen.Key} {
		te.exec(t, `UPDATE tickets SET created_at = $2 WHERE key = $1`, k, lateLastMonth)
	}

	b = te.admin.board(t, "")
	if got := column(b, gen.BoardColumnIdNew); !slices.Equal(got, []string{stillOpen.Key}) {
		t.Errorf("new = %v, want the open ticket from last month", got)
	}
	if got := column(b, gen.BoardColumnIdResolved); !slices.Equal(got, []string{done.Key, closed.Key, first.Key}) {
		t.Errorf("resolved this month = %v", got)
	}
	if c := card(b, closed.Key); c.Status != gen.TicketStatusClosed || len(c.AllowedStatuses) != 0 {
		t.Errorf("closed card = %s, allowed %v", c.Status, c.AllowedStatuses)
	}

	query := "?from=" + lastFrom.Format(time.DateOnly) + "&to=" + lastTo.Format(time.DateOnly)
	lb := te.admin.board(t, query)
	if !lb.DoneFrom.Time.Equal(lastFrom) || !lb.DoneTo.Time.Equal(lastTo) {
		t.Errorf("asked range = %s..%s, want %s..%s", lb.DoneFrom, lb.DoneTo, lastFrom, lastTo)
	}
	if got := column(lb, gen.BoardColumnIdResolved); !slices.Equal(got, []string{old.Key}) {
		t.Errorf("resolved last month = %v, want %v", got, []string{old.Key})
	}
	if got := column(lb, gen.BoardColumnIdNew); !slices.Equal(got, []string{stillOpen.Key}) {
		t.Errorf("new with last month = %v", got)
	}
	if !reflect.DeepEqual(lb.Stats, b.Stats) {
		t.Errorf("stats moved with the range: %+v, want %+v", lb.Stats, b.Stats)
	}

	day := from.Format(time.DateOnly)
	// The end before the start, or half a range: the column could not name its days.
	for _, q := range []string{"?from=" + day + "&to=" + lastTo.Format(time.DateOnly), "?from=" + day, "?to=" + day} {
		expectProblem(t, te.admin.do(http.MethodGet, "/board"+q, nil), http.StatusBadRequest, apperr.CodeInvalidDateRange)
	}
	for _, q := range []string{"?from=2026-13-01&to=2026-12-31", "?from=yesterday&to=" + day} {
		expectProblem(t, te.admin.do(http.MethodGet, "/board"+q, nil), http.StatusBadRequest, apperr.CodeInvalidRequest)
	}
}

// A full column shows its first cards and still counts every ticket.
func TestBoardColumnCap(t *testing.T) {
	te := newTicketEnv(t)
	extra := 5
	n := tickets.BoardColumnLimit + extra
	te.exec(t, `INSERT INTO tickets (organization_id, number, key, type, status, priority, channel, client_id, requester_id, created_by,
		project_id, subject, description, sla_snapshot)
		SELECT (SELECT organization_id FROM clients WHERE id = $1), 900000 + g, 'HD-' || (900000 + g)::text, 'incident', 'new', 'P3', 'portal', $1, $2, $2, $3, $4, 'D', '{}'
		FROM generate_series(1, $5::int) AS g`,
		te.clientA, te.requesterID, te.projectA, "Bulk ticket", n)
	b := te.admin.board(t, "")
	c := b.Columns[0]
	if len(c.Cards) != tickets.BoardColumnLimit || c.Total != n {
		t.Fatalf("new column holds %d of %d, want %d of %d", len(c.Cards), c.Total, tickets.BoardColumnLimit, n)
	}
}
