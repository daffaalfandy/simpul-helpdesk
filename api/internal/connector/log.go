package connector

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/codeflow"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/commitmsg"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/gitevent"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// LogLimit is how many recent events the Integrations page lists.
const LogLimit = 20

// MaxTestMessageLen caps the tester's message, matching the API.
const MaxTestMessageLen = 5000

// TestRef is one ticket a tested message names.
type TestRef struct {
	Key          string
	Action       commitmsg.Action
	TicketExists bool
}

// TestMessage reads a commit message the way the connector does ("Try a commit message").
func (s *Service) TestMessage(ctx context.Context, message string) ([]TestRef, error) {
	q := s.store.Q()
	prefixes, err := q.ListTicketPrefixes(ctx)
	if err != nil {
		return nil, err
	}
	refs := commitmsg.New(prefixes).Message(message)
	keys := make([]string, len(refs))
	for i, r := range refs {
		keys[i] = r.Key
	}
	found, err := q.TicketKeysExist(ctx, keys)
	if err != nil {
		return nil, err
	}
	out := make([]TestRef, len(refs))
	for i, r := range refs {
		out[i] = TestRef{Key: r.Key, Action: r.Action, TicketExists: slices.Contains(found, r.Key)}
	}
	return out, nil
}

// LogEntry is one thing a code event did, for the recent-events log.
type LogEntry struct {
	ID          uuid.UUID
	At          time.Time
	Provider    gitevent.Provider
	Repository  string
	Kind        gitevent.Kind
	Ref         string
	Title       string
	Branch      string
	Target      string
	URL         string
	TicketKey   string
	TicketFound bool
	Outcome     codeflow.Outcome
	Items       []dod.Item
	StatusFrom  *ticket.Status
	StatusTo    *ticket.Status
}

// Log lists what the latest code events did, newest first, ignored keys included.
func (s *Service) Log(ctx context.Context) ([]LogEntry, error) {
	rows, err := s.store.Q().ListCodeLog(ctx, LogLimit)
	if err != nil {
		return nil, err
	}
	out := make([]LogEntry, len(rows))
	for i, r := range rows {
		items := make([]dod.Item, len(r.Items))
		for j, it := range r.Items {
			items[j] = dod.Item(it)
		}
		kind := gitevent.Kind(r.Kind)
		out[i] = LogEntry{
			ID: r.ID, At: r.At, Provider: gitevent.Provider(r.Provider), Repository: r.FullName, Kind: kind,
			Ref: ref(kind, r.Sha, r.Number, r.Title), Title: r.Title, Branch: r.Branch, Target: r.Target, URL: r.Url,
			TicketKey: r.TicketKey, TicketFound: r.TicketFound, Outcome: codeflow.Outcome(r.Outcome), Items: items,
			StatusFrom: statusPtr(r.StatusFrom), StatusTo: statusPtr(r.StatusTo),
		}
	}
	return out, nil
}

func statusPtr(s *string) *ticket.Status {
	if s == nil {
		return nil
	}
	st := ticket.Status(*s)
	return &st
}
