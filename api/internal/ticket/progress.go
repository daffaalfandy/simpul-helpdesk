package ticket

import "time"

// Step is one stage of the requester's progress tracker (HANDOFF §4, PortalTicket).
type Step string

const (
	StepReceived     Step = "received"
	StepAssigned     Step = "assigned"
	StepInProgress   Step = "in_progress"
	StepReadyForTest Step = "ready_for_test"
	StepResolved     Step = "resolved"
)

// Steps lists the tracker steps in order.
var Steps = []Step{StepReceived, StepAssigned, StepInProgress, StepReadyForTest, StepResolved}

// StepState is how far a step has come.
type StepState string

const (
	StateDone     StepState = "done"
	StateCurrent  StepState = "current"
	StateUpcoming StepState = "upcoming"
	StateSkipped  StepState = "skipped" // passed without the ticket ever being in it
)

// StatusChange is one entry in a ticket's status history.
type StatusChange struct {
	To Status
	At time.Time
}

// ProgressStep is a tracker step as the requester sees it.
type ProgressStep struct {
	Step  Step
	State StepState
	At    *time.Time // when the ticket last entered the step; nil if it never did
}

// StepOf maps a status to the tracker step that shows it. Waiting on the client is still
// "in progress" from the requester's point of view.
func StepOf(s Status) Step {
	switch s {
	case StatusAssigned:
		return StepAssigned
	case StatusInProgress, StatusWaitingOnClient:
		return StepInProgress
	case StatusInUAT:
		return StepReadyForTest
	case StatusResolved, StatusClosed:
		return StepResolved
	}
	return StepReceived
}

// Progress builds the tracker from the status history (oldest first, creation included). Earlier
// steps are done or skipped, later ones upcoming; a reopen makes later steps upcoming again.
func Progress(current Status, createdAt time.Time, history []StatusChange) []ProgressStep {
	entered := map[Step]time.Time{StepReceived: createdAt}
	for _, c := range history {
		entered[StepOf(c.To)] = c.At
	}
	cur := StepOf(current)
	out := make([]ProgressStep, 0, len(Steps))
	past := true
	for _, step := range Steps {
		ps := ProgressStep{Step: step}
		if at, ok := entered[step]; ok && past {
			ps.At = &at
		}
		switch {
		case step == cur && cur == StepResolved:
			ps.State = StateDone
		case step == cur:
			ps.State = StateCurrent
		case !past:
			ps.State = StateUpcoming
		case ps.At != nil:
			ps.State = StateDone
		default:
			ps.State = StateSkipped
		}
		if step == cur {
			past = false
		}
		out = append(out, ps)
	}
	return out
}
