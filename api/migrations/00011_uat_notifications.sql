-- +goose Up

-- UAT and Definition of Done notifications (M6): the client's test request, its result for the
-- team, and the supervisors' Definition of Done alerts.
ALTER TABLE notification_preferences DROP CONSTRAINT notification_preferences_event_type_check,
  ADD CONSTRAINT notification_preferences_event_type_check CHECK (event_type IN (
    'comment', 'internal_note', 'status', 'assignment', 'priority', 'sla', 'uat_result', 'dod',
    'reply', 'question', 'resolved', 'colleague', 'ticket_logged', 'uat_requested'));
ALTER TABLE notifications DROP CONSTRAINT notifications_event_type_check,
  ADD CONSTRAINT notifications_event_type_check CHECK (event_type IN (
    'comment', 'internal_note', 'status', 'assignment', 'priority', 'sla', 'uat_result', 'dod',
    'reply', 'question', 'resolved', 'colleague', 'ticket_logged', 'uat_requested'));

-- +goose Down
DELETE FROM notifications WHERE event_type IN ('uat_result', 'dod', 'uat_requested');
DELETE FROM notification_preferences WHERE event_type IN ('uat_result', 'dod', 'uat_requested');
ALTER TABLE notification_preferences DROP CONSTRAINT notification_preferences_event_type_check,
  ADD CONSTRAINT notification_preferences_event_type_check CHECK (event_type IN (
    'comment', 'internal_note', 'status', 'assignment', 'priority', 'sla',
    'reply', 'question', 'resolved', 'colleague', 'ticket_logged'));
ALTER TABLE notifications DROP CONSTRAINT notifications_event_type_check,
  ADD CONSTRAINT notifications_event_type_check CHECK (event_type IN (
    'comment', 'internal_note', 'status', 'assignment', 'priority', 'sla',
    'reply', 'question', 'resolved', 'colleague', 'ticket_logged'));
