package ticket

import (
	"errors"
	"slices"
	"testing"
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
	for _, cur := range Statuses {
		got, changed := StatusAfterRequesterReply(cur)
		wantChanged := cur == wc
		want := cur
		if wantChanged {
			want = ip
		}
		if got != want || changed != wantChanged {
			t.Errorf("StatusAfterRequesterReply(%s) = %s, %v", cur, got, changed)
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
