package report

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

var jakarta = mustZone("Asia/Jakarta")

func mustZone(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

func TestParseMonth(t *testing.T) {
	tests := []struct {
		in   string
		want Month
		err  bool
	}{
		{in: "2026-11", want: Month{2026, time.November}},
		{in: "2026-01", want: Month{2026, time.January}},
		{in: "2026-1", err: true},
		{in: "2026-13", err: true},
		{in: "2026-00", err: true},
		{in: "26-11", err: true},
		{in: "2026-11-01", err: true},
		{in: "", err: true},
	}
	for _, tt := range tests {
		got, err := ParseMonth(tt.in)
		if tt.err {
			if !errors.Is(err, ErrInvalidMonth) {
				t.Errorf("ParseMonth(%q) error = %v, want ErrInvalidMonth", tt.in, err)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("ParseMonth(%q) = %v, %v; want %v", tt.in, got, err, tt.want)
		}
		if got.String() != tt.in {
			t.Errorf("Month(%v).String() = %q, want %q", got, got.String(), tt.in)
		}
	}
}

func TestMonthBoundsAndSteps(t *testing.T) {
	newYork := mustZone("America/New_York")
	tests := []struct {
		name       string
		m          Month
		loc        *time.Location
		start, end time.Time
	}{
		{"Jakarta November", Month{2026, time.November}, jakarta,
			time.Date(2026, time.November, 1, 0, 0, 0, 0, jakarta), time.Date(2026, time.December, 1, 0, 0, 0, 0, jakarta)},
		{"December rolls the year", Month{2026, time.December}, jakarta,
			time.Date(2026, time.December, 1, 0, 0, 0, 0, jakarta), time.Date(2027, time.January, 1, 0, 0, 0, 0, jakarta)},
		// November in New York starts on summer time and ends on standard time.
		{"DST zone", Month{2026, time.November}, newYork,
			time.Date(2026, time.November, 1, 0, 0, 0, 0, newYork), time.Date(2026, time.December, 1, 0, 0, 0, 0, newYork)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := tt.m.Bounds(tt.loc)
			if !start.Equal(tt.start) || !end.Equal(tt.end) {
				t.Errorf("Bounds = %v .. %v, want %v .. %v", start, end, tt.start, tt.end)
			}
		})
	}

	jan := Month{2027, time.January}
	if got := jan.Prev(); got != (Month{2026, time.December}) {
		t.Errorf("Prev = %v", got)
	}
	if got := (Month{2026, time.December}).Next(); got != jan {
		t.Errorf("Next = %v", got)
	}
	if !jan.Prev().Before(jan) || !jan.After(jan.Prev()) || jan.Before(jan) {
		t.Error("Before/After disagree")
	}
}

func TestMonthOfReadsTheOrganisationZone(t *testing.T) {
	// 31 Oct 23:30 WIB is 16:30 UTC: still October in Jakarta.
	late := time.Date(2026, time.October, 31, 23, 30, 0, 0, jakarta)
	if got := MonthOf(late.UTC(), jakarta); got != (Month{2026, time.October}) {
		t.Errorf("MonthOf = %v, want 2026-10", got)
	}
	if got := MonthOf(late.Add(time.Hour).UTC(), jakarta); got != (Month{2026, time.November}) {
		t.Errorf("MonthOf = %v, want 2026-11", got)
	}
}

func minutes(n int) *time.Duration {
	d := time.Duration(n) * time.Minute
	return &d
}

func pct(v float64) *float64 { return &v }

var (
	clientA  = Ref{uuid.MustParse("00000000-0000-0000-0000-00000000000a"), "Dinas Kominfo Jambi"}
	clientB  = Ref{uuid.MustParse("00000000-0000-0000-0000-00000000000b"), "bappeda Prov. Riau"}
	projectX = uuid.MustParse("00000000-0000-0000-0000-0000000000f1")
)

func portal(first *time.Duration) Ticket {
	return Ticket{ProjectID: projectX, Priority: sla.P3, Type: ticket.TypeIncident, Channel: ticket.ChannelPortal,
		FirstResponse: first, PausesWhileWaiting: true}
}

func TestSummarize(t *testing.T) {
	logged := portal(minutes(0))
	logged.Channel = ticket.ChannelPhone
	request := portal(minutes(30))
	request.Type = ticket.TypeRequest
	lateReply := portal(minutes(90))
	lateReply.FirstResponseBreached = true
	lateFix := portal(minutes(10))
	lateFix.ResolutionBreached = true
	bothLate := portal(minutes(50))
	bothLate.FirstResponseBreached, bothLate.ResolutionBreached = true, true

	tests := []struct {
		name string
		rows []Ticket
		want Figures
	}{
		{name: "no tickets", rows: nil, want: Figures{}},
		{
			name: "split, average of answered portal reports",
			rows: []Ticket{portal(minutes(10)), request, portal(nil)},
			want: Figures{Tickets: 3, Incidents: 2, Requests: 1, WithinPercent: pct(100), AvgFirstResponse: minutes(20)},
		},
		{
			// Logged tickets are answered when logged; counting them would pull the average to 0.
			name: "tickets the team logs stay out of the average",
			rows: []Ticket{portal(minutes(40)), logged},
			want: Figures{Tickets: 2, Incidents: 2, WithinPercent: pct(100), AvgFirstResponse: minutes(40)},
		},
		{
			name: "only logged tickets: no average",
			rows: []Ticket{logged},
			want: Figures{Tickets: 1, Incidents: 1, WithinPercent: pct(100)},
		},
		{
			name: "a ticket that missed both targets is one breach",
			rows: []Ticket{lateReply, lateFix, bothLate, portal(minutes(10))},
			want: Figures{Tickets: 4, Incidents: 4, Breaches: 3, WithinPercent: pct(25), AvgFirstResponse: minutes(40)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Summarize(tt.rows)
			if !sameFigures(got, tt.want) {
				t.Errorf("Summarize = %s, want %s", show(got), show(tt.want))
			}
		})
	}
}

func TestBelowTarget(t *testing.T) {
	tests := []struct {
		within *float64
		want   bool
	}{
		{nil, false}, // no tickets is never below target
		{pct(89.9), true},
		{pct(TargetPercent), false},
		{pct(100), false},
	}
	for _, tt := range tests {
		if got := (Figures{WithinPercent: tt.within}).BelowTarget(); got != tt.want {
			t.Errorf("BelowTarget(%v) = %v, want %v", deref(tt.within), got, tt.want)
		}
	}
}

func TestOldest(t *testing.T) {
	tests := []struct {
		current, want Month
	}{
		{Month{2026, time.November}, Month{2025, time.November}},
		{Month{2026, time.January}, Month{2025, time.January}},
		{Month{2026, time.December}, Month{2025, time.December}},
	}
	for _, tt := range tests {
		got := Oldest(tt.current)
		if got != tt.want {
			t.Errorf("Oldest(%v) = %v, want %v", tt.current, got, tt.want)
		}
		// The report offers the current month and MonthsBack before it.
		n := 0
		for m := tt.current; !m.Before(got); m = m.Prev() {
			n++
		}
		if n != MonthsBack+1 {
			t.Errorf("Oldest(%v) offers %d months, want %d", tt.current, n, MonthsBack+1)
		}
	}
}

func TestMonthRangeValidate(t *testing.T) {
	nov, jan := Month{2026, time.November}, Month{2027, time.January}
	tests := []struct {
		name    string
		r       MonthRange
		current Month
		want    error
	}{
		{"current month only", SingleMonth(nov), nov, nil},
		{"one earlier month", SingleMonth(Month{2026, time.September}), nov, nil},
		{"several months to date", MonthRange{Month{2026, time.September}, nov}, nov, nil},
		{"the whole window", MonthRange{Month{2025, time.November}, nov}, nov, nil},
		{"across the year end", MonthRange{Month{2026, time.November}, jan}, jan, nil},
		{"oldest month alone", SingleMonth(Month{2026, time.January}), jan, nil},
		{"end before start", MonthRange{Month{2026, time.October}, Month{2026, time.September}}, nov, ErrMonthsReversed},
		{"end before start across the year end", MonthRange{jan, Month{2026, time.December}}, jan, ErrMonthsReversed},
		{"start too far back", MonthRange{Month{2025, time.October}, nov}, nov, ErrMonthTooOld},
		{"both too far back", SingleMonth(Month{2025, time.December}), jan, ErrMonthTooOld},
		{"end not started", MonthRange{Month{2026, time.October}, Month{2026, time.December}}, nov, ErrMonthInFuture},
		{"next year", SingleMonth(jan), nov, ErrMonthInFuture},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.r.Validate(tt.current); !errors.Is(err, tt.want) {
				t.Errorf("Validate(%v..%v, current %v) = %v, want %v", tt.r.From, tt.r.To, tt.current, err, tt.want)
			}
		})
	}
}

func TestMonthRangeBounds(t *testing.T) {
	newYork := mustZone("America/New_York")
	tests := []struct {
		name       string
		r          MonthRange
		loc        *time.Location
		start, end time.Time
		single     bool
	}{
		{"one month", SingleMonth(Month{2026, time.November}), jakarta,
			time.Date(2026, time.November, 1, 0, 0, 0, 0, jakarta), time.Date(2026, time.December, 1, 0, 0, 0, 0, jakarta), true},
		{"three months", MonthRange{Month{2026, time.September}, Month{2026, time.November}}, jakarta,
			time.Date(2026, time.September, 1, 0, 0, 0, 0, jakarta), time.Date(2026, time.December, 1, 0, 0, 0, 0, jakarta), false},
		{"across the year end", MonthRange{Month{2026, time.November}, Month{2027, time.January}}, jakarta,
			time.Date(2026, time.November, 1, 0, 0, 0, 0, jakarta), time.Date(2027, time.February, 1, 0, 0, 0, 0, jakarta), false},
		// Starts on standard time and ends on summer time: still local midnight at both ends.
		{"DST zone", MonthRange{Month{2026, time.February}, Month{2026, time.April}}, newYork,
			time.Date(2026, time.February, 1, 0, 0, 0, 0, newYork), time.Date(2026, time.May, 1, 0, 0, 0, 0, newYork), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := tt.r.Bounds(tt.loc)
			if !start.Equal(tt.start) || !end.Equal(tt.end) {
				t.Errorf("Bounds = %v .. %v, want %v .. %v", start, end, tt.start, tt.end)
			}
			if got := tt.r.Single(); got != tt.single {
				t.Errorf("Single = %v, want %v", got, tt.single)
			}
		})
	}
}

// on returns n copies of a ticket on project p; breached of them missed a target.
func on(p uuid.UUID, n, breached int) []Ticket {
	out := make([]Ticket, n)
	for i := range out {
		out[i] = portal(minutes(10))
		out[i].ProjectID = p
		out[i].ResolutionBreached = i < breached
	}
	return out
}

func TestBuild(t *testing.T) {
	eOffice := Project{ID: uuid.New(), Name: "e-Office", Client: clientA, Active: true}
	email := Project{ID: uuid.New(), Name: "Email", Client: clientA, Active: true}
	website := Project{ID: uuid.New(), Name: "website", Client: clientB, Active: true}
	sipd := Project{ID: uuid.New(), Name: "SIPD", Client: clientB, Active: true}
	legacy := Project{ID: uuid.New(), Name: "Legacy portal", Client: clientA, Active: false}
	archive := Project{ID: uuid.New(), Name: "Archive", Client: clientB, Active: false}
	all := []Project{eOffice, email, website, sipd, legacy, archive}

	tests := []struct {
		name                    string
		projects                []Project
		rows                    []Ticket
		order                   []string
		tickets, breaches       int
		withTickets, belowCount int
	}{
		{
			name:     "no tickets: active projects by client, then name; hidden ones left out",
			projects: all,
			order:    []string{"bappeda Prov. Riau/SIPD", "bappeda Prov. Riau/website", "Dinas Kominfo Jambi/e-Office", "Dinas Kominfo Jambi/Email"},
		},
		{
			// e-Office: 1 of 10 breached (90%, on target). website: 1 of 2 (50%). Legacy is hidden
			// but has a ticket this month, so it is listed and counted.
			name:     "busiest first; a hidden project with tickets is listed",
			projects: all,
			rows:     slices.Concat(on(eOffice.ID, 10, 1), on(website.ID, 2, 1), on(legacy.ID, 1, 0)),
			order: []string{"Dinas Kominfo Jambi/e-Office", "bappeda Prov. Riau/website", "Dinas Kominfo Jambi/Legacy portal",
				"bappeda Prov. Riau/SIPD", "Dinas Kominfo Jambi/Email"},
			tickets: 13, breaches: 2, withTickets: 3, belowCount: 1,
		},
		{
			name:     "tickets of projects not listed are left out",
			projects: []Project{eOffice},
			rows:     slices.Concat(on(eOffice.ID, 2, 0), on(uuid.New(), 3, 3)),
			order:    []string{"Dinas Kominfo Jambi/e-Office"},
			tickets:  2, withTickets: 1,
		},
		{
			name:     "ties on tickets sort by client, then project, ignoring case",
			projects: all,
			rows:     slices.Concat(on(email.ID, 1, 0), on(sipd.ID, 1, 0), on(eOffice.ID, 1, 0), on(website.ID, 1, 0)),
			order: []string{"bappeda Prov. Riau/SIPD", "bappeda Prov. Riau/website", "Dinas Kominfo Jambi/e-Office",
				"Dinas Kominfo Jambi/Email"},
			tickets: 4, withTickets: 4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rep := Build(tt.projects, tt.rows)
			var order []string
			for _, g := range rep.Projects {
				order = append(order, g.Client.Name+"/"+g.Name)
			}
			if !slices.Equal(order, tt.order) {
				t.Errorf("order = %v, want %v", order, tt.order)
			}
			if rep.Totals.Tickets != tt.tickets || rep.Totals.Breaches != tt.breaches {
				t.Errorf("totals = %s, want %d tickets and %d breaches", show(rep.Totals), tt.tickets, tt.breaches)
			}
			if rep.ProjectsWithTickets != tt.withTickets || rep.ProjectsBelowTarget != tt.belowCount {
				t.Errorf("with tickets = %d, below target = %d; want %d and %d",
					rep.ProjectsWithTickets, rep.ProjectsBelowTarget, tt.withTickets, tt.belowCount)
			}
			for _, g := range rep.Projects {
				if g.Tickets == 0 && (g.WithinPercent != nil || g.BelowTarget()) {
					t.Errorf("%s without tickets = %s, want empty and not below target", g.Name, show(g.Figures))
				}
			}
		})
	}
}

func TestByPriority(t *testing.T) {
	at := func(p sla.Priority, n, breached int) []Ticket {
		out := on(projectX, n, breached)
		for i := range out {
			out[i].Priority = p
		}
		return out
	}
	tests := []struct {
		name string
		rows []Ticket
		want []string
	}{
		{name: "no tickets", rows: nil, want: nil},
		{
			name: "P1 first whatever the row order; empty priorities left out",
			rows: slices.Concat(at(sla.P4, 1, 0), at(sla.P1, 2, 1), at(sla.P3, 3, 0)),
			want: []string{"P1: 2 tickets, 1 breaches", "P3: 3 tickets, 0 breaches", "P4: 1 tickets, 0 breaches"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, g := range ByPriority(tt.rows) {
				got = append(got, fmt.Sprintf("%s: %d tickets, %d breaches", g.Priority, g.Tickets, g.Breaches))
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("ByPriority = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWaitingPauseOf(t *testing.T) {
	pausing := portal(nil)
	running := portal(nil)
	running.PausesWhileWaiting = false
	tests := []struct {
		rows   []Ticket
		want   WaitingPause
		wantOK bool
	}{
		{nil, "", false},
		{[]Ticket{pausing, pausing}, WaitingPauseAll, true},
		{[]Ticket{pausing, running}, WaitingPauseSome, true},
		{[]Ticket{running}, WaitingPauseNone, true},
	}
	for _, tt := range tests {
		got, ok := WaitingPauseOf(tt.rows)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("WaitingPauseOf(%d rows) = %q, %v; want %q, %v", len(tt.rows), got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestHoursMinutes(t *testing.T) {
	tests := []struct {
		d    time.Duration
		h, m int
	}{
		{0, 0, 0},
		{59 * time.Second, 0, 0},
		{16*time.Minute + 40*time.Second, 0, 16},
		{26*time.Hour + 5*time.Minute, 26, 5},
		{-time.Minute, 0, 0},
	}
	for _, tt := range tests {
		if h, m := HoursMinutes(tt.d); h != tt.h || m != tt.m {
			t.Errorf("HoursMinutes(%v) = %d, %d; want %d, %d", tt.d, h, m, tt.h, tt.m)
		}
	}
}

func sameFigures(a, b Figures) bool {
	return a.Tickets == b.Tickets && a.Incidents == b.Incidents && a.Requests == b.Requests &&
		a.Breaches == b.Breaches && deref(a.WithinPercent) == deref(b.WithinPercent) &&
		(a.WithinPercent == nil) == (b.WithinPercent == nil) &&
		(a.AvgFirstResponse == nil) == (b.AvgFirstResponse == nil) &&
		(a.AvgFirstResponse == nil || *a.AvgFirstResponse == *b.AvgFirstResponse)
}

func deref[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

func show(f Figures) string {
	return fmt.Sprintf("{tickets %d, incidents %d, requests %d, breaches %d, within %v, avg %v}",
		f.Tickets, f.Incidents, f.Requests, f.Breaches, deref(f.WithinPercent), deref(f.AvgFirstResponse))
}

func TestClockOutcome(t *testing.T) {
	tests := []struct {
		met, breached, done bool
		want                Outcome
	}{
		{met: true, want: OutcomeMet},
		{met: true, breached: true, want: OutcomeBreached}, // met late
		{breached: true, want: OutcomeBreached},
		{done: true, want: OutcomeMet}, // resolved before any reply, in time
		{want: OutcomeRunning},
	}
	for _, tt := range tests {
		if got := ClockOutcome(tt.met, tt.breached, tt.done); got != tt.want {
			t.Errorf("ClockOutcome(met %v, breached %v, done %v) = %s, want %s", tt.met, tt.breached, tt.done, got, tt.want)
		}
	}
}
