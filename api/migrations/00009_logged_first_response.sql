-- +goose Up

-- An agent-logged ticket is answered when it is logged (owner, 15 Nov; the service does this
-- for new tickets since M5b). Apply it to older ones, with the audit entry the service writes,
-- so the SLA engine doesn't replay them as unanswered and flag them breached.
INSERT INTO audit_log (at, actor_kind, actor_user_id, actor_name, action, target_type, target_id, data)
SELECT t.created_at, 'user', t.created_by, u.name, 'ticket.first_responded', 'ticket', t.id,
       '{"cause": "logged_by_staff"}'::jsonb
FROM tickets t
JOIN users u ON u.id = t.created_by
WHERE t.channel <> 'portal' AND t.first_response_at IS NULL;

-- Correcting history isn't an edit, so updated_at stays as it was.
ALTER TABLE tickets DISABLE TRIGGER tickets_updated_at;
UPDATE tickets SET first_response_at = created_at
WHERE channel <> 'portal' AND first_response_at IS NULL;
ALTER TABLE tickets ENABLE TRIGGER tickets_updated_at;

-- Clocks already replayed under the old rule are replayed again (silently) by the next SLA scan.
DELETE FROM ticket_sla s
USING tickets t
WHERE s.ticket_id = t.id AND t.channel <> 'portal' AND s.first_response_met_at IS NULL;

-- +goose Down
-- Nothing to undo: the old rule would only make these tickets wrongly unanswered again.
SELECT 1;
