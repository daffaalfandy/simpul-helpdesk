package notification

import (
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// ChangeKind is what one audit entry of a ticket transaction did.
type ChangeKind string

const (
	ChangeCreated        ChangeKind = "created"
	ChangeMessage        ChangeKind = "message"
	ChangeStatus         ChangeKind = "status"
	ChangeAssignee       ChangeKind = "assignee"
	ChangePriority       ChangeKind = "priority"
	ChangeWatcherAdded   ChangeKind = "watcher_added"
	ChangeWatcherRemoved ChangeKind = "watcher_removed"
	ChangeSLAAtRisk      ChangeKind = "sla_at_risk"
	ChangeSLABreached    ChangeKind = "sla_breached"
	ChangeDoD            ChangeKind = "dod" // an agent gave the ticket its own Definition of Done
)

// ChangeKinds lists every change kind.
var ChangeKinds = []ChangeKind{
	ChangeCreated, ChangeMessage, ChangeStatus, ChangeAssignee, ChangePriority,
	ChangeWatcherAdded, ChangeWatcherRemoved, ChangeSLAAtRisk, ChangeSLABreached, ChangeDoD,
}

// Kind tells apart notifications of one event, e.g. assigned and unassigned.
type Kind string

const (
	KindNone         Kind = ""
	KindAssigned     Kind = "assigned"
	KindUnassigned   Kind = "unassigned"
	KindReply        Kind = "reply"  // a colleague replied
	KindStatus       Kind = "status" // a colleague's change moved the ticket
	KindAdded        Kind = "added"  // a colleague added you to the ticket
	KindAtRisk       Kind = "at_risk"
	KindBreached     Kind = "breached"
	KindApproved     Kind = "approved"      // the client approved the test
	KindRejected     Kind = "rejected"      // the client sent the fix back
	KindAutoApproved Kind = "auto_approved" // nobody answered in time
	KindSkipped      Kind = "skipped"       // resolved without meeting the Definition of Done
	KindOverride     Kind = "override"      // the ticket got a Definition of Done of its own
)

// Kinds lists every kind a notification can carry (KindNone is stored as no kind).
var Kinds = []Kind{
	KindAssigned, KindUnassigned, KindReply, KindStatus, KindAdded, KindAtRisk, KindBreached,
	KindApproved, KindRejected, KindAutoApproved, KindSkipped, KindOverride,
}

// uatKinds name the client's answer behind a status change.
var uatKinds = map[ticket.Cause]Kind{
	ticket.CauseUATApproved:     KindApproved,
	ticket.CauseUATRejected:     KindRejected,
	ticket.CauseUATAutoApproved: KindAutoApproved,
}

// urgentEvents always email right away, whatever the user's summary setting (HANDOFF §9: P1,
// SLA and UAT items).
var urgentEvents = []Event{EventSLA, EventUATRequested, EventUATResult}

// Person is a user named in a change.
type Person struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// Change is one audit entry of a ticket transaction, in the form the router needs.
type Change struct {
	AuditID     uuid.UUID
	Kind        ChangeKind
	StaffLogged bool // created: an agent logged the ticket for the requester
	MessageID   *uuid.UUID
	Internal    bool // message: an internal note
	StatusFrom  ticket.Status
	StatusTo    ticket.Status
	Priority    *PriorityChange
	Reason      string
	AssigneeTo  *Person
	AssigneeOut *Person // the previous assignee
	WatcherID   *uuid.UUID
	SLAClock    string // which SLA clock: first response or resolution
	SLADueAt    *time.Time
	SLALeft     *time.Duration // time left when the clock turned at risk
	Cause       ticket.Cause   // status: why it moved, when it wasn't a plain move by hand
	At          time.Time      // when the change happened
	// AutoApproveAt (a move In UAT) and ReopenUntil (a move to resolved) are the client's
	// deadlines as of the change; the fan-out fills them in.
	AutoApproveAt *time.Time
	ReopenUntil   *time.Time
}

// PriorityChange is a priority's old and new value.
type PriorityChange struct {
	From sla.Priority `json:"from"`
	To   sla.Priority `json:"to"`
}

// Recipient is what the router needs to know about a user it may notify.
type Recipient struct {
	Staff     bool
	Suspended bool
}

// Input is one ticket transaction and everyone it may concern.
type Input struct {
	Actor       *uuid.UUID // nil when the system made the change
	ActorStaff  bool
	Assignee    *uuid.UUID // after the transaction
	Requester   uuid.UUID
	Watchers    []uuid.UUID
	Supervisors []uuid.UUID // the active supervisors, filled only when a change concerns them
	// NotifySupervisorOnBreach is the ticket's project switch for SLA breach alerts.
	NotifySupervisorOnBreach bool
	Priority                 sla.Priority
	Changes                  []Change
	Users                    map[uuid.UUID]Recipient
	Prefs                    map[uuid.UUID]map[Event]Pref
}

// Delivery is the notification one user gets for a transaction: its main change, the other
// changes it folds in ("Also changed"), and where it goes.
type Delivery struct {
	UserID   uuid.UUID
	Event    Event
	Kind     Kind
	Change   Change
	Also     []Change
	Channels Pref
	Urgent   bool
}

// precedence orders events when one transaction gives a user several: the first wins.
var precedence = []Event{
	EventSLA, EventTicketLogged, EventUATRequested, EventUATResult, EventQuestion, EventResolved, EventDoD,
	EventAssignment, EventReply, EventComment, EventInternalNote, EventStatus, EventPriority, EventColleague,
}

type candidate struct {
	user   uuid.UUID
	event  Event
	kind   Kind
	change Change
}

// Route decides who hears about a transaction. The actor and suspended users never do, and
// internal notes never reach client staff: any such delivery is dropped and counted, which
// would be a bug in the rules.
func Route(in Input) (deliveries []Delivery, dropped int) {
	var cands []candidate
	for _, c := range in.Changes {
		cands = append(cands, in.candidates(c)...)
	}
	byUser := map[uuid.UUID][]candidate{}
	var order []uuid.UUID
	for _, c := range cands {
		r, known := in.Users[c.user]
		if !known || r.Suspended || (in.Actor != nil && c.user == *in.Actor) {
			continue
		}
		if !r.Staff && (c.event == EventInternalNote || c.change.Internal) {
			dropped++
			continue
		}
		if !r.Staff {
			c.change.Reason = "" // staff reasons (a priority change, resolving without the DoD) stay with the team
		}
		if _, seen := byUser[c.user]; !seen {
			order = append(order, c.user)
		}
		byUser[c.user] = append(byUser[c.user], c)
	}
	for _, user := range order {
		if d, ok := in.deliver(user, byUser[user]); ok {
			deliveries = append(deliveries, d)
		}
	}
	return deliveries, dropped
}

// deliver folds one user's candidates into a delivery whose main change is the highest
// event they still get on some channel.
func (in Input) deliver(user uuid.UUID, cands []candidate) (Delivery, bool) {
	aud := AudienceFor(in.Users[user].Staff)
	slices.SortStableFunc(cands, func(a, b candidate) int {
		return slices.Index(precedence, a.event) - slices.Index(precedence, b.event)
	})
	for i, c := range cands {
		ch := Channels(aud, c.event, in.Prefs[user])
		if !ch.Any() {
			continue
		}
		d := Delivery{
			UserID: user, Event: c.event, Kind: c.kind, Change: c.change, Channels: ch,
			Urgent: in.Priority == sla.P1 || slices.Contains(urgentEvents, c.event),
		}
		for j, other := range cands {
			if j != i && other.change.AuditID != c.change.AuditID {
				d.Also = append(d.Also, other.change)
			}
		}
		return d, true
	}
	return Delivery{}, false
}

// candidates lists who one change may concern, before the actor, preferences and folding.
func (in Input) candidates(c Change) []candidate {
	var out []candidate
	add := func(user *uuid.UUID, e Event, k Kind) {
		if user != nil {
			out = append(out, candidate{user: *user, event: e, kind: k, change: c})
		}
	}
	clientSide := func(e Event, k Kind) {
		for _, u := range in.clientSide() {
			add(&u, e, k)
		}
	}
	switch c.Kind {
	case ChangeCreated:
		if c.StaffLogged {
			add(&in.Requester, EventTicketLogged, KindNone)
		}
	case ChangeMessage:
		switch {
		case c.Internal:
			add(in.Assignee, EventInternalNote, KindNone)
		case in.ActorStaff:
			add(in.Assignee, EventComment, KindNone)
			clientSide(EventReply, KindNone)
		default:
			add(in.Assignee, EventComment, KindNone)
			clientSide(EventColleague, KindReply)
		}
	case ChangeStatus:
		if k, ok := uatKinds[c.Cause]; ok {
			add(in.Assignee, EventUATResult, k)
		} else {
			add(in.Assignee, EventStatus, KindNone)
		}
		if c.Cause == ticket.CauseDoDSkipped {
			for _, s := range in.Supervisors {
				add(&s, EventDoD, KindSkipped)
			}
		}
		if in.Actor != nil && !in.ActorStaff {
			clientSide(EventColleague, KindStatus)
		} else {
			clientSide(portalStatusEvent(c.StatusTo), KindNone)
		}
	case ChangeAssignee:
		if c.AssigneeTo != nil {
			add(&c.AssigneeTo.ID, EventAssignment, KindAssigned)
		}
		if c.AssigneeOut != nil {
			add(&c.AssigneeOut.ID, EventAssignment, KindUnassigned)
		}
	case ChangePriority:
		add(in.Assignee, EventPriority, KindNone)
	case ChangeWatcherAdded:
		add(c.WatcherID, EventColleague, KindAdded)
	case ChangeSLAAtRisk:
		add(in.Assignee, EventSLA, KindAtRisk)
	case ChangeSLABreached:
		add(in.Assignee, EventSLA, KindBreached)
		if in.NotifySupervisorOnBreach {
			for _, s := range in.Supervisors {
				add(&s, EventSLA, KindBreached)
			}
		}
	case ChangeDoD:
		for _, s := range in.Supervisors {
			add(&s, EventDoD, KindOverride)
		}
	}
	return out
}

// clientSide is the requester and the colleagues following the ticket.
func (in Input) clientSide() []uuid.UUID {
	out := []uuid.UUID{in.Requester}
	for _, w := range in.Watchers {
		if !slices.Contains(out, w) {
			out = append(out, w)
		}
	}
	return out
}

// portalStatusEvent is the portal event for a move by the team or the system: a move to Waiting
// on client is a question for the requester (HANDOFF §13 Q8), a move In UAT asks for their test,
// and a move to Resolved or Closed is its own event.
func portalStatusEvent(to ticket.Status) Event {
	switch to {
	case ticket.StatusWaitingOnClient:
		return EventQuestion
	case ticket.StatusInUAT:
		return EventUATRequested
	case ticket.StatusResolved, ticket.StatusClosed:
		return EventResolved
	}
	return EventStatus
}

// Rank orders events by importance, most important first, so an email headlines the change
// that matters most.
func Rank(e Event) int { return slices.Index(precedence, e) }
