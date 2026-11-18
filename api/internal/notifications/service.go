package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/notification"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/richtext"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

// Inbox paging.
const (
	DefaultPageSize = 30
	MaxPageSize     = 100
)

// Audit entries for settings changes.
const (
	ActionSettingsUpdated = "notification_settings.updated"

	dataEvent          = "event"
	dataWeb            = "web"
	dataEmail          = "email"
	dataEmailFrequency = "email_frequency"
)

// quoteEllipsis ends a quote cut at notification.QuoteMaxRunes.
const quoteEllipsis = "…"

var publicVisibility = string(ticket.VisibilityPublic)

// Item is one inbox entry.
type Item struct {
	ID        uuid.UUID
	Event     notification.Event
	Data      notification.Data
	TicketKey string
	Subject   string
	ActorName *string
	Quote     *string
	Channels  notification.Pref
	CreatedAt time.Time
	ReadAt    *time.Time
}

// Page is one page of the inbox.
type Page struct {
	Items       []Item
	UnreadCount int
	NextBefore  *uuid.UUID
}

// Settings are a user's notification switches and email timing.
type Settings struct {
	Audience  notification.Audience
	Frequency notification.Frequency
	Events    []notification.Setting
}

type Service struct {
	store *store.Store
}

func NewService(st *store.Store) *Service { return &Service{store: st} }

// List returns a page of p's inbox, about tickets p can still see.
func (s *Service) List(ctx context.Context, p *auth.Principal, f notification.Filter, before *uuid.UUID, limit int) (Page, error) {
	scope, err := tickets.ScopeFor(p)
	if err != nil {
		return Page{}, err
	}
	limit = pageSize(limit)
	events := []string{}
	for _, e := range notification.FilterEvents(f) {
		events = append(events, string(e))
	}
	q := s.store.Q()
	rows, err := q.ListNotifications(ctx, db.ListNotificationsParams{
		QuoteRunes: notification.QuoteSourceRunes, UserID: p.UserID,
		OrganizationID: scope.OrganizationID, ScopeClientID: scope.ClientID, ScopeAgentID: scope.AgentID, Events: events,
		UnreadOnly: f == notification.FilterUnread, Before: before, RowLimit: int32(limit + 1),
	})
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: []Item{}}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[limit-1].ID
		page.NextBefore = &last
	}
	for _, r := range rows {
		it := Item{
			ID: r.ID, Event: notification.Event(r.EventType), TicketKey: r.TicketKey, Subject: r.TicketSubject,
			ActorName: r.ActorName, Channels: notification.Pref{Web: r.Web, Email: r.Email},
			CreatedAt: r.CreatedAt, ReadAt: r.ReadAt,
		}
		_ = json.Unmarshal(r.Data, &it.Data)
		// Client staff only ever see public messages quoted.
		if r.QuoteVisibility != nil && (scope.Staff || *r.QuoteVisibility == publicVisibility) {
			if quote := cutQuote(richtext.Line(r.Quote)); quote != "" {
				it.Quote = &quote
			}
		}
		page.Items = append(page.Items, it)
	}
	page.UnreadCount, err = s.unread(ctx, q, p.UserID, scope)
	return page, err
}

// UnreadCount is the bell's number.
func (s *Service) UnreadCount(ctx context.Context, p *auth.Principal) (int, error) {
	scope, err := tickets.ScopeFor(p)
	if err != nil {
		return 0, err
	}
	return s.unread(ctx, s.store.Q(), p.UserID, scope)
}

func (s *Service) unread(ctx context.Context, q *db.Queries, user uuid.UUID, scope tickets.Scope) (int, error) {
	n, err := q.CountUnreadNotifications(ctx, db.CountUnreadNotificationsParams{
		UserID: user, OrganizationID: scope.OrganizationID, ScopeClientID: scope.ClientID, ScopeAgentID: scope.AgentID,
	})
	return int(n), err
}

// SetRead marks one of p's inbox items read or unread. Anyone else's gives 404.
func (s *Service) SetRead(ctx context.Context, p *auth.Principal, id uuid.UUID, read bool) error {
	scope, err := tickets.ScopeFor(p)
	if err != nil {
		return err
	}
	n, err := s.store.Q().SetNotificationRead(ctx, db.SetNotificationReadParams{
		Read: read, ID: id, UserID: p.UserID, OrganizationID: scope.OrganizationID, ScopeClientID: scope.ClientID, ScopeAgentID: scope.AgentID,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return apperr.NotFound()
	}
	return nil
}

// ReadAll marks every item of p's inbox read.
func (s *Service) ReadAll(ctx context.Context, p *auth.Principal) error {
	scope, err := tickets.ScopeFor(p)
	if err != nil {
		return err
	}
	return s.store.Q().MarkAllNotificationsRead(ctx, db.MarkAllNotificationsReadParams{
		UserID: p.UserID, OrganizationID: scope.OrganizationID, ScopeClientID: scope.ClientID, ScopeAgentID: scope.AgentID,
	})
}

// Settings returns p's switches, in the order their audience lists them.
func (s *Service) Settings(ctx context.Context, p *auth.Principal) (Settings, error) {
	if p == nil {
		return Settings{}, apperr.Unauthenticated()
	}
	return s.settings(ctx, s.store.Q(), p)
}

func (s *Service) settings(ctx context.Context, q *db.Queries, p *auth.Principal) (Settings, error) {
	freq, err := q.GetUserEmailFrequency(ctx, p.UserID)
	if err != nil {
		return Settings{}, err
	}
	rows, err := q.ListNotificationPreferences(ctx, []uuid.UUID{p.UserID})
	if err != nil {
		return Settings{}, err
	}
	overrides := map[notification.Event]notification.Pref{}
	for _, r := range rows {
		overrides[notification.Event(r.EventType)] = notification.Pref{Web: r.Web, Email: r.Email}
	}
	aud := audienceOf(p)
	return Settings{
		Audience: aud, Frequency: notification.Frequency(freq), Events: notification.Settings(aud, supervises(p), overrides),
	}, nil
}

// SetEventPreference switches one event's channels. Events outside p's settings and switching
// off an always-on event are refused, here and not only in the UI.
func (s *Service) SetEventPreference(ctx context.Context, p *auth.Principal, e notification.Event, pref notification.Pref) (Settings, error) {
	if p == nil {
		return Settings{}, apperr.Unauthenticated()
	}
	known, allowed := notification.Switchable(audienceOf(p), supervises(p), e, pref)
	switch {
	case !known:
		return Settings{}, apperr.Invalid(apperr.CodeNotificationEventUnknown, "This notification isn't in your settings", "")
	case !allowed:
		return Settings{}, apperr.Invalid(apperr.CodeNotificationLocked, "This notification is always on", "")
	}
	var out Settings
	err := s.store.Tx(ctx, func(tx store.Tx) error {
		if err := tx.Q.UpsertNotificationPreference(ctx, db.UpsertNotificationPreferenceParams{
			UserID: p.UserID, EventType: string(e), Web: pref.Web, Email: pref.Email,
		}); err != nil {
			return err
		}
		if err := audit.Write(ctx, tx.Q, p.AuditActor().Entry(ActionSettingsUpdated, auth.TargetUser, &p.UserID, map[string]any{
			dataEvent: e, dataWeb: pref.Web, dataEmail: pref.Email,
		})); err != nil {
			return err
		}
		var err error
		out, err = s.settings(ctx, tx.Q, p)
		return err
	})
	return out, err
}

// SetEmailFrequency chooses when p's notification emails go out.
func (s *Service) SetEmailFrequency(ctx context.Context, p *auth.Principal, f notification.Frequency) (Settings, error) {
	switch {
	case p == nil:
		return Settings{}, apperr.Unauthenticated()
	case !f.Valid():
		return Settings{}, apperr.Invalid(apperr.CodeInvalidEmailFrequency, "Choose when emails go out", "")
	}
	var out Settings
	err := s.store.Tx(ctx, func(tx store.Tx) error {
		old, err := tx.Q.GetUserEmailFrequency(ctx, p.UserID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.Unauthenticated()
		}
		if err != nil {
			return err
		}
		if err := tx.Q.SetUserEmailFrequency(ctx, db.SetUserEmailFrequencyParams{EmailFrequency: string(f), ID: p.UserID}); err != nil {
			return err
		}
		changes := audit.Changes{}
		audit.Track(changes, dataEmailFrequency, notification.Frequency(old), f)
		if len(changes) > 0 {
			if err := audit.Write(ctx, tx.Q, p.AuditActor().Entry(ActionSettingsUpdated, auth.TargetUser, &p.UserID, changes)); err != nil {
				return err
			}
		}
		out, err = s.settings(ctx, tx.Q, p)
		return err
	})
	return out, err
}

func audienceOf(p *auth.Principal) notification.Audience {
	return notification.AudienceFor(!auth.Can(p.Role, auth.CapPortal))
}

// supervises reports whether p is a supervisor of the organization they work in: only they hear
// about tickets nobody is assigned to. Administrators have no membership, so they don't.
func supervises(p *auth.Principal) bool { return p.Role == auth.RoleSupervisor }

// cutQuote trims a plain-text quote to notification.QuoteMaxRunes, marking the cut.
func cutQuote(s string) string {
	r := []rune(s)
	if len(r) <= notification.QuoteMaxRunes {
		return s
	}
	return string(r[:notification.QuoteMaxRunes]) + quoteEllipsis
}

func pageSize(n int) int {
	switch {
	case n <= 0:
		return DefaultPageSize
	case n > MaxPageSize:
		return MaxPageSize
	}
	return n
}
