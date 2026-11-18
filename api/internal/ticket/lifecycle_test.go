package ticket

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
)

// Shorthand for the expected-move table.
const (
	nw = StatusNew
	as = StatusAssigned
	ip = StatusInProgress
	wc = StatusWaitingOnClient
	ua = StatusInUAT
	rs = StatusResolved
	cl = StatusClosed
)

// allowed[from][hasAssignee] lists every status an agent may move to by hand.
var allowed = map[Status]map[bool][]Status{
	nw: {false: {ip, wc, rs}, true: {ip, wc, rs}},
	as: {false: {ip, wc, rs}, true: {ip, wc, rs}},
	ip: {false: {nw, wc, ua, rs}, true: {as, wc, ua, rs}},
	wc: {false: {nw, ip, ua, rs}, true: {as, ip, ua, rs}},
	ua: {false: {ip, wc, rs}, true: {ip, wc, rs}},
	rs: {false: {ip}, true: {ip}},
	cl: {false: {}, true: {}},
}

func TestManualMoveMatrix(t *testing.T) {
	if len(allowed) != len(Statuses) {
		t.Fatalf("matrix covers %d statuses, want %d", len(allowed), len(Statuses))
	}
	for _, from := range Statuses {
		for _, hasAssignee := range []bool{false, true} {
			want := allowed[from][hasAssignee]
			for _, to := range Statuses {
				err := CheckManualMove(from, to, hasAssignee)
				if ok := err == nil; ok != slices.Contains(want, to) {
					t.Errorf("%s -> %s (assignee %v): err = %v", from, to, hasAssignee, err)
				}
			}
			if got := AllowedMoves(from, hasAssignee); !slices.Equal(got, want) {
				t.Errorf("AllowedMoves(%s, %v) = %v, want %v", from, hasAssignee, got, want)
			}
		}
	}
}

func TestManualMoveErrors(t *testing.T) {
	tests := []struct {
		from, to    Status
		hasAssignee bool
		want        error
	}{
		{cl, ip, true, ErrTicketClosed},
		{ip, "done", false, ErrUnknownStatus},
		{"done", ip, false, ErrUnknownStatus},
		{ip, cl, true, ErrMoveNotAllowed},
		{ip, ip, true, ErrMoveNotAllowed},
		{ip, nw, true, ErrNeedsNoAssignee},
		{ip, as, false, ErrNeedsAssignee},
		{nw, as, true, ErrMoveNotAllowed},
	}
	for _, tt := range tests {
		if err := CheckManualMove(tt.from, tt.to, tt.hasAssignee); !errors.Is(err, tt.want) {
			t.Errorf("%s -> %s: err = %v, want %v", tt.from, tt.to, err, tt.want)
		}
	}
}

func TestStatusAfterAssignment(t *testing.T) {
	tests := []struct {
		cur         Status
		hasAssignee bool
		want        Status
	}{
		{nw, true, as},
		{nw, false, nw},
		{as, false, nw},
		{as, true, as},
		{ip, false, ip},
		{ip, true, ip},
		{wc, false, wc},
		{rs, true, rs},
	}
	for _, tt := range tests {
		if got := StatusAfterAssignment(tt.cur, tt.hasAssignee); got != tt.want {
			t.Errorf("StatusAfterAssignment(%s, %v) = %s, want %s", tt.cur, tt.hasAssignee, got, tt.want)
		}
	}
}

func TestStatusAfterRequesterReply(t *testing.T) {
	// A reply hands a ticket waiting on the client back: to its assignee, else to the New queue.
	resumed := map[bool]Status{true: ip, false: nw}
	for _, cur := range Statuses {
		for _, hasAssignee := range []bool{false, true} {
			got, changed := StatusAfterRequesterReply(cur, hasAssignee)
			wantChanged := cur == wc
			want := cur
			if wantChanged {
				want = resumed[hasAssignee]
			}
			if got != want || changed != wantChanged {
				t.Errorf("StatusAfterRequesterReply(%s, %v) = %s, %v", cur, hasAssignee, got, changed)
			}
		}
	}
}

func TestOpenAndDoneStatuses(t *testing.T) {
	for _, s := range Statuses {
		open := slices.Contains(OpenStatuses, s)
		done := slices.Contains(DoneStatuses, s)
		if open == done {
			t.Errorf("%s: open %v, done %v; want exactly one", s, open, done)
		}
		if s.IsOpen() != open || MarksResolved(s) != done {
			t.Errorf("%s: IsOpen %v, MarksResolved %v", s, s.IsOpen(), MarksResolved(s))
		}
	}
	if !ChannelPhone.AgentLoggable() || ChannelPortal.AgentLoggable() || Channel("fax").AgentLoggable() {
		t.Error("AgentLoggable")
	}
}

func TestCheckUATAnswer(t *testing.T) {
	for _, s := range Statuses {
		var want error
		switch s {
		case ua:
		case cl:
			want = ErrTicketClosed
		default:
			want = ErrNotInUAT
		}
		if err := CheckUATAnswer(s); !errors.Is(err, want) {
			t.Errorf("CheckUATAnswer(%s) = %v, want %v", s, err, want)
		}
	}
}

func TestCheckReopen(t *testing.T) {
	until := time.Date(2026, time.November, 19, 15, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		s    Status
		now  time.Time
		want error
	}{
		{"inside the window", rs, until.Add(-time.Hour), nil},
		{"at the last instant", rs, until, nil},
		{"after the window", rs, until.Add(time.Second), ErrNotReopenable},
		{"not resolved", ip, until.Add(-time.Hour), ErrNotReopenable},
		{"closed", cl, until.Add(-time.Hour), ErrTicketClosed},
	}
	for _, tt := range tests {
		if err := CheckReopen(tt.s, until, tt.now); !errors.Is(err, tt.want) {
			t.Errorf("%s: CheckReopen = %v, want %v", tt.name, err, tt.want)
		}
	}
}

func TestReopenUntil(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	weekdays := []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}
	window := calendar.Window{Open: 8 * time.Hour, Close: 17 * time.Hour}
	b, err := calendar.NewBusinessHours(calendar.Config{Location: loc, Week: calendar.UniformWeek(weekdays, window)})
	if err != nil {
		t.Fatal(err)
	}
	s := sla.Schedules{Business: b, Always: calendar.Always{}, WorkingDay: window.Length()}
	resolved := time.Date(2026, time.November, 13, 16, 0, 0, 0, loc) // Friday
	if got, want := ReopenUntil(resolved, s), time.Date(2026, time.November, 18, 16, 0, 0, 0, loc); !got.Equal(want) {
		t.Fatalf("ReopenUntil = %v, want %v", got, want)
	}
}

func TestStatusAfterClientAnswers(t *testing.T) {
	tests := []struct {
		name string
		got  Status
		want Status
	}{
		{"approve", StatusAfterUATAnswer(UATApprove, true), rs},
		{"approve without an assignee", StatusAfterUATAnswer(UATApprove, false), rs},
		{"reject", StatusAfterUATAnswer(UATReject, true), ip},
		{"reject without an assignee", StatusAfterUATAnswer(UATReject, false), nw},
		{"reopen", StatusAfterReopen(true), ip},
		{"reopen without an assignee", StatusAfterReopen(false), nw},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %s, want %s", tt.name, tt.got, tt.want)
		}
	}
	if !UATApprove.Valid() || !UATReject.Valid() || UATAnswer("maybe").Valid() {
		t.Error("UATAnswer.Valid")
	}
}
