package connector

import (
	"context"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/codeflow"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/gitevent"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

// shortSHALen is how a commit is named in the activity log, as on GitHub and GitLab.
const shortSHALen = 7

// maxStoredRuns caps the earlier CI runs and deployments read for one ticket.
const maxStoredRuns = 100

// apply hands the delivery's events to the tickets they reach, one transaction per ticket:
//   - a commit or pull/merge request reaches the tickets it names, when their project reads
//     code from this repository (other keys are logged and ignored);
//   - a CI run or deployment reaches the open tickets of linked projects that need one and that
//     code already reached;
//   - a ticket named now also gets the CI runs and deployments stored since, so one that arrived
//     before its merge still counts.
func (s *Service) apply(ctx context.Context, provider gitevent.Provider, repoID uuid.UUID, events []storedEvent) error {
	q := s.store.Q()
	var keys []string
	for _, e := range events {
		for _, r := range e.Refs {
			keys = append(keys, r.Key)
		}
	}
	found, err := q.ListTicketsByKeys(ctx, keys)
	if err != nil {
		return err
	}
	byKey := map[string]db.ListTicketsByKeysRow{}
	for _, t := range found {
		byKey[t.Key] = t
	}
	perTicket := map[uuid.UUID][]tickets.CodeEvent{}
	var order []uuid.UUID
	add := func(id uuid.UUID, ce tickets.CodeEvent) {
		if _, ok := perTicket[id]; !ok {
			order = append(order, id)
		}
		perTicket[id] = append(perTicket[id], ce)
	}
	var runs []tickets.CodeEvent
	for _, e := range events {
		ce := codeEvent(provider, e)
		if e.Event.Kind == gitevent.KindPipeline || e.Event.Kind == gitevent.KindDeployment {
			runs = append(runs, ce)
			continue
		}
		for _, r := range e.Refs {
			t, ok := byKey[r.Key]
			switch {
			case !ok:
				err = ignore(ctx, q, e.ID, nil, r.Key, codeflow.OutcomeUnknownTicket)
			case t.RepositoryID == nil || *t.RepositoryID != repoID:
				err = ignore(ctx, q, e.ID, &t.ID, r.Key, codeflow.OutcomeOtherRepository)
			default:
				named := ce
				named.Action = r.Action
				add(t.ID, named)
			}
			if err != nil {
				return err
			}
		}
	}
	if err := s.addRuns(ctx, q, repoID, runs, add); err != nil {
		return err
	}
	for _, id := range order {
		evs, err := s.withStoredRuns(ctx, q, provider, repoID, perTicket[id])
		if err != nil {
			return err
		}
		if err := s.tickets.ApplyCode(ctx, id, repoID, evs); err != nil {
			return err
		}
	}
	return nil
}

// addRuns gives the delivery's CI runs and deployments to the tickets waiting for one.
func (s *Service) addRuns(ctx context.Context, q *db.Queries, repoID uuid.UUID, runs []tickets.CodeEvent, add func(uuid.UUID, tickets.CodeEvent)) error {
	if len(runs) == 0 {
		return nil
	}
	statuses := make([]string, len(ticket.OpenStatuses))
	for i, st := range ticket.OpenStatuses {
		statuses[i] = string(st)
	}
	cands, err := q.ListRunCandidates(ctx, db.ListRunCandidatesParams{RepositoryID: &repoID, Statuses: statuses})
	if err != nil {
		return err
	}
	for _, c := range cands {
		eff, _, err := dod.ParseEffective(c.DodConfig, c.DodOverride)
		if err != nil {
			return err
		}
		for _, r := range runs {
			if (r.Kind == gitevent.KindPipeline && eff.CIPassed) || (r.Kind == gitevent.KindDeployment && eff.DeployedProduction) {
				add(c.ID, r)
			}
		}
	}
	return nil
}

// withStoredRuns adds the CI runs and deployments stored from the ticket's earliest event on.
func (s *Service) withStoredRuns(ctx context.Context, q *db.Queries, provider gitevent.Provider, repoID uuid.UUID, evs []tickets.CodeEvent) ([]tickets.CodeEvent, error) {
	var since time.Time
	for _, e := range evs {
		if e.Action != "" && (since.IsZero() || e.At.Before(since)) {
			since = e.At
		}
	}
	if since.IsZero() {
		return evs, nil // reached by a run only
	}
	stored, err := q.ListRunsSince(ctx, db.ListRunsSinceParams{RepositoryID: repoID, Since: since, RowLimit: maxStoredRuns})
	if err != nil {
		return nil, err
	}
	for _, r := range stored {
		if slices.ContainsFunc(evs, func(e tickets.CodeEvent) bool { return e.ID == r.ID }) {
			continue
		}
		evs = append(evs, tickets.CodeEvent{
			ID: r.ID, Provider: provider, Kind: gitevent.Kind(r.Kind), Ref: ref(gitevent.Kind(r.Kind), r.Sha, r.Number, r.Title),
			Branch: r.Branch, Target: r.Target, At: r.OccurredAt, AuthorID: r.AuthorUserID,
			AuthorName: authorName(r.AuthorLogin, r.AuthorName, r.AuthorUserName),
		})
	}
	return evs, nil
}

func codeEvent(provider gitevent.Provider, e storedEvent) tickets.CodeEvent {
	var number *int64
	if e.Event.Number != 0 {
		number = &e.Event.Number
	}
	return tickets.CodeEvent{
		ID: e.ID, Provider: provider, Kind: e.Event.Kind, Ref: ref(e.Event.Kind, e.Event.SHA, number, e.Event.Title),
		Branch: e.Event.Branch, Target: e.Event.Target, At: e.Event.OccurredAt, AuthorID: e.AuthorID, AuthorName: e.AuthorName,
	}
}

// ref is how the activity log names an event: a short commit sha, a request or run number, or
// the environment a deployment went to.
func ref(kind gitevent.Kind, sha string, number *int64, title string) string {
	switch {
	case kind == gitevent.KindCommit && len(sha) > shortSHALen:
		return sha[:shortSHALen]
	case kind == gitevent.KindCommit:
		return sha
	case kind == gitevent.KindDeployment:
		return title
	case number != nil:
		return strconv.FormatInt(*number, 10)
	}
	return title
}

// ignore logs a key the event named without reaching its ticket.
func ignore(ctx context.Context, q *db.Queries, eventID uuid.UUID, ticketID *uuid.UUID, key string, o codeflow.Outcome) error {
	return q.InsertCodeLink(ctx, db.InsertCodeLinkParams{
		CodeEventID: eventID, TicketID: ticketID, TicketKey: key, Outcome: string(o), Items: []string{},
	})
}
