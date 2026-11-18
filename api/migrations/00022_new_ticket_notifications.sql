-- +goose Up

-- New ticket notifications (10b): a ticket nobody is assigned to tells the organization's
-- supervisors.
ALTER TABLE notification_preferences DROP CONSTRAINT notification_preferences_event_type_check,
  ADD CONSTRAINT notification_preferences_event_type_check CHECK (event_type IN (
    'comment', 'internal_note', 'status', 'assignment', 'priority', 'sla', 'uat_result', 'dod', 'code', 'new_ticket',
    'reply', 'question', 'resolved', 'colleague', 'ticket_logged', 'uat_requested'));
ALTER TABLE notifications DROP CONSTRAINT notifications_event_type_check,
  ADD CONSTRAINT notifications_event_type_check CHECK (event_type IN (
    'comment', 'internal_note', 'status', 'assignment', 'priority', 'sla', 'uat_result', 'dod', 'code', 'new_ticket',
    'reply', 'question', 'resolved', 'colleague', 'ticket_logged', 'uat_requested'));

-- +goose Down
DELETE FROM notifications WHERE event_type = 'new_ticket';
DELETE FROM notification_preferences WHERE event_type = 'new_ticket';
ALTER TABLE notification_preferences DROP CONSTRAINT notification_preferences_event_type_check,
  ADD CONSTRAINT notification_preferences_event_type_check CHECK (event_type IN (
    'comment', 'internal_note', 'status', 'assignment', 'priority', 'sla', 'uat_result', 'dod', 'code',
    'reply', 'question', 'resolved', 'colleague', 'ticket_logged', 'uat_requested'));
ALTER TABLE notifications DROP CONSTRAINT notifications_event_type_check,
  ADD CONSTRAINT notifications_event_type_check CHECK (event_type IN (
    'comment', 'internal_note', 'status', 'assignment', 'priority', 'sla', 'uat_result', 'dod', 'code',
    'reply', 'question', 'resolved', 'colleague', 'ticket_logged', 'uat_requested'));
