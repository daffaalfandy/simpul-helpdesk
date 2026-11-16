package report

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

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
	clientA  = uuid.MustParse("00000000-0000-0000-0000-00000000000a")
	clientB  = uuid.MustParse("00000000-0000-0000-0000-00000000000b")
	clientC  = uuid.MustParse("00000000-0000-0000-0000-00000000000c")
	projectX = uuid.MustParse("00000000-0000-0000-0000-0000000000f1")
	projectY = uuid.MustParse("00000000-0000-0000-0000-0000000000f2")
)

func portal(first *time.Duration) Ticket {
	return Ticket{ClientID: clientA, ProjectID: projectX, ProjectName: "e-Office", Type: ticket.TypeIncident,
		Channel: ticket.ChannelPortal, FirstResponse: first, PausesWhileWaiting: true}
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

func TestBuild(t *testing.T) {
	ok := portal(minutes(10))
	late := portal(minutes(10))
	late.ResolutionBreached = true
	other := portal(minutes(20))
	other.ClientID = clientB
	stray := portal(minutes(20))
	stray.ClientID = uuid.New() // a client not listed

	// A: 1 of 10 breached (90%, on target). B: 2 tickets, 1 breached (50%). C: none.
	rowsA := slices.Repeat([]Ticket{ok}, 9)
	rowsA = append(rowsA, late)
	lateB := other
	lateB.FirstResponseBreached = true
	rows := append(rowsA, other, lateB, stray)
	clients := []Ref{{clientC, "Bappeda"}, {clientB, "BPKAD"}, {clientA, "Kominfo"}}

	rep := Build(clients, rows)
	names := make([]string, len(rep.Clients))
	for i, c := range rep.Clients {
		names[i] = c.Name
	}
	if want := []string{"Kominfo", "BPKAD", "Bappeda"}; !slices.Equal(names, want) {
		t.Errorf("order = %v, want %v (busiest first, then name)", names, want)
	}
	if rep.Totals.Tickets != 12 || rep.Totals.Breaches != 2 {
		t.Errorf("totals = %s, want 12 tickets and 2 breaches (unlisted clients left out)", show(rep.Totals))
	}
	if rep.ClientsWithTickets != 2 || rep.ClientsBelowTarget != 1 {
		t.Errorf("with tickets = %d, below target = %d; want 2 and 1", rep.ClientsWithTickets, rep.ClientsBelowTarget)
	}
	if c := rep.Clients[2]; c.Tickets != 0 || c.WithinPercent != nil || c.BelowTarget() {
		t.Errorf("client without tickets = %s, want empty and not below target", show(c.Figures))
	}
}

func TestBuildSortsTiesByName(t *testing.T) {
	rep := Build([]Ref{{clientB, "bpkad"}, {clientA, "Bappeda"}, {clientC, "Arsip"}}, nil)
	var names []string
	for _, c := range rep.Clients {
		names = append(names, c.Name)
	}
	if want := []string{"Arsip", "Bappeda", "bpkad"}; !slices.Equal(names, want) {
		t.Errorf("order = %v, want %v (case-insensitive)", names, want)
	}
}

func TestProjects(t *testing.T) {
	x := portal(minutes(10))
	y := portal(minutes(30))
	y.ProjectID, y.ProjectName = projectY, "SIPD"
	got := Projects([]Ticket{x, y, y})
	if len(got) != 2 || got[0].Name != "SIPD" || got[0].Tickets != 2 || got[1].Name != "e-Office" || got[1].Tickets != 1 {
		t.Errorf("Projects = %+v, want SIPD (2) then e-Office (1)", got)
	}
	if Projects(nil) != nil {
		t.Error("Projects(nil) should be empty")
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
