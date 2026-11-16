// Package codeflow decides what code activity does to one ticket (HANDOFF §8): link it, start
// the work, tick Definition of Done items, move the ticket on once its code conditions are met,
// or reopen it. It is pure and replays from the ticket's current state, so running it again on
// the same events changes nothing.
package codeflow

import (
	"slices"
	"time"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/commitmsg"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/gitevent"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// Outcome is what an event did to a ticket, as the recent-events log tells it. Values match the
// code_event_links.outcome CHECK.
type Outcome string

const (
	OutcomeLinked          Outcome = "linked"           // linked to the ticket, nothing else
	OutcomeInReview        Outcome = "in_review"        // a pull/merge request naming the ticket opened
	OutcomeFixCommitted    Outcome = "fix_committed"    // "Fix committed" ticked
	OutcomeMerged          Outcome = "merged"           // "merged" ticked
	OutcomeCIPassed        Outcome = "ci_passed"        // "CI passed" ticked
	OutcomeDeployed        Outcome = "deployed"         // "Deployed" ticked
	OutcomeReopened        Outcome = "reopened"         // the ticket went back to work
	OutcomeTicketClosed    Outcome = "ticket_closed"    // the ticket is closed; ignored
	OutcomeUnknownTicket   Outcome = "unknown_ticket"   // no ticket has the key; ignored
	OutcomeOtherRepository Outcome = "other_repository" // the ticket's project uses another repository; ignored
)

// Outcomes lists every outcome.
var Outcomes = []Outcome{
	OutcomeLinked, OutcomeInReview, OutcomeFixCommitted, OutcomeMerged, OutcomeCIPassed, OutcomeDeployed,
	OutcomeReopened, OutcomeTicketClosed, OutcomeUnknownTicket, OutcomeOtherRepository,
}

// strength orders outcomes when one event does several things: the log names the strongest.
var strength = map[Outcome]int{
	OutcomeLinked: 0, OutcomeInReview: 1, OutcomeFixCommitted: 2, OutcomeMerged: 3,
	OutcomeCIPassed: 4, OutcomeDeployed: 5, OutcomeReopened: 6,
}

// advanceFrom are the statuses a ticket moves on from once its code conditions are met. A ticket
// already In UAT or resolved stays where it is.
var advanceFrom = []ticket.Status{ticket.StatusNew, ticket.StatusAssigned, ticket.StatusInProgress, ticket.StatusWaitingOnClient}

// reopenFrom are the statuses a reopen commit takes back to work. Closed tickets stay closed.
var reopenFrom = []ticket.Status{ticket.StatusInUAT, ticket.StatusResolved}

// Event is one code event as it reaches the ticket.
type Event struct {
	Kind   gitevent.Kind
	Action commitmsg.Action // what the text asks of this ticket; empty for CI runs and deployments
	Branch string           // pushed branch, request source branch, or where CI ran or deployed
	Target string           // the branch a pull/merge request merges into
	At     time.Time
	// New is true the first time the event reaches the ticket; seen again (a commit pushed to
	// another branch, a redelivery) it only counts if it changes something.
	New bool
	// AuthorCanTake says the author is an active staff member who can work the ticket's client,
	// so the event may assign an unassigned ticket to them.
	AuthorCanTake bool
}

// Input is the ticket's state and the events reaching it, in time order.
type Input struct {
	Status      ticket.Status
	HasAssignee bool
	DoD         dod.Config
	Done        map[dod.Item]time.Time // items already met, with the merge's own time for merged
	LinkedAt    time.Time              // when code first reached the ticket; zero if it never did
	Events      []Event
}

// StepKind is what a step changes.
type StepKind string

const (
	StepAssign StepKind = "assign" // the event's author takes the ticket
	StepTick   StepKind = "tick"   // a Definition of Done item is met
	StepMove   StepKind = "move"   // the status changes
)

// Step is one change, applied in order.
type Step struct {
	Kind  StepKind
	Event int // index into Input.Events of the event that caused it
	Item  dod.Item
	From  ticket.Status
	To    ticket.Status
	Cause ticket.Cause
}

// Result is what one event did, for the recent-events log and the ticket's code activity.
type Result struct {
	Event   int
	Outcome Outcome
	Items   []dod.Item    // items it ticked
	From    ticket.Status // the status before, when it moved the ticket
	To      ticket.Status
}

// Plan is everything the events do to the ticket. Both are empty when nothing changes.
type Plan struct {
	Steps   []Step
	Results []Result
}

type state struct {
	in          Input
	status      ticket.Status
	hasAssignee bool
	done        map[dod.Item]time.Time
	linkedAt    time.Time
	plan        Plan
}

// Decide plans what the events do to the ticket.
func Decide(in Input) Plan {
	s := &state{in: in, status: in.Status, hasAssignee: in.HasAssignee, done: map[dod.Item]time.Time{}, linkedAt: in.LinkedAt}
	for it, at := range in.Done {
		s.done[it] = at
	}
	for i, e := range in.Events {
		s.apply(i, e)
	}
	return s.plan
}

func (s *state) apply(i int, e Event) {
	r := Result{Event: i}
	if s.status.Locked() {
		if e.New && e.Action != "" {
			r.Outcome = OutcomeTicketClosed
			s.plan.Results = append(s.plan.Results, r)
		}
		return
	}
	from := s.status
	named := e.Action != "" // a commit or request that names the ticket
	if named && (s.linkedAt.IsZero() || e.At.Before(s.linkedAt)) {
		s.linkedAt = e.At
	}
	if named && e.New {
		r.Outcome = OutcomeLinked
		if e.Kind == gitevent.KindPROpened {
			r.Outcome = OutcomeInReview
		}
	}

	// Moves come from new events only: a redelivered commit must not undo an agent's later move.
	switch {
	case !e.New:
	case e.Action == commitmsg.ActionReopen:
		if slices.Contains(reopenFrom, s.status) {
			s.move(i, ticket.StatusAfterReopen(s.hasAssignee), ticket.CauseCodeReopened)
			r.Outcome = OutcomeReopened
		}
	case named && e.Kind != gitevent.KindPRClosed:
		s.startWork(i, e)
	}

	if e.Action == commitmsg.ActionFix && s.tick(i, dod.ItemFixCommitted, e.At) {
		r = withItem(r, dod.ItemFixCommitted, OutcomeFixCommitted)
	}
	if s.mergesHere(e) && s.tick(i, dod.ItemMerged, e.At) {
		r = withItem(r, dod.ItemMerged, OutcomeMerged)
	}
	if e.Kind == gitevent.KindPipeline && s.in.DoD.CIPassed && s.in.DoD.Merged.Branch.Matches(e.Branch) &&
		s.afterCode(e.At) && s.tick(i, dod.ItemCIPassed, e.At) {
		r = withItem(r, dod.ItemCIPassed, OutcomeCIPassed)
	}
	if e.Kind == gitevent.KindDeployment && s.in.DoD.DeployedProduction && s.afterCode(e.At) &&
		s.tick(i, dod.ItemDeployed, e.At) {
		r = withItem(r, dod.ItemDeployed, OutcomeDeployed)
	}
	s.advance(i)

	if s.status != from {
		r.From, r.To = from, s.status
		if r.Outcome == "" {
			r.Outcome = OutcomeLinked
		}
	}
	if r.Outcome != "" {
		s.plan.Results = append(s.plan.Results, r)
	}
}

// startWork moves a new or assigned ticket to In progress. An unassigned ticket goes to the
// event's author when they can take it, and otherwise stays New.
func (s *state) startWork(i int, e Event) {
	if s.status == ticket.StatusNew && !s.hasAssignee {
		if !e.AuthorCanTake {
			return
		}
		s.hasAssignee = true
		s.plan.Steps = append(s.plan.Steps, Step{Kind: StepAssign, Event: i})
		s.status = ticket.StatusAfterAssignment(s.status, true)
	}
	if s.status == ticket.StatusNew || s.status == ticket.StatusAssigned {
		s.move(i, ticket.StatusInProgress, ticket.CauseCodeActivity)
	}
}

// mergesHere reports whether the event puts the ticket's code on its Definition of Done branch:
// a pull/merge request merged into it, or a commit pushed straight onto it (owner, 16 Nov).
func (s *state) mergesHere(e Event) bool {
	m := s.in.DoD.Merged
	switch {
	case !m.Enabled || e.Action == "" || e.Action == commitmsg.ActionReopen:
		return false // a reopen commit says the fix failed, not that one landed
	case e.Kind == gitevent.KindPRMerged:
		return m.Branch.Matches(e.Target)
	case e.Kind == gitevent.KindCommit:
		return m.Branch.Matches(e.Branch)
	}
	return false
}

// afterCode reports whether a CI run or deployment at `at` includes the ticket's code: it came
// after the merge when the ticket needs one, else after code first reached the ticket.
func (s *state) afterCode(at time.Time) bool {
	anchor := s.linkedAt
	if s.in.DoD.Merged.Enabled {
		merged, ok := s.done[dod.ItemMerged]
		if !ok {
			return false
		}
		anchor = merged
	}
	return !anchor.IsZero() && !at.Before(anchor)
}

// advance moves the ticket on once every code condition it needs is met: to In UAT when the
// client checks the work, else to resolved.
func (s *state) advance(i int) {
	if !slices.Contains(advanceFrom, s.status) {
		return
	}
	needsCode := false
	for _, it := range dod.Required(s.in.DoD) {
		if !slices.Contains(dod.CodeItems, it) {
			continue // fix committed is never required; client items wait In UAT
		}
		if _, ok := s.done[it]; !ok {
			return
		}
		needsCode = true
	}
	if !needsCode {
		return // nothing to wait for from the repository: an agent moves the ticket by hand
	}
	to := ticket.StatusResolved
	if dod.HasClientCheck(s.in.DoD) {
		to = ticket.StatusInUAT
	}
	s.move(i, to, ticket.CauseCodeDone)
}

func (s *state) tick(i int, it dod.Item, at time.Time) bool {
	if _, ok := s.done[it]; ok {
		return false
	}
	s.done[it] = at
	s.plan.Steps = append(s.plan.Steps, Step{Kind: StepTick, Event: i, Item: it})
	return true
}

// move changes the status and forgets the items the move clears (dod.Cleared), as storing the
// move does.
func (s *state) move(i int, to ticket.Status, cause ticket.Cause) {
	if to == s.status {
		return
	}
	s.plan.Steps = append(s.plan.Steps, Step{Kind: StepMove, Event: i, From: s.status, To: to, Cause: cause})
	for _, it := range dod.Cleared(s.status, to) {
		delete(s.done, it)
	}
	s.status = to
}

func withItem(r Result, it dod.Item, o Outcome) Result {
	r.Items = append(r.Items, it)
	if strength[o] > strength[r.Outcome] || r.Outcome == "" {
		r.Outcome = o
	}
	return r
}
