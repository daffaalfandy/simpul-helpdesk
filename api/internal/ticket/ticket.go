// Package ticket is the pure ticket domain: types, lifecycle, progress tracker, keys and SLA
// snapshot. It has no database access, so later packages (DoD, commit parsing) can share it.
package ticket

import "slices"

// Type says whether a ticket reports a fault or asks for something new.
// Values match the tickets.type CHECK constraint.
type Type string

// Ticket types (HANDOFF §2).
const (
	TypeIncident Type = "incident" // something broke
	TypeRequest  Type = "request"  // service request: needs something new
)

var Types = []Type{TypeIncident, TypeRequest}

func (t Type) Valid() bool { return slices.Contains(Types, t) }

// Status is where a ticket is in its lifecycle. Values match the tickets.status CHECK.
type Status string

// Statuses (HANDOFF §6).
const (
	StatusNew             Status = "new"
	StatusAssigned        Status = "assigned"
	StatusInProgress      Status = "in_progress"
	StatusWaitingOnClient Status = "waiting_on_client"
	StatusInUAT           Status = "in_uat"
	StatusResolved        Status = "resolved"
	StatusClosed          Status = "closed"
)

// Statuses lists every status in lifecycle order.
var Statuses = []Status{
	StatusNew, StatusAssigned, StatusInProgress, StatusWaitingOnClient, StatusInUAT, StatusResolved, StatusClosed,
}

func (s Status) Valid() bool { return slices.Contains(Statuses, s) }

// IsOpen reports whether work on the ticket is still going on.
func (s Status) IsOpen() bool { return s.Valid() && !slices.Contains(DoneStatuses, s) }

// Locked reports whether the ticket takes no more changes or messages.
func (s Status) Locked() bool { return s == StatusClosed }

// DoneStatuses are the statuses after work has finished.
var DoneStatuses = []Status{StatusResolved, StatusClosed}

// OpenStatuses are the statuses before work has finished.
var OpenStatuses = []Status{StatusNew, StatusAssigned, StatusInProgress, StatusWaitingOnClient, StatusInUAT}

// Channel is how a ticket reached the service desk. Values match the tickets.channel CHECK.
type Channel string

const (
	ChannelPortal   Channel = "portal"
	ChannelPhone    Channel = "phone"
	ChannelEmail    Channel = "email"
	ChannelWhatsApp Channel = "whatsapp"
	ChannelInPerson Channel = "in_person"
)

var Channels = []Channel{ChannelPortal, ChannelPhone, ChannelEmail, ChannelWhatsApp, ChannelInPerson}

func (c Channel) Valid() bool { return slices.Contains(Channels, c) }

// AgentLoggable reports whether an agent may log a ticket that arrived this way. Portal
// tickets come only from requesters themselves.
func (c Channel) AgentLoggable() bool { return c.Valid() && c != ChannelPortal }

// Visibility says who can read a message. Values match the ticket_messages.visibility CHECK.
type Visibility string

const (
	VisibilityPublic   Visibility = "public"   // requester and staff
	VisibilityInternal Visibility = "internal" // staff only; never reaches requesters
)

func (v Visibility) Valid() bool { return v == VisibilityPublic || v == VisibilityInternal }

// Cause explains a status change the system made on its own.
type Cause string

const (
	CauseAssignment     Cause = "assignment"      // assigning or unassigning moved new ↔ assigned
	CauseRequesterReply Cause = "requester_reply" // the requester answered while we waited on them
)
