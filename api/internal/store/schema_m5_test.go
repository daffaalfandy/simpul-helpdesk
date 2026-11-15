package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/notification"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
)

const (
	constraintNotificationChannel  = "notifications_has_channel"
	constraintNotificationSource   = "notifications_source_user_key"
	constraintNotificationSkipped  = "notifications_skipped_is_handled"
	constraintNotificationEvent    = "notifications_event_type_check"
	constraintPreferenceEvent      = "notification_preferences_event_type_check"
	constraintUserEmailFrequency   = "users_email_frequency_check"
	constraintWatcherPrimaryKey    = "ticket_watchers_pkey"
	sqlInsertNotificationForTicket = `
		INSERT INTO notifications (user_id, ticket_id, event_type, source_id, web, email, emailed_at, email_skipped)
		VALUES ($1, $2, $3, $4, $5, $6, CASE WHEN $7 THEN now() END, $7)`
)

func TestNotificationConstraints(t *testing.T) {
	ctx := context.Background()
	f := newTicketFixture(t)
	tk := f.mustInsert(t, ticket{})
	source := uuid.New()
	insert := func(event string, src uuid.UUID, web, email, skipped bool) error {
		_, err := f.pool.Exec(ctx, sqlInsertNotificationForTicket, f.agent, tk, event, src, web, email, skipped)
		return err
	}
	if err := insert(string(notification.EventComment), source, true, false, false); err != nil {
		t.Fatalf("valid notification: %v", err)
	}
	cases := []struct {
		name       string
		err        error
		state      string
		constraint string
	}{
		{"same change twice", insert(string(notification.EventComment), source, true, true, false), store.SQLStateUniqueViolation, constraintNotificationSource},
		{"no channel", insert(string(notification.EventComment), uuid.New(), false, false, false), store.SQLStateCheckViolation, constraintNotificationChannel},
		{"unknown event", insert("billing", uuid.New(), true, false, false), store.SQLStateCheckViolation, constraintNotificationEvent},
	}
	for _, c := range cases {
		if !store.IsViolation(c.err, c.state, c.constraint) {
			t.Errorf("%s: err = %v", c.name, c.err)
		}
	}
	// A skipped email is a handled email.
	_, err := f.pool.Exec(ctx, `UPDATE notifications SET email_skipped = true WHERE source_id = $1`, source)
	if !store.IsViolation(err, store.SQLStateCheckViolation, constraintNotificationSkipped) {
		t.Errorf("skipped without emailed_at: err = %v", err)
	}
}

// Every event the code knows is one the database accepts.
func TestNotificationEventsMatchSchema(t *testing.T) {
	ctx := context.Background()
	f := newTicketFixture(t)
	tk := f.mustInsert(t, ticket{})
	for _, e := range notification.Events {
		if _, err := f.pool.Exec(ctx, sqlInsertNotificationForTicket, f.agent, tk, string(e), uuid.New(), true, false, false); err != nil {
			t.Errorf("notification %s: %v", e, err)
		}
		if _, err := f.pool.Exec(ctx, `INSERT INTO notification_preferences (user_id, event_type, web, email) VALUES ($1, $2, true, false)`,
			f.agent, string(e)); err != nil {
			t.Errorf("preference %s: %v", e, err)
		}
	}
	_, err := f.pool.Exec(ctx, `INSERT INTO notification_preferences (user_id, event_type, web, email) VALUES ($1, 'billing', true, true)`, f.agent)
	if !store.IsViolation(err, store.SQLStateCheckViolation, constraintPreferenceEvent) {
		t.Errorf("unknown preference event: err = %v", err)
	}
	for _, freq := range notification.Frequencies {
		if _, err := f.pool.Exec(ctx, `UPDATE users SET email_frequency = $1 WHERE id = $2`, string(freq), f.agent); err != nil {
			t.Errorf("frequency %s: %v", freq, err)
		}
	}
	_, err = f.pool.Exec(ctx, `UPDATE users SET email_frequency = 'weekly' WHERE id = $1`, f.agent)
	if !store.IsViolation(err, store.SQLStateCheckViolation, constraintUserEmailFrequency) {
		t.Errorf("unknown frequency: err = %v", err)
	}
}

func TestWatchersAreUniquePerTicket(t *testing.T) {
	ctx := context.Background()
	f := newTicketFixture(t)
	tk := f.mustInsert(t, ticket{})
	add := func() error {
		_, err := f.pool.Exec(ctx, `INSERT INTO ticket_watchers (ticket_id, user_id, added_by) VALUES ($1, $2, $2)`, tk, f.requester)
		return err
	}
	if err := add(); err != nil {
		t.Fatal(err)
	}
	if err := add(); !store.IsViolation(err, store.SQLStateUniqueViolation, constraintWatcherPrimaryKey) {
		t.Errorf("second watch: err = %v", err)
	}
}
