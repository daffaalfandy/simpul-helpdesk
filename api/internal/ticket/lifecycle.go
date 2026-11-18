package ticket

import (
	"errors"
	"slices"
	"time"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
)

var (
	ErrUnknownStatus   = errors.New("unknown status")
	ErrMoveNotAllowed  = errors.New("this status change is not allowed")
	ErrTicketClosed    = errors.New("the ticket is closed")
	ErrNeedsAssignee   = errors.New("assign someone first")
	ErrNeedsNoAssignee = errors.New("unassign the ticket first")
	ErrNotInUAT        = errors.New("the ticket is not waiting for a test")
	ErrNotReopenable   = errors.New("the ticket can no longer be reopened")
)

// manualMoves are an agent's hand moves (HANDOFF §6). Closed is system-only; the client's
// answers and reopen have their own checks below. CheckManualMove adds the assignee rules for
// new and assigned.
var manualMoves = map[Status][]Status{
	StatusNew:             {StatusInProgress, StatusWaitingOnClient, StatusResolved},
	StatusAssigned:        {StatusInProgress, StatusWaitingOnClient, StatusResolved},
	StatusInProgress:      {StatusNew, StatusAssigned, StatusWaitingOnClient, StatusInUAT, StatusResolved},
	StatusWaitingOnClient: {StatusNew, StatusAssigned, StatusInProgress, StatusInUAT, StatusResolved},
	StatusInUAT:           {StatusInProgress, StatusWaitingOnClient, StatusResolved},
	StatusResolved:        {StatusInProgress}, // an agent reopens the ticket
	StatusClosed:          {},
}

// CheckManualMove reports whether an agent may move a ticket from one status to another.
// Moving to the current status is not a move; callers treat it as a no-op first.
func CheckManualMove(from, to Status, hasAssignee bool) error {
	switch {
	case from.Locked():
		return ErrTicketClosed
	case !from.Valid() || !to.Valid():
		return ErrUnknownStatus
	case !slices.Contains(manualMoves[from], to):
		return ErrMoveNotAllowed
	case to == StatusNew && hasAssignee:
		return ErrNeedsNoAssignee
	case to == StatusAssigned && !hasAssignee:
		return ErrNeedsAssignee
	}
	return nil
}

// AllowedMoves lists the statuses an agent may move the ticket to, in lifecycle order.
func AllowedMoves(from Status, hasAssignee bool) []Status {
	out := []Status{}
	for _, to := range Statuses {
		if CheckManualMove(from, to, hasAssignee) == nil {
			out = append(out, to)
		}
	}
	return out
}

// StatusAfterAssignment is the status once the assignee changes: assigning a new ticket
// makes it assigned and unassigning an assigned ticket makes it new again.
func StatusAfterAssignment(cur Status, hasAssignee bool) Status {
	switch {
	case cur == StatusNew && hasAssignee:
		return StatusAssigned
	case cur == StatusAssigned && !hasAssignee:
		return StatusNew
	}
	return cur
}

// StatusAfterRequesterReply is the status once the requester posts a public reply. A reply
// while the desk waits on the client hands the ticket back to the desk: to its assignee, or to
// the New queue when nobody is on it.
func StatusAfterRequesterReply(cur Status, hasAssignee bool) (Status, bool) {
	if cur == StatusWaitingOnClient {
		return backToWork(hasAssignee), true
	}
	return cur, false
}

// MarksResolved reports whether a ticket in status s counts as resolved.
func MarksResolved(s Status) bool { return slices.Contains(DoneStatuses, s) }

// ReopenWorkingDays is how long the client can reopen a resolved ticket; then it closes.
const ReopenWorkingDays = 3

// ReopenUntil is the end of the reopen window for a ticket resolved at resolvedAt. The ticket
// can be reopened up to and including that instant and closes after it.
func ReopenUntil(resolvedAt time.Time, s sla.Schedules) time.Time {
	return s.AddWorkingDays(resolvedAt, ReopenWorkingDays)
}

// UATAnswer is the client's answer to a test. Values match the UatAnswer enum in the API.
type UATAnswer string

const (
	UATApprove UATAnswer = "approve" // it works
	UATReject  UATAnswer = "reject"  // still a problem
)

func (a UATAnswer) Valid() bool { return a == UATApprove || a == UATReject }

// CheckUATAnswer reports whether the client can answer a test on a ticket in status s.
func CheckUATAnswer(s Status) error {
	switch {
	case s.Locked():
		return ErrTicketClosed
	case s != StatusInUAT:
		return ErrNotInUAT
	}
	return nil
}

// CheckReopen reports whether the client can reopen a ticket in status s whose reopen window
// ends at until.
func CheckReopen(s Status, until, now time.Time) error {
	switch {
	case s.Locked():
		return ErrTicketClosed
	case s != StatusResolved || now.After(until):
		return ErrNotReopenable
	}
	return nil
}

// StatusAfterUATAnswer is where the client's answer takes the ticket: approval resolves it
// (HANDOFF §6), a rejection hands it back to its assignee, or to the New queue without one.
func StatusAfterUATAnswer(a UATAnswer, hasAssignee bool) Status {
	if a == UATApprove {
		return StatusResolved
	}
	return backToWork(hasAssignee)
}

// StatusAfterReopen is where a reopened ticket goes: back to its assignee with the same
// priority, or to the New queue without one, so someone picks it up.
func StatusAfterReopen(hasAssignee bool) Status { return backToWork(hasAssignee) }

func backToWork(hasAssignee bool) Status {
	if hasAssignee {
		return StatusInProgress
	}
	return StatusNew
}
