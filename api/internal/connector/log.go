package connector

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/commitmsg"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/gitevent"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

// LogLimit is how many recent events the Integrations page lists.
const LogLimit = 20

// MaxTestMessageLen caps the tester's message, matching the API.
const MaxTestMessageLen = 5000

// TestRef is one ticket a tested message names.
type TestRef struct {
	Key          string
	TicketExists bool
}

// TestMessage reads a commit message the way the connector does ("Try a commit message").
func (s *Service) TestMessage(ctx context.Context, message string) ([]TestRef, error) {
	q := s.store.Q()
	prefixes, err := q.ListTicketPrefixes(ctx)
	if err != nil {
		return nil, err
	}
	keys := commitmsg.New(prefixes).Keys(message)
	found, err := q.TicketKeysExist(ctx, keys)
	if err != nil {
		return nil, err
	}
	out := make([]TestRef, len(keys))
	for i, k := range keys {
		out[i] = TestRef{Key: k, TicketExists: slices.Contains(found, k)}
	}
	return out, nil
}

// LogEntry is one key a commit named, for the recent-events log.
type LogEntry struct {
	ID          uuid.UUID
	At          time.Time
	Provider    gitevent.Provider
	Repository  string
	Ref         string // short sha
	Title       string
	Branch      string
	URL         string
	TicketKey   string
	TicketFound bool
	Outcome     tickets.CodeOutcome
}

// Log lists the latest commit links, newest first, ignored keys included.
func (s *Service) Log(ctx context.Context) ([]LogEntry, error) {
	rows, err := s.store.Q().ListCodeLog(ctx, LogLimit)
	if err != nil {
		return nil, err
	}
	out := make([]LogEntry, len(rows))
	for i, r := range rows {
		out[i] = LogEntry{
			ID: r.ID, At: r.At, Provider: gitevent.Provider(r.Provider), Repository: r.FullName,
			Ref: shortSHA(r.Sha), Title: r.Title, Branch: r.Branch, URL: r.Url,
			TicketKey: r.TicketKey, TicketFound: r.TicketFound, Outcome: tickets.CodeOutcome(r.Outcome),
		}
	}
	return out, nil
}
