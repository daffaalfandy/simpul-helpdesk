package notification

import (
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// The cast of every routing case.
var (
	agent      = uuid.MustParse("00000000-0000-0000-0000-00000000000a")
	agent2     = uuid.MustParse("00000000-0000-0000-0000-00000000000b")
	supervisor = uuid.MustParse("00000000-0000-0000-0000-00000000000c")
	requester  = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	colleague  = uuid.MustParse("00000000-0000-0000-0000-000000000002")
	suspended  = uuid.MustParse("00000000-0000-0000-0000-000000000003")
	auditA     = uuid.MustParse("00000000-0000-0000-0000-0000000000a1")
	auditB     = uuid.MustParse("00000000-0000-0000-0000-0000000000a2")
	messageID  = uuid.MustParse("00000000-0000-0000-0000-0000000000f1")
)

var users = map[uuid.UUID]Recipient{
	agent:      {Staff: true},
	agent2:     {Staff: true},
	supervisor: {Staff: true},
	requester:  {},
	colleague:  {},
	suspended:  {Suspended: true},
}

// got is one delivery reduced to what a case checks.
type got struct {
	user  uuid.UUID
	event Event
	kind  Kind
	also  int
}

func base() Input {
	return Input{
		Assignee: &agent, Requester: requester, Watchers: []uuid.UUID{colleague, suspended},
		Priority: sla.P3, Users: users,
	}
}

func by(id uuid.UUID, staff bool) func(*Input) {
	return func(in *Input) { in.Actor, in.ActorStaff = &id, staff }
}

func TestRoute(t *testing.T) {
	publicReply := Change{AuditID: auditA, Kind: ChangeMessage, MessageID: &messageID}
	note := Change{AuditID: auditA, Kind: ChangeMessage, MessageID: &messageID, Internal: true}
	move := func(from, to ticket.Status) Change {
		return Change{AuditID: auditB, Kind: ChangeStatus, StatusFrom: from, StatusTo: to}
	}
	moveFor := func(from, to ticket.Status, cause ticket.Cause) Change {
		c := move(from, to)
		c.Cause = cause
		return c
	}
	supervisors := func(in *Input) { in.Supervisors = []uuid.UUID{supervisor} }
	rejection := Change{AuditID: auditA, Kind: ChangeMessage, MessageID: &messageID}
	tests := []struct {
		name    string
		setup   []func(*Input)
		changes []Change
		want    []got
	}{
		{
			name:    "staff-logged ticket: requester gets the copy",
			setup:   []func(*Input){by(agent, true)},
			changes: []Change{{AuditID: auditA, Kind: ChangeCreated, StaffLogged: true}},
			want:    []got{{requester, EventTicketLogged, KindNone, 0}},
		},
		{
			name:    "portal ticket: nobody",
			setup:   []func(*Input){by(requester, false), func(in *Input) { in.Assignee = nil; in.Watchers = nil }},
			changes: []Change{{AuditID: auditA, Kind: ChangeCreated}},
		},
		{
			name:    "staff reply: assignee comment, client side reply",
			setup:   []func(*Input){by(agent2, true)},
			changes: []Change{publicReply},
			want:    []got{{agent, EventComment, KindNone, 0}, {requester, EventReply, KindNone, 0}, {colleague, EventReply, KindNone, 0}},
		},
		{
			name:    "assignee's own reply never notifies them",
			setup:   []func(*Input){by(agent, true)},
			changes: []Change{publicReply},
			want:    []got{{requester, EventReply, KindNone, 0}, {colleague, EventReply, KindNone, 0}},
		},
		{
			name:    "internal note: team only",
			setup:   []func(*Input){by(agent2, true)},
			changes: []Change{note},
			want:    []got{{agent, EventInternalNote, KindNone, 0}},
		},
		{
			name:    "requester reply resumes the ticket: one item for the assignee",
			setup:   []func(*Input){by(requester, false)},
			changes: []Change{publicReply, move(ticket.StatusWaitingOnClient, ticket.StatusInProgress)},
			want:    []got{{agent, EventComment, KindNone, 1}, {colleague, EventColleague, KindReply, 1}},
		},
		{
			name:    "move to waiting on client asks the client a question",
			setup:   []func(*Input){by(agent, true)},
			changes: []Change{move(ticket.StatusInProgress, ticket.StatusWaitingOnClient)},
			want:    []got{{requester, EventQuestion, KindNone, 0}, {colleague, EventQuestion, KindNone, 0}},
		},
		{
			name:    "resolve by a supervisor",
			setup:   []func(*Input){by(supervisor, true)},
			changes: []Change{move(ticket.StatusInProgress, ticket.StatusResolved)},
			want:    []got{{agent, EventStatus, KindNone, 0}, {requester, EventResolved, KindNone, 0}, {colleague, EventResolved, KindNone, 0}},
		},
		{
			name:    "other moves are status changes",
			setup:   []func(*Input){by(agent, true)},
			changes: []Change{move(ticket.StatusAssigned, ticket.StatusInProgress)},
			want:    []got{{requester, EventStatus, KindNone, 0}, {colleague, EventStatus, KindNone, 0}},
		},
		{
			name:  "reassignment with a status flip",
			setup: []func(*Input){by(supervisor, true), func(in *Input) { in.Assignee = &agent2 }},
			changes: []Change{
				{AuditID: auditA, Kind: ChangeAssignee, AssigneeTo: &Person{ID: agent2}, AssigneeOut: &Person{ID: agent}},
				move(ticket.StatusNew, ticket.StatusAssigned),
			},
			want: []got{
				{agent2, EventAssignment, KindAssigned, 1}, {agent, EventAssignment, KindUnassigned, 0},
				{requester, EventStatus, KindNone, 0}, {colleague, EventStatus, KindNone, 0},
			},
		},
		{
			name:    "priority change: assignee only",
			setup:   []func(*Input){by(supervisor, true)},
			changes: []Change{{AuditID: auditA, Kind: ChangePriority, Priority: &PriorityChange{From: sla.P3, To: sla.P2}}},
			want:    []got{{agent, EventPriority, KindNone, 0}},
		},
		{
			name:    "colleague added",
			setup:   []func(*Input){by(requester, false)},
			changes: []Change{{AuditID: auditA, Kind: ChangeWatcherAdded, WatcherID: &colleague}},
			want:    []got{{colleague, EventColleague, KindAdded, 0}},
		},
		{
			name:    "colleague removed: nobody",
			setup:   []func(*Input){by(requester, false)},
			changes: []Change{{AuditID: auditA, Kind: ChangeWatcherRemoved, WatcherID: &colleague}},
		},
		{
			name:    "SLA at risk: assignee only",
			changes: []Change{{AuditID: auditA, Kind: ChangeSLAAtRisk}},
			want:    []got{{agent, EventSLA, KindAtRisk, 0}},
		},
		{
			name:    "SLA breach: assignee and supervisors",
			setup:   []func(*Input){supervisors, func(in *Input) { in.NotifySupervisorOnBreach = true }},
			changes: []Change{{AuditID: auditA, Kind: ChangeSLABreached}},
			want:    []got{{agent, EventSLA, KindBreached, 0}, {supervisor, EventSLA, KindBreached, 0}},
		},
		{
			name:    "SLA breach on a project that doesn't alert supervisors",
			setup:   []func(*Input){supervisors},
			changes: []Change{{AuditID: auditA, Kind: ChangeSLABreached}},
			want:    []got{{agent, EventSLA, KindBreached, 0}},
		},
		{
			name:    "move In UAT asks the client side to test",
			setup:   []func(*Input){by(agent, true)},
			changes: []Change{move(ticket.StatusInProgress, ticket.StatusInUAT)},
			want:    []got{{requester, EventUATRequested, KindNone, 0}, {colleague, EventUATRequested, KindNone, 0}},
		},
		{
			name:    "requester approves: assignee hears the result, the colleague a colleague's change",
			setup:   []func(*Input){by(requester, false)},
			changes: []Change{moveFor(ticket.StatusInUAT, ticket.StatusResolved, ticket.CauseUATApproved)},
			want:    []got{{agent, EventUATResult, KindApproved, 0}, {colleague, EventColleague, KindStatus, 0}},
		},
		{
			name:    "requester rejects with a message: one result item for the assignee",
			setup:   []func(*Input){by(requester, false)},
			changes: []Change{rejection, moveFor(ticket.StatusInUAT, ticket.StatusInProgress, ticket.CauseUATRejected)},
			want:    []got{{agent, EventUATResult, KindRejected, 1}, {colleague, EventColleague, KindReply, 1}},
		},
		{
			name:    "overdue test approved by the system",
			changes: []Change{moveFor(ticket.StatusInUAT, ticket.StatusResolved, ticket.CauseUATAutoApproved)},
			want: []got{
				{agent, EventUATResult, KindAutoApproved, 0}, {requester, EventResolved, KindNone, 0},
				{colleague, EventResolved, KindNone, 0},
			},
		},
		{
			name:    "reopen window ends: closed for everyone",
			changes: []Change{moveFor(ticket.StatusResolved, ticket.StatusClosed, ticket.CauseAutoClosed)},
			want: []got{
				{agent, EventStatus, KindNone, 0}, {requester, EventResolved, KindNone, 0},
				{colleague, EventResolved, KindNone, 0},
			},
		},
		{
			name:    "requester reopens: the assignee hears it",
			setup:   []func(*Input){by(requester, false)},
			changes: []Change{moveFor(ticket.StatusResolved, ticket.StatusInProgress, ticket.CauseReopened)},
			want:    []got{{agent, EventStatus, KindNone, 0}, {colleague, EventColleague, KindStatus, 0}},
		},
		{
			name:    "resolved without the DoD: supervisors are told",
			setup:   []func(*Input){by(agent, true), supervisors},
			changes: []Change{moveFor(ticket.StatusInProgress, ticket.StatusResolved, ticket.CauseDoDSkipped)},
			want: []got{
				{supervisor, EventDoD, KindSkipped, 0}, {requester, EventResolved, KindNone, 0},
				{colleague, EventResolved, KindNone, 0},
			},
		},
		{
			name:    "a ticket's own DoD: supervisors only",
			setup:   []func(*Input){by(agent, true), supervisors},
			changes: []Change{{AuditID: auditA, Kind: ChangeDoD}},
			want:    []got{{supervisor, EventDoD, KindOverride, 0}},
		},
		{
			name:    "code activity: the assignee only, never client staff",
			setup:   []func(*Input){by(colleague, false)},
			changes: []Change{{AuditID: auditA, Kind: ChangeCode, Code: &Code{Provider: "github", Kind: "commit", Ref: "a1f9c2e"}}},
			want:    []got{{agent, EventCode, KindNone, 0}},
		},
		{
			name:  "code that moves the ticket: the team hears the code, the client the status",
			setup: []func(*Input){func(in *Input) { in.Actor = nil }},
			changes: []Change{
				{AuditID: auditA, Kind: ChangeCode, Code: &Code{Provider: "github", Kind: "pr_merged", Ref: "142"}},
				moveFor(ticket.StatusInProgress, ticket.StatusInUAT, ticket.CauseCodeDone),
			},
			want: []got{
				{agent, EventStatus, KindNone, 1}, {requester, EventUATRequested, KindNone, 0},
				{colleague, EventUATRequested, KindNone, 0},
			},
		},
		{
			name:    "unassigned SLA at risk: nobody (HANDOFF §13 Q7)",
			setup:   []func(*Input){func(in *Input) { in.Assignee = nil }},
			changes: []Change{{AuditID: auditA, Kind: ChangeSLAAtRisk}},
		},
		{
			name: "a switched-off event falls back to the next one",
			setup: []func(*Input){by(requester, false), func(in *Input) {
				in.Prefs = map[uuid.UUID]map[Event]Pref{agent: {EventComment: {}}}
			}},
			changes: []Change{publicReply, move(ticket.StatusWaitingOnClient, ticket.StatusInProgress)},
			want:    []got{{agent, EventStatus, KindNone, 1}, {colleague, EventColleague, KindReply, 1}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := base()
			for _, f := range tt.setup {
				f(&in)
			}
			in.Changes = tt.changes
			ds, dropped := Route(in)
			if dropped != 0 {
				t.Errorf("dropped = %d", dropped)
			}
			var gotten []got
			for _, d := range ds {
				gotten = append(gotten, got{d.UserID, d.Event, d.Kind, len(d.Also)})
			}
			if !slices.Equal(gotten, tt.want) {
				t.Errorf("deliveries\n got %v\nwant %v", gotten, tt.want)
			}
		})
	}
}

// A rule that sent an internal note to client staff would be dropped, not delivered.
func TestRouteNeverSendsInternalNotesToClients(t *testing.T) {
	in := base()
	in.Assignee = &requester // only a bad rule or bad data could do this
	in.Actor, in.ActorStaff = &agent, true
	in.Changes = []Change{{AuditID: auditA, Kind: ChangeMessage, Internal: true}}
	ds, dropped := Route(in)
	if len(ds) != 0 || dropped != 1 {
		t.Fatalf("deliveries %v, dropped %d; want none delivered, 1 dropped", ds, dropped)
	}
}

func TestRouteUrgency(t *testing.T) {
	for _, tt := range []struct {
		priority sla.Priority
		change   ChangeKind
		want     bool
	}{
		{sla.P1, ChangePriority, true},
		{sla.P2, ChangePriority, false},
		{sla.P4, ChangeSLAAtRisk, true},
	} {
		in := base()
		in.Priority = tt.priority
		in.Changes = []Change{{AuditID: auditA, Kind: tt.change}}
		ds, _ := Route(in)
		if len(ds) != 1 || ds[0].Urgent != tt.want {
			t.Errorf("%s %s: deliveries %v, want one with urgent=%v", tt.priority, tt.change, ds, tt.want)
		}
	}
}

func TestRouteChannelsFollowPreferences(t *testing.T) {
	in := base()
	in.Actor, in.ActorStaff = &agent2, true
	in.Changes = []Change{{AuditID: auditA, Kind: ChangeMessage, Internal: true}}
	ds, _ := Route(in)
	if len(ds) != 1 || ds[0].Channels != (Pref{Web: true}) {
		t.Fatalf("internal note defaults to web only, got %v", ds)
	}
	in.Prefs = map[uuid.UUID]map[Event]Pref{agent: {EventInternalNote: {}}}
	if ds, _ := Route(in); len(ds) != 0 {
		t.Fatalf("a switched-off event notifies nobody, got %v", ds)
	}
}

// UAT items go right away whatever the user's summary setting; staff reasons never reach the
// client side.
func TestRouteUrgencyAndReasons(t *testing.T) {
	in := base()
	in.Actor, in.ActorStaff = &agent2, true
	in.Supervisors = []uuid.UUID{supervisor}
	in.Changes = []Change{{AuditID: auditA, Kind: ChangeStatus, StatusFrom: ticket.StatusInProgress,
		StatusTo: ticket.StatusResolved, Cause: ticket.CauseDoDSkipped, Reason: "Client confirmed by phone"}}
	ds, _ := Route(in)
	for _, d := range ds {
		staff := users[d.UserID].Staff
		if (d.Change.Reason != "") != staff {
			t.Errorf("%s (staff %v) reason = %q", d.UserID, staff, d.Change.Reason)
		}
	}
	in.Changes = []Change{{AuditID: auditA, Kind: ChangeStatus, StatusFrom: ticket.StatusInProgress, StatusTo: ticket.StatusInUAT}}
	ds, _ = Route(in)
	for _, d := range ds {
		if d.UserID == requester && (!d.Urgent || d.Event != EventUATRequested) {
			t.Fatalf("UAT request = %+v", d)
		}
		if d.UserID == agent && d.Urgent {
			t.Fatalf("a P3 status change is urgent: %+v", d)
		}
	}
}
