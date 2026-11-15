-- name: GetTicketForSLA :one
SELECT t.id, t.type, t.status, t.priority, t.sla_snapshot, t.first_response_at, t.resolved_at
FROM tickets t
WHERE t.id = @id;

-- name: ListSLAHistory :many
-- The audit entries that move a ticket's clocks, oldest first.
SELECT id, at, action, data
FROM audit_log
WHERE target_type = @target_type AND target_id = @target_id AND action = ANY(@actions::text[])
ORDER BY at, id;

-- name: GetTicketSLA :one
SELECT * FROM ticket_sla WHERE ticket_id = @ticket_id;

-- name: UpsertTicketSLA :exec
INSERT INTO ticket_sla (
  ticket_id, computed_at,
  first_response_budget_ms, first_response_calendar, first_response_elapsed_ms, first_response_running_since,
  first_response_due_at, first_response_at_risk_at, first_response_at_risk_flagged_at, first_response_breached_at,
  first_response_met_at,
  resolution_budget_ms, resolution_calendar, resolution_elapsed_ms, resolution_running_since,
  resolution_due_at, resolution_at_risk_at, resolution_at_risk_flagged_at, resolution_breached_at,
  resolution_met_at)
VALUES (
  @ticket_id, @computed_at,
  @first_response_budget_ms, @first_response_calendar, @first_response_elapsed_ms, sqlc.narg(first_response_running_since),
  sqlc.narg(first_response_due_at), sqlc.narg(first_response_at_risk_at), sqlc.narg(first_response_at_risk_flagged_at),
  sqlc.narg(first_response_breached_at), sqlc.narg(first_response_met_at),
  @resolution_budget_ms, @resolution_calendar, @resolution_elapsed_ms, sqlc.narg(resolution_running_since),
  sqlc.narg(resolution_due_at), sqlc.narg(resolution_at_risk_at), sqlc.narg(resolution_at_risk_flagged_at),
  sqlc.narg(resolution_breached_at), sqlc.narg(resolution_met_at))
ON CONFLICT (ticket_id) DO UPDATE SET
  computed_at = EXCLUDED.computed_at,
  first_response_budget_ms = EXCLUDED.first_response_budget_ms,
  first_response_calendar = EXCLUDED.first_response_calendar,
  first_response_elapsed_ms = EXCLUDED.first_response_elapsed_ms,
  first_response_running_since = EXCLUDED.first_response_running_since,
  first_response_due_at = EXCLUDED.first_response_due_at,
  first_response_at_risk_at = EXCLUDED.first_response_at_risk_at,
  first_response_at_risk_flagged_at = EXCLUDED.first_response_at_risk_flagged_at,
  first_response_breached_at = EXCLUDED.first_response_breached_at,
  first_response_met_at = EXCLUDED.first_response_met_at,
  resolution_budget_ms = EXCLUDED.resolution_budget_ms,
  resolution_calendar = EXCLUDED.resolution_calendar,
  resolution_elapsed_ms = EXCLUDED.resolution_elapsed_ms,
  resolution_running_since = EXCLUDED.resolution_running_since,
  resolution_due_at = EXCLUDED.resolution_due_at,
  resolution_at_risk_at = EXCLUDED.resolution_at_risk_at,
  resolution_at_risk_flagged_at = EXCLUDED.resolution_at_risk_flagged_at,
  resolution_breached_at = EXCLUDED.resolution_breached_at,
  resolution_met_at = EXCLUDED.resolution_met_at;

-- name: ListOpenTicketIDsAfter :many
-- Open tickets in id order, a batch at a time (recompute).
SELECT id FROM tickets
WHERE status = ANY(@open_statuses::text[]) AND (sqlc.narg(after)::uuid IS NULL OR id > sqlc.narg(after)::uuid)
ORDER BY id
LIMIT @row_limit;

-- name: LockTicketForSLA :one
SELECT id FROM tickets WHERE id = @id FOR UPDATE;

-- name: ListUnreplayedTicketIDs :many
-- Tickets the engine hasn't run for yet (seed and tickets from before it): the scan backfills them.
SELECT t.id FROM tickets t
WHERE NOT EXISTS (SELECT 1 FROM ticket_sla s WHERE s.ticket_id = t.id)
ORDER BY t.id
LIMIT @row_limit;

-- name: ListSLABreaches :many
-- Running clocks past their due time that aren't flagged yet (breach is strictly after due).
SELECT ticket_id, @first_response_clock::text AS clock FROM ticket_sla
WHERE first_response_breached_at IS NULL AND first_response_due_at < @now::timestamptz
UNION ALL
SELECT ticket_id, @resolution_clock::text AS clock FROM ticket_sla
WHERE resolution_breached_at IS NULL AND resolution_due_at < @now::timestamptz
LIMIT @row_limit;

-- name: ListSLAAtRisk :many
-- Running clocks past their at-risk share that aren't flagged or breached yet.
SELECT ticket_id, @first_response_clock::text AS clock FROM ticket_sla
WHERE first_response_at_risk_flagged_at IS NULL AND first_response_breached_at IS NULL
  AND first_response_at_risk_at <= @now::timestamptz
UNION ALL
SELECT ticket_id, @resolution_clock::text AS clock FROM ticket_sla
WHERE resolution_at_risk_flagged_at IS NULL AND resolution_breached_at IS NULL
  AND resolution_at_risk_at <= @now::timestamptz
LIMIT @row_limit;

-- name: GetTicketForAlert :one
SELECT t.id, t.status, t.priority, t.sla_snapshot, c.pic_email
FROM tickets t
JOIN clients c ON c.id = t.client_id
WHERE t.id = @id;
