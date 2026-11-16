package codeflow

import (
	"reflect"
	"testing"
	"time"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/commitmsg"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/gitevent"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

var base = time.Date(2026, time.November, 16, 1, 0, 0, 0, time.UTC)

func at(min int) time.Time { return base.Add(time.Duration(min) * time.Minute) }

// Projects from the mock (HANDOFF §7).
var (
	eOffice = func() dod.Config { // merged into main, then the client tests
		c := dod.Default()
		c.Merged = dod.Merged{Enabled: true, Branch: dod.BranchMain}
		return c
	}()
	sipd = func() dod.Config { // merged into a release branch with CI, no client test
		c := dod.None()
		c.Merged = dod.Merged{Enabled: true, Branch: dod.BranchRelease}
		c.CIPassed = true
		return c
	}()
	website = func() dod.Config { // merged into main and deployed
		c := dod.None()
		c.Merged = dod.Merged{Enabled: true, Branch: dod.BranchMain}
		c.DeployedProduction = true
		return c
	}()
	mainOnly = func() dod.Config {
		c := dod.None()
		c.Merged = dod.Merged{Enabled: true, Branch: dod.BranchMain}
		return c
	}()
	deployOnly = func() dod.Config {
		c := dod.None()
		c.DeployedProduction = true
		return c
	}()
)

func commit(action commitmsg.Action, branch string, min int) Event {
	return Event{Kind: gitevent.KindCommit, Action: action, Branch: branch, At: at(min), New: true}
}

func pr(kind gitevent.Kind, target string, min int) Event {
	return Event{Kind: kind, Action: commitmsg.ActionLink, Branch: "fix/HD-1", Target: target, At: at(min), New: true}
}

func pipeline(branch string, min int) Event {
	return Event{Kind: gitevent.KindPipeline, Branch: branch, At: at(min), New: true}
}

func deploy(min int) Event {
	return Event{Kind: gitevent.KindDeployment, Branch: "main", At: at(min), New: true}
}

func seen(e Event) Event { e.New = false; return e }

func takes(e Event) Event { e.AuthorCanTake = true; return e }

func move(ev int, from, to ticket.Status, c ticket.Cause) Step {
	return Step{Kind: StepMove, Event: ev, From: from, To: to, Cause: c}
}

func tick(ev int, it dod.Item) Step { return Step{Kind: StepTick, Event: ev, Item: it} }

func TestDecide(t *testing.T) {
	const (
		nw  = ticket.StatusNew
		asg = ticket.StatusAssigned
		inp = ticket.StatusInProgress
		woc = ticket.StatusWaitingOnClient
		uat = ticket.StatusInUAT
		res = ticket.StatusResolved
		cls = ticket.StatusClosed
	)
	link, fix, reopen := commitmsg.ActionLink, commitmsg.ActionFix, commitmsg.ActionReopen
	tests := []struct {
		name    string
		in      Input
		steps   []Step
		results []Result
	}{
		{
			name:    "refs starts the work",
			in:      Input{Status: asg, HasAssignee: true, DoD: eOffice, Events: []Event{commit(link, "fix/HD-1", 0)}},
			steps:   []Step{move(0, asg, inp, ticket.CauseCodeActivity)},
			results: []Result{{Event: 0, Outcome: OutcomeLinked, From: asg, To: inp}},
		},
		{
			name:    "the author takes a new ticket",
			in:      Input{Status: nw, DoD: eOffice, Events: []Event{takes(commit(link, "fix/HD-1", 0))}},
			steps:   []Step{{Kind: StepAssign, Event: 0}, move(0, asg, inp, ticket.CauseCodeActivity)},
			results: []Result{{Event: 0, Outcome: OutcomeLinked, From: nw, To: inp}},
		},
		{
			name:    "an unknown author leaves it New",
			in:      Input{Status: nw, DoD: eOffice, Events: []Event{commit(link, "fix/HD-1", 0)}},
			results: []Result{{Event: 0, Outcome: OutcomeLinked}},
		},
		{
			name:    "fix ticks Fix committed",
			in:      Input{Status: inp, HasAssignee: true, DoD: eOffice, Events: []Event{commit(fix, "fix/HD-1", 0)}},
			steps:   []Step{tick(0, dod.ItemFixCommitted)},
			results: []Result{{Event: 0, Outcome: OutcomeFixCommitted, Items: []dod.Item{dod.ItemFixCommitted}}},
		},
		{
			name:    "pull request opened is in review",
			in:      Input{Status: asg, HasAssignee: true, DoD: eOffice, Events: []Event{pr(gitevent.KindPROpened, "main", 0)}},
			steps:   []Step{move(0, asg, inp, ticket.CauseCodeActivity)},
			results: []Result{{Event: 0, Outcome: OutcomeInReview, From: asg, To: inp}},
		},
		{
			name:    "pull request closed only links",
			in:      Input{Status: asg, HasAssignee: true, DoD: eOffice, Events: []Event{pr(gitevent.KindPRClosed, "main", 0)}},
			results: []Result{{Event: 0, Outcome: OutcomeLinked}},
		},
		{
			name:    "merged into main: the client tests",
			in:      Input{Status: inp, HasAssignee: true, DoD: eOffice, Events: []Event{pr(gitevent.KindPRMerged, "main", 0)}},
			steps:   []Step{tick(0, dod.ItemMerged), move(0, inp, uat, ticket.CauseCodeDone)},
			results: []Result{{Event: 0, Outcome: OutcomeMerged, Items: []dod.Item{dod.ItemMerged}, From: inp, To: uat}},
		},
		{
			name:    "merged into another branch only links",
			in:      Input{Status: inp, HasAssignee: true, DoD: eOffice, Events: []Event{pr(gitevent.KindPRMerged, "develop", 0)}},
			results: []Result{{Event: 0, Outcome: OutcomeLinked}},
		},
		{
			name: "a commit pushed straight to main counts as merged",
			in:   Input{Status: asg, HasAssignee: true, DoD: mainOnly, Events: []Event{commit(fix, "main", 0)}},
			steps: []Step{
				move(0, asg, inp, ticket.CauseCodeActivity), tick(0, dod.ItemFixCommitted), tick(0, dod.ItemMerged),
				move(0, inp, res, ticket.CauseCodeDone),
			},
			results: []Result{{Event: 0, Outcome: OutcomeMerged, Items: []dod.Item{dod.ItemFixCommitted, dod.ItemMerged}, From: asg, To: res}},
		},
		{
			name:    "a commit seen before reaching main still counts",
			in:      Input{Status: inp, HasAssignee: true, DoD: eOffice, Events: []Event{seen(commit(link, "main", 0))}},
			steps:   []Step{tick(0, dod.ItemMerged), move(0, inp, uat, ticket.CauseCodeDone)},
			results: []Result{{Event: 0, Outcome: OutcomeMerged, Items: []dod.Item{dod.ItemMerged}, From: inp, To: uat}},
		},
		{
			name:    "merge not required: nothing ticked or moved",
			in:      Input{Status: inp, HasAssignee: true, DoD: dod.Default(), Events: []Event{pr(gitevent.KindPRMerged, "main", 0)}},
			results: []Result{{Event: 0, Outcome: OutcomeLinked}},
		},
		{
			name:    "merged into a release branch waits for CI",
			in:      Input{Status: inp, HasAssignee: true, DoD: sipd, Events: []Event{pr(gitevent.KindPRMerged, "release/2026.11", 0)}},
			steps:   []Step{tick(0, dod.ItemMerged)},
			results: []Result{{Event: 0, Outcome: OutcomeMerged, Items: []dod.Item{dod.ItemMerged}}},
		},
		{
			name: "CI after the merge resolves",
			in: Input{Status: inp, HasAssignee: true, DoD: sipd, Done: map[dod.Item]time.Time{dod.ItemMerged: at(0)},
				Events: []Event{pipeline("release/2026.11", 1)}},
			steps:   []Step{tick(0, dod.ItemCIPassed), move(0, inp, res, ticket.CauseCodeDone)},
			results: []Result{{Event: 0, Outcome: OutcomeCIPassed, Items: []dod.Item{dod.ItemCIPassed}, From: inp, To: res}},
		},
		{
			name: "CI stored before the merge was read still counts",
			in: Input{Status: inp, HasAssignee: true, DoD: sipd,
				Events: []Event{pr(gitevent.KindPRMerged, "release/2026.11", 0), seen(pipeline("release/2026.11", 1))}},
			steps: []Step{tick(0, dod.ItemMerged), tick(1, dod.ItemCIPassed), move(1, inp, res, ticket.CauseCodeDone)},
			results: []Result{
				{Event: 0, Outcome: OutcomeMerged, Items: []dod.Item{dod.ItemMerged}},
				{Event: 1, Outcome: OutcomeCIPassed, Items: []dod.Item{dod.ItemCIPassed}, From: inp, To: res},
			},
		},
		{
			name: "CI before the merge doesn't count",
			in: Input{Status: inp, HasAssignee: true, DoD: sipd, Done: map[dod.Item]time.Time{dod.ItemMerged: at(5)},
				Events: []Event{pipeline("release/2026.11", 1)}},
		},
		{
			name: "CI on another branch doesn't count",
			in: Input{Status: inp, HasAssignee: true, DoD: sipd, Done: map[dod.Item]time.Time{dod.ItemMerged: at(0)},
				Events: []Event{pipeline("fix/HD-1", 1)}},
		},
		{
			name:    "deployed after the merge resolves",
			in:      Input{Status: inp, HasAssignee: true, DoD: website, Done: map[dod.Item]time.Time{dod.ItemMerged: at(0)}, Events: []Event{deploy(10)}},
			steps:   []Step{tick(0, dod.ItemDeployed), move(0, inp, res, ticket.CauseCodeDone)},
			results: []Result{{Event: 0, Outcome: OutcomeDeployed, Items: []dod.Item{dod.ItemDeployed}, From: inp, To: res}},
		},
		{
			name: "deploy without the merge waits",
			in:   Input{Status: inp, HasAssignee: true, DoD: website, Events: []Event{deploy(10)}},
		},
		{
			name:    "deploy-only counts from the first link",
			in:      Input{Status: inp, HasAssignee: true, DoD: deployOnly, LinkedAt: at(0), Events: []Event{deploy(10)}},
			steps:   []Step{tick(0, dod.ItemDeployed), move(0, inp, res, ticket.CauseCodeDone)},
			results: []Result{{Event: 0, Outcome: OutcomeDeployed, Items: []dod.Item{dod.ItemDeployed}, From: inp, To: res}},
		},
		{
			name: "deploy-only without any code does nothing",
			in:   Input{Status: inp, HasAssignee: true, DoD: deployOnly, Events: []Event{deploy(10)}},
		},
		{
			name:    "waiting on the client moves on when merged",
			in:      Input{Status: woc, HasAssignee: true, DoD: eOffice, Events: []Event{seen(pr(gitevent.KindPRMerged, "main", 0))}},
			steps:   []Step{tick(0, dod.ItemMerged), move(0, woc, uat, ticket.CauseCodeDone)},
			results: []Result{{Event: 0, Outcome: OutcomeMerged, Items: []dod.Item{dod.ItemMerged}, From: woc, To: uat}},
		},
		{
			name:    "In UAT stays put",
			in:      Input{Status: uat, HasAssignee: true, DoD: website, Done: map[dod.Item]time.Time{dod.ItemMerged: at(0)}, Events: []Event{deploy(5)}},
			steps:   []Step{tick(0, dod.ItemDeployed)},
			results: []Result{{Event: 0, Outcome: OutcomeDeployed, Items: []dod.Item{dod.ItemDeployed}}},
		},
		{
			name: "reopen takes a resolved ticket back and forgets the merge",
			in: Input{Status: res, HasAssignee: true, DoD: eOffice,
				Done:   map[dod.Item]time.Time{dod.ItemMerged: at(0), dod.ItemUATApproved: at(1)},
				Events: []Event{commit(reopen, "main", 5)}},
			steps:   []Step{move(0, res, inp, ticket.CauseCodeReopened)},
			results: []Result{{Event: 0, Outcome: OutcomeReopened, From: res, To: inp}},
		},
		{
			name: "after a reopen the next merge moves it on again",
			in: Input{Status: res, HasAssignee: true, DoD: eOffice, Done: map[dod.Item]time.Time{dod.ItemMerged: at(0)},
				Events: []Event{commit(reopen, "fix/HD-1", 5), pr(gitevent.KindPRMerged, "main", 9)}},
			steps: []Step{
				move(0, res, inp, ticket.CauseCodeReopened), tick(1, dod.ItemMerged), move(1, inp, uat, ticket.CauseCodeDone),
			},
			results: []Result{
				{Event: 0, Outcome: OutcomeReopened, From: res, To: inp},
				{Event: 1, Outcome: OutcomeMerged, Items: []dod.Item{dod.ItemMerged}, From: inp, To: uat},
			},
		},
		{
			name:    "reopen without an assignee goes to New",
			in:      Input{Status: uat, DoD: dod.Default(), Events: []Event{commit(reopen, "fix/HD-1", 0)}},
			steps:   []Step{move(0, uat, nw, ticket.CauseCodeReopened)},
			results: []Result{{Event: 0, Outcome: OutcomeReopened, From: uat, To: nw}},
		},
		{
			name:    "reopen on a ticket in progress only links",
			in:      Input{Status: inp, HasAssignee: true, DoD: dod.Default(), Events: []Event{commit(reopen, "fix/HD-1", 0)}},
			results: []Result{{Event: 0, Outcome: OutcomeLinked}},
		},
		{
			name:    "closed tickets are ignored",
			in:      Input{Status: cls, HasAssignee: true, DoD: eOffice, Events: []Event{commit(reopen, "main", 0), deploy(1)}},
			results: []Result{{Event: 0, Outcome: OutcomeTicketClosed}},
		},
		{
			name: "seen again: nothing changes",
			in: Input{Status: asg, HasAssignee: true, DoD: eOffice, LinkedAt: at(0),
				Done:   map[dod.Item]time.Time{dod.ItemFixCommitted: at(0)},
				Events: []Event{seen(commit(fix, "fix/HD-1", 0)), seen(commit(reopen, "fix/HD-1", 1))}},
		},
		{
			name: "projects without code conditions are moved by hand",
			in:   Input{Status: inp, HasAssignee: true, DoD: dod.Default(), Events: []Event{seen(commit(link, "main", 0))}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Decide(tt.in)
			if !reflect.DeepEqual(got.Steps, tt.steps) {
				t.Errorf("steps = %+v\nwant    %+v", got.Steps, tt.steps)
			}
			if !reflect.DeepEqual(got.Results, tt.results) {
				t.Errorf("results = %+v\nwant      %+v", got.Results, tt.results)
			}
		})
	}
}

func TestDecideLeavesInputAlone(t *testing.T) {
	done := map[dod.Item]time.Time{}
	Decide(Input{Status: ticket.StatusInProgress, HasAssignee: true, DoD: eOffice, Done: done,
		Events: []Event{pr(gitevent.KindPRMerged, "main", 0)}})
	if len(done) != 0 {
		t.Fatalf("Decide changed the caller's items: %v", done)
	}
}
