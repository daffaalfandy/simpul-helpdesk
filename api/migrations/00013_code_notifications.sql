-- +goose Up

-- Code activity notifications (M7): commits, pull/merge requests, CI runs and deployments that
-- reach a ticket, for its assignee.
ALTER TABLE notification_preferences DROP CONSTRAINT notification_preferences_event_type_check,
  ADD CONSTRAINT notification_preferences_event_type_check CHECK (event_type IN (
    'comment', 'internal_note', 'status', 'assignment', 'priority', 'sla', 'uat_result', 'dod', 'code',
    'reply', 'question', 'resolved', 'colleague', 'ticket_logged', 'uat_requested'));
ALTER TABLE notifications DROP CONSTRAINT notifications_event_type_check,
  ADD CONSTRAINT notifications_event_type_check CHECK (event_type IN (
    'comment', 'internal_note', 'status', 'assignment', 'priority', 'sla', 'uat_result', 'dod', 'code',
    'reply', 'question', 'resolved', 'colleague', 'ticket_logged', 'uat_requested'));

-- +goose Down
DELETE FROM notifications WHERE event_type = 'code';
DELETE FROM notification_preferences WHERE event_type = 'code';
ALTER TABLE notification_preferences DROP CONSTRAINT notification_preferences_event_type_check,
  ADD CONSTRAINT notification_preferences_event_type_check CHECK (event_type IN (
    'comment', 'internal_note', 'status', 'assignment', 'priority', 'sla', 'uat_result', 'dod',
    'reply', 'question', 'resolved', 'colleague', 'ticket_logged', 'uat_requested'));
ALTER TABLE notifications DROP CONSTRAINT notifications_event_type_check,
  ADD CONSTRAINT notifications_event_type_check CHECK (event_type IN (
    'comment', 'internal_note', 'status', 'assignment', 'priority', 'sla', 'uat_result', 'dod',
    'reply', 'question', 'resolved', 'colleague', 'ticket_logged', 'uat_requested'));
