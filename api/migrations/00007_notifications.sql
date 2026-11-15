-- +goose Up

-- How often a user's notification emails go out. Urgent items skip the summaries.
ALTER TABLE users ADD COLUMN email_frequency text NOT NULL DEFAULT 'instant'
  CONSTRAINT users_email_frequency_check CHECK (email_frequency IN ('instant', 'hourly', 'daily'));

-- "People notified" on the portal: the requester's colleagues who follow a ticket.
CREATE TABLE ticket_watchers (
  ticket_id  uuid NOT NULL CONSTRAINT ticket_watchers_ticket_id_fkey REFERENCES tickets (id) ON DELETE CASCADE,
  user_id    uuid NOT NULL CONSTRAINT ticket_watchers_user_id_fkey REFERENCES users (id) ON DELETE CASCADE,
  added_by   uuid NOT NULL CONSTRAINT ticket_watchers_added_by_fkey REFERENCES users (id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (ticket_id, user_id)
);
CREATE INDEX ticket_watchers_user_id_idx ON ticket_watchers (user_id);

-- A user's explicit web/email choice for one event. Defaults and always-on events live in code.
CREATE TABLE notification_preferences (
  user_id    uuid NOT NULL CONSTRAINT notification_preferences_user_id_fkey REFERENCES users (id) ON DELETE CASCADE,
  event_type text NOT NULL CONSTRAINT notification_preferences_event_type_check CHECK (event_type IN (
    'comment', 'internal_note', 'status', 'assignment', 'priority', 'sla',
    'reply', 'question', 'resolved', 'colleague', 'ticket_logged')),
  web        boolean NOT NULL,
  email      boolean NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, event_type)
);

-- One user's notice about one ticket change. source_id is the change (the first audit entry of
-- its transaction), so a retried fan-out never notifies twice. Bodies and subjects are not
-- copied: the inbox and emails read them from the ticket, so erasing a message erases its quote.
CREATE TABLE notifications (
  id               uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id          uuid NOT NULL CONSTRAINT notifications_user_id_fkey REFERENCES users (id) ON DELETE CASCADE,
  ticket_id        uuid NOT NULL CONSTRAINT notifications_ticket_id_fkey REFERENCES tickets (id) ON DELETE CASCADE,
  event_type       text NOT NULL CONSTRAINT notifications_event_type_check CHECK (event_type IN (
    'comment', 'internal_note', 'status', 'assignment', 'priority', 'sla',
    'reply', 'question', 'resolved', 'colleague', 'ticket_logged')),
  actor_user_id    uuid CONSTRAINT notifications_actor_user_id_fkey REFERENCES users (id) ON DELETE SET NULL,
  source_id        uuid NOT NULL,
  message_id       uuid CONSTRAINT notifications_message_id_fkey REFERENCES ticket_messages (id) ON DELETE SET NULL,
  data             jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(data) = 'object'),
  web              boolean NOT NULL,
  email            boolean NOT NULL,
  urgent           boolean NOT NULL DEFAULT false,
  created_at       timestamptz NOT NULL DEFAULT now(),
  read_at          timestamptz,
  -- Email delivery: a send job claims rows, sends, then marks them. A stale claim is retried.
  email_claimed_at timestamptz,
  emailed_at       timestamptz,
  email_skipped    boolean NOT NULL DEFAULT false,
  CONSTRAINT notifications_source_user_key UNIQUE (source_id, user_id),
  CONSTRAINT notifications_has_channel CHECK (web OR email),
  CONSTRAINT notifications_skipped_is_handled CHECK (NOT email_skipped OR emailed_at IS NOT NULL)
);
CREATE INDEX notifications_inbox_idx ON notifications (user_id, created_at DESC, id DESC) WHERE web;
CREATE INDEX notifications_unread_idx ON notifications (user_id) WHERE web AND read_at IS NULL;
CREATE INDEX notifications_email_pending_idx ON notifications (user_id, ticket_id, created_at)
  WHERE email AND emailed_at IS NULL;
CREATE INDEX notifications_created_at_idx ON notifications (created_at);
CREATE INDEX notifications_ticket_id_idx ON notifications (ticket_id);

-- +goose Down
DROP TABLE notifications;
DROP TABLE notification_preferences;
DROP TABLE ticket_watchers;
ALTER TABLE users DROP COLUMN email_frequency;
