// Package notification decides who hears about a ticket change, on which channels, and when
// their email goes out (HANDOFF §9). It is pure: no database, no clock, no templates.
package notification

import "slices"

// Event is a kind of notification a user can switch per channel. Values match the
// notifications.event_type and notification_preferences.event_type CHECK constraints.
type Event string

// Team events (vendor staff).
const (
	EventComment      Event = "comment"       // public replies from the client or teammates
	EventInternalNote Event = "internal_note" // staff-only notes; never reach requesters
	EventStatus       Event = "status"        // also a portal event
	EventAssignment   Event = "assignment"
	EventPriority     Event = "priority"
	EventSLA          Event = "sla"
	EventUATResult    Event = "uat_result" // the client approved or rejected a test, or it was approved automatically
	EventDoD          Event = "dod"        // supervisors: resolved without the DoD, or a ticket's own DoD
	EventCode         Event = "code"       // commits, pull/merge requests, CI and deploys naming the ticket
	EventNewTicket    Event = "new_ticket" // supervisors: a ticket nobody is assigned to was reported
)

// Portal events (client staff).
const (
	EventReply        Event = "reply"         // public replies from the support team
	EventQuestion     Event = "question"      // the team moved the ticket to Waiting on client; always on
	EventResolved     Event = "resolved"      // resolved or closed
	EventColleague    Event = "colleague"     // a colleague's change on a ticket you follow
	EventTicketLogged Event = "ticket_logged" // an agent logged a ticket for you
	EventUATRequested Event = "uat_requested" // a fix is ready for the client's test
)

// Events lists every event; the database CHECK constraints hold the same set.
var Events = []Event{
	EventComment, EventInternalNote, EventStatus, EventAssignment, EventPriority, EventSLA, EventUATResult, EventDoD,
	EventCode, EventNewTicket, EventReply, EventQuestion, EventResolved, EventColleague, EventTicketLogged, EventUATRequested,
}

func (e Event) Valid() bool { return slices.Contains(Events, e) }

// Audience is who a user is to the service desk: vendor staff or client staff.
type Audience string

const (
	AudienceTeam   Audience = "team"
	AudiencePortal Audience = "portal"
)

// AudienceFor picks the audience from whether the user is vendor staff.
func AudienceFor(staff bool) Audience {
	if staff {
		return AudienceTeam
	}
	return AudiencePortal
}

// Pref is a user's choice of channels for one event.
type Pref struct {
	Web   bool
	Email bool
}

// Any reports whether at least one channel is on.
func (p Pref) Any() bool { return p.Web || p.Email }

var (
	webAndEmail = Pref{Web: true, Email: true}
	webOnly     = Pref{Web: true}
)

// eventSpec is one event as an audience sees it in Settings.
type eventSpec struct {
	event           Event
	defaults        Pref
	locked          bool // always on, both channels; the API refuses to switch it off
	hidden          bool // always on and not listed in Settings
	supervisorsOnly bool // listed, and switchable, only for the organization's supervisors
}

// audienceEvents holds each audience's events in Settings order, with the defaults from the
// Notifications artboard. Settings lists only events the build sends. Supervisors' Definition
// of Done alerts are always on, like SLA breaches, and have no row of their own. A question
// for the client (Waiting on client) is always on: the ticket waits for their answer.
var audienceEvents = map[Audience][]eventSpec{
	AudienceTeam: {
		{event: EventComment, defaults: webAndEmail},
		{event: EventInternalNote, defaults: webOnly},
		{event: EventStatus, defaults: webAndEmail},
		{event: EventAssignment, defaults: webAndEmail, locked: true},
		{event: EventNewTicket, defaults: webAndEmail, supervisorsOnly: true},
		{event: EventPriority, defaults: webOnly},
		{event: EventSLA, defaults: webAndEmail, locked: true},
		{event: EventUATResult, defaults: webAndEmail},
		{event: EventCode, defaults: webOnly},
		{event: EventDoD, defaults: webAndEmail, locked: true, hidden: true},
	},
	AudiencePortal: {
		{event: EventReply, defaults: webAndEmail},
		{event: EventStatus, defaults: webAndEmail},
		{event: EventUATRequested, defaults: webAndEmail, locked: true},
		{event: EventQuestion, defaults: webAndEmail, locked: true},
		{event: EventResolved, defaults: webAndEmail},
		{event: EventColleague, defaults: webOnly},
		{event: EventTicketLogged, defaults: webAndEmail, locked: true, hidden: true},
	},
}

func spec(a Audience, e Event) (eventSpec, bool) {
	for _, s := range audienceEvents[a] {
		if s.event == e {
			return s, true
		}
	}
	return eventSpec{}, false
}

// Setting is one row of a user's notification settings.
type Setting struct {
	Event  Event
	Pref   Pref
	Locked bool
}

// listed reports whether a user sees the event in Settings: hidden events never, supervisor-only
// events only for supervisors.
func (s eventSpec) listed(supervisor bool) bool {
	return !s.hidden && (supervisor || !s.supervisorsOnly)
}

// Settings lists the events an audience can switch, in order, with the user's choices applied.
// supervisor is whether the user supervises the organization they work in.
func Settings(a Audience, supervisor bool, overrides map[Event]Pref) []Setting {
	out := []Setting{}
	for _, s := range audienceEvents[a] {
		if !s.listed(supervisor) {
			continue
		}
		out = append(out, Setting{Event: s.event, Pref: Channels(a, s.event, overrides), Locked: s.locked})
	}
	return out
}

// Channels is where a user gets an event: locked events always on both channels, an event
// outside the audience nowhere, otherwise the user's choice or the default.
func Channels(a Audience, e Event, overrides map[Event]Pref) Pref {
	s, ok := spec(a, e)
	switch {
	case !ok:
		return Pref{}
	case s.locked:
		return webAndEmail
	}
	if p, ok := overrides[e]; ok {
		return p
	}
	return s.defaults
}

// Switchable reports whether a user of the audience may change the event (only events their
// Settings list), and whether the requested channels are allowed (locked events can't be
// switched off).
func Switchable(a Audience, supervisor bool, e Event, p Pref) (known, allowed bool) {
	s, ok := spec(a, e)
	if !ok || !s.listed(supervisor) {
		return false, false
	}
	return true, !s.locked || p == webAndEmail
}

// Filter narrows the inbox.
type Filter string

const (
	FilterAll      Filter = "all"
	FilterUnread   Filter = "unread"
	FilterComments Filter = "comments" // team: replies and internal notes
	FilterStatus   Filter = "status"
	FilterAssigned Filter = "assigned" // team: assignments and new tickets nobody is assigned to
	FilterSLA      Filter = "sla"
	FilterReplies  Filter = "replies" // portal: replies and questions
	FilterCode     Filter = "code"    // team: code activity
)

// Filters lists every inbox filter.
var Filters = []Filter{FilterAll, FilterUnread, FilterComments, FilterStatus, FilterAssigned, FilterSLA, FilterReplies, FilterCode}

func (f Filter) Valid() bool { return slices.Contains(Filters, f) }

// filterEvents are the events each type filter shows. All and Unread show every event.
var filterEvents = map[Filter][]Event{
	FilterComments: {EventComment, EventInternalNote},
	FilterStatus:   {EventStatus, EventResolved, EventUATRequested, EventUATResult, EventDoD},
	FilterAssigned: {EventAssignment, EventNewTicket},
	FilterSLA:      {EventSLA},
	FilterReplies:  {EventReply, EventQuestion},
	FilterCode:     {EventCode},
}

// FilterEvents returns the events a filter shows, or nil for every event.
func FilterEvents(f Filter) []Event { return filterEvents[f] }
