package ticket

import (
	"errors"
	"slices"
)

// Lifecycle errors.
var (
	ErrUnknownStatus   = errors.New("unknown status")
	ErrMoveNotAllowed  = errors.New("this status change is not allowed")
	ErrTicketClosed    = errors.New("the ticket is closed")
	ErrNeedsAssignee   = errors.New("assign someone first")
	ErrNeedsNoAssignee = errors.New("unassign the ticket first")
)

// manualMoves are the status changes an agent may make by hand (HANDOFF §6). Closed is
// reached only by the system (auto-close, M6); UAT approval and requester reopen are M6.
// Moves to new require no assignee and moves to assigned require one; see CheckManualMove.
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
// while the desk waits on the client hands the ticket back to the desk.
func StatusAfterRequesterReply(cur Status) (Status, bool) {
	if cur == StatusWaitingOnClient {
		return StatusInProgress, true
	}
	return cur, false
}

// MarksResolved reports whether a ticket in status s counts as resolved.
func MarksResolved(s Status) bool { return slices.Contains(DoneStatuses, s) }
