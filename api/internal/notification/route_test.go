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
	boss2      = uuid.MustParse("00000000-0000-0000-0000-00000000000d") // a second supervisor
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
	boss2:      {Staff: true},
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

func unassigned(in *Input) { in.Assignee = nil }

// withSupervisors fills in the supervisors when the changes concern them, as the fan-out does.
func withSupervisors(in *Input) {
	if in.NeedsSupervisors() {
		in.Supervisors = []uuid.UUID{supervisor, boss2}
	}
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
	rejection := Change{AuditID: auditA, Kind: ChangeMessage, MessageID: &messageID}
	created := func(staffLogged bool, assignee *uuid.UUID) Change {
		c := Change{AuditID: auditA, Kind: ChangeCreated, StaffLogged: staffLogged}
		if assignee != nil {
			c.AssigneeTo = &Person{ID: *assignee}
		}
		return c
	}
	tests := []struct {
		name    string
		setup   []func(*Input)
		changes []Change
		want    []got
	}{
		{
			name:    "logged and assigned to me: only the requester gets the copy",
			setup:   []func(*Input){by(agent, true)},
			changes: []Change{created(true, &agent)},
			want:    []got{{requester, EventTicketLogged, KindNone, 0}},
		},
		{
			name:    "portal ticket nobody is on: the supervisors are told",
			setup:   []func(*Input){by(requester, false), unassigned, func(in *Input) { in.Watchers = nil }},
			changes: []Change{created(false, nil)},
			want:    []got{{supervisor, EventNewTicket, KindNone, 0}, {boss2, EventNewTicket, KindNone, 0}},
		},
		{
			name:    "a supervisor logs an unassigned ticket: the other supervisors and the requester",
			setup:   []func(*Input){by(supervisor, true), unassigned},
			changes: []Change{created(true, nil)},
			want:    []got{{boss2, EventNewTicket, KindNone, 0}, {requester, EventTicketLogged, KindNone, 0}},
		},
		{
			name:    "logged with another assignee: that assignee, not the supervisors",
			setup:   []func(*Input){by(supervisor, true), func(in *Input) { in.Assignee = &agent2 }},
			changes: []Change{created(true, &agent2)},
			want:    []got{{agent2, EventAssignment, KindAssigned, 0}, {requester, EventTicketLogged, KindNone, 0}},
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
			setup:   []func(*Input){func(in *Input) { in.NotifySupervisorOnBreach = true }},
			changes: []Change{{AuditID: auditA, Kind: ChangeSLABreached}},
			want: []got{
				{agent, EventSLA, KindBreached, 0}, {supervisor, EventSLA, KindBreached, 0}, {boss2, EventSLA, KindBreached, 0},
			},
		},
		{
			name:    "SLA breach on a project that doesn't alert supervisors",
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
			setup:   []func(*Input){by(agent, true)},
			changes: []Change{moveFor(ticket.StatusInProgress, ticket.StatusResolved, ticket.CauseDoDSkipped)},
			want: []got{
				{supervisor, EventDoD, KindSkipped, 0}, {boss2, EventDoD, KindSkipped, 0}, {requester, EventResolved, KindNone, 0},
				{colleague, EventResolved, KindNone, 0},
			},
		},
		{
			name:    "resolved without the DoD while unassigned: one DoD alert per supervisor",
			setup:   []func(*Input){by(supervisor, true), unassigned},
			changes: []Change{moveFor(ticket.StatusInProgress, ticket.StatusResolved, ticket.CauseDoDSkipped)},
			want: []got{
				{boss2, EventDoD, KindSkipped, 0}, {requester, EventResolved, KindNone, 0}, {colleague, EventResolved, KindNone, 0},
			},
		},
		{
			name:    "a ticket's own DoD: supervisors only",
			setup:   []func(*Input){by(agent, true)},
			changes: []Change{{AuditID: auditA, Kind: ChangeDoD}},
			want:    []got{{supervisor, EventDoD, KindOverride, 0}, {boss2, EventDoD, KindOverride, 0}},
		},
		{
			name:    "unassigned: a move goes to the supervisors",
			setup:   []func(*Input){by(agent, true), unassigned},
			changes: []Change{move(ticket.StatusNew, ticket.StatusWaitingOnClient)},
			want: []got{
				{supervisor, EventStatus, KindNone, 0}, {boss2, EventStatus, KindNone, 0},
				{requester, EventQuestion, KindNone, 0}, {colleague, EventQuestion, KindNone, 0},
			},
		},
		{
			name:    "unassigned: the client's reply goes to the supervisors",
			setup:   []func(*Input){by(requester, false), unassigned},
			changes: []Change{publicReply, move(ticket.StatusWaitingOnClient, ticket.StatusNew)},
			want: []got{
				{supervisor, EventComment, KindNone, 1}, {boss2, EventComment, KindNone, 1}, {colleague, EventColleague, KindReply, 1},
			},
		},
		{
			name:    "unassigned: a teammate's public reply goes to the supervisors",
			setup:   []func(*Input){by(agent, true), unassigned},
			changes: []Change{publicReply},
			want: []got{
				{supervisor, EventComment, KindNone, 0}, {boss2, EventComment, KindNone, 0},
				{requester, EventReply, KindNone, 0}, {colleague, EventReply, KindNone, 0},
			},
		},
		{
			name:    "unassigned: a rejected test goes to the supervisors",
			setup:   []func(*Input){by(requester, false), unassigned},
			changes: []Change{rejection, moveFor(ticket.StatusInUAT, ticket.StatusInProgress, ticket.CauseUATRejected)},
			want: []got{
				{supervisor, EventUATResult, KindRejected, 1}, {boss2, EventUATResult, KindRejected, 1},
				{colleague, EventColleague, KindReply, 1},
			},
		},
		{
			name:  "an agent releases their ticket: the supervisors hear it is back in New",
			setup: []func(*Input){by(agent, true), unassigned},
			changes: []Change{
				{AuditID: auditA, Kind: ChangeAssignee, AssigneeOut: &Person{ID: agent}},
				move(ticket.StatusAssigned, ticket.StatusNew),
			},
			want: []got{
				{supervisor, EventStatus, KindNone, 0}, {boss2, EventStatus, KindNone, 0},
				{requester, EventStatus, KindNone, 0}, {colleague, EventStatus, KindNone, 0},
			},
		},
		{
			name:    "unassigned: an internal note tells nobody",
			setup:   []func(*Input){by(agent2, true), unassigned},
			changes: []Change{note},
		},
		{
			name:    "unassigned: a priority change tells nobody",
			setup:   []func(*Input){by(supervisor, true), unassigned},
			changes: []Change{{AuditID: auditA, Kind: ChangePriority, Priority: &PriorityChange{From: sla.P3, To: sla.P2}}},
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
			withSupervisors(&in)
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

func TestNeedsSupervisors(t *testing.T) {
	assignee := &Person{ID: agent}
	tests := []struct {
		name     string
		assigned bool
		breach   bool // the project alerts supervisors about breaches
		change   Change
		want     bool
	}{
		{"breach, project alerts them", true, true, Change{Kind: ChangeSLABreached}, true},
		{"breach, project doesn't", true, false, Change{Kind: ChangeSLABreached}, false},
		{"a ticket's own DoD", true, false, Change{Kind: ChangeDoD}, true},
		{"resolved without the DoD", true, false, Change{Kind: ChangeStatus, Cause: ticket.CauseDoDSkipped}, true},
		{"reported with nobody on it", false, false, Change{Kind: ChangeCreated}, true},
		{"logged and assigned", true, false, Change{Kind: ChangeCreated, AssigneeTo: assignee}, false},
		{"public message, unassigned", false, false, Change{Kind: ChangeMessage}, true},
		{"public message, assigned", true, false, Change{Kind: ChangeMessage}, false},
		{"internal note, unassigned", false, false, Change{Kind: ChangeMessage, Internal: true}, false},
		{"move, unassigned", false, false, Change{Kind: ChangeStatus}, true},
		{"move, assigned", true, false, Change{Kind: ChangeStatus}, false},
		{"priority, unassigned", false, false, Change{Kind: ChangePriority}, false},
		{"SLA at risk, unassigned", false, false, Change{Kind: ChangeSLAAtRisk}, false},
		{"code, unassigned", false, false, Change{Kind: ChangeCode}, false},
		{"colleague added, unassigned", false, false, Change{Kind: ChangeWatcherAdded}, false},
	}
	for _, tt := range tests {
		in := Input{NotifySupervisorOnBreach: tt.breach, Changes: []Change{tt.change}}
		if tt.assigned {
			in.Assignee = &agent
		}
		if got := in.NeedsSupervisors(); got != tt.want {
			t.Errorf("%s: NeedsSupervisors() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// A question for the client is always on and always urgent: the ticket waits for their answer,
// so neither a switched-off preference nor a summary setting holds it back.
func TestQuestionIsAlwaysOnAndUrgent(t *testing.T) {
	in := base()
	in.Actor, in.ActorStaff = &agent, true
	in.Prefs = map[uuid.UUID]map[Event]Pref{requester: {EventQuestion: {}}}
	in.Changes = []Change{{AuditID: auditA, Kind: ChangeStatus, StatusFrom: ticket.StatusInProgress, StatusTo: ticket.StatusWaitingOnClient}}
	ds, _ := Route(in)
	var found bool
	for _, d := range ds {
		if d.UserID != requester {
			continue
		}
		found = true
		if d.Event != EventQuestion || d.Channels != webAndEmail || !d.Urgent {
			t.Errorf("question = %+v, want web and email, urgent", d)
		}
	}
	if !found {
		t.Fatal("the requester wasn't asked")
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

// UAT items go right away whatever the user's summary setting; staff reasons and causes never
// reach the client side.
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
		if (d.Change.Cause != "") != staff {
			t.Errorf("%s (staff %v) cause = %q", d.UserID, staff, d.Change.Cause)
		}
	}
	in.Changes = []Change{{AuditID: auditA, Kind: ChangeStatus, StatusFrom: ticket.StatusResolved,
		StatusTo: ticket.StatusInProgress, Cause: ticket.CauseReopened}}
	in.Actor, in.ActorStaff = nil, false
	ds, _ = Route(in)
	told := false
	for _, d := range ds {
		if d.UserID == requester {
			told = true
			if d.Change.Cause != ticket.CauseReopened {
				t.Errorf("the client's own cause = %q, want %q", d.Change.Cause, ticket.CauseReopened)
			}
		}
	}
	if !told {
		t.Error("the requester isn't told about the reopened ticket")
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
