-- The agent board. Staff only, so only the agent scope applies (see tickets.sql).

-- name: ListBoardCards :many
-- A ticket with a status in @statuses lands in the column at the same index of @columns.
-- Done tickets show only once resolved_at >= @resolved_since (open ones have no resolved_at).
-- Each column keeps its first @column_limit cards, P1 first (priority codes sort as text),
-- then the longest in the column, and reports how many it holds in total.
-- With @sla_at_risk, only open tickets with an unmet clock the scan flagged at risk or breached.
WITH board AS (
  SELECT t.id, t.key, t.type, t.status, t.priority, t.subject, t.status_changed_at,
         t.client_id, c.name AS client_name, t.assignee_id, a.name AS assignee_name,
         (@columns::text[])[array_position(@statuses::text[], t.status)]::text AS column_id,
         s.ticket_id AS sla_ticket_id,
         s.first_response_budget_ms, s.first_response_calendar, s.first_response_elapsed_ms,
         s.first_response_running_since, s.first_response_due_at, s.first_response_at_risk_flagged_at,
         s.first_response_breached_at, s.first_response_met_at,
         s.resolution_budget_ms, s.resolution_calendar, s.resolution_elapsed_ms,
         s.resolution_running_since, s.resolution_due_at, s.resolution_at_risk_flagged_at,
         s.resolution_breached_at, s.resolution_met_at
  FROM tickets t
  JOIN clients c ON c.id = t.client_id
  LEFT JOIN users a ON a.id = t.assignee_id
  LEFT JOIN ticket_sla s ON s.ticket_id = t.id
  WHERE t.status = ANY(@statuses::text[])
    AND (NOT @sla_at_risk::boolean
         OR (t.status = ANY(@open_statuses::text[])
             AND ((s.first_response_met_at IS NULL
                   AND (s.first_response_at_risk_flagged_at IS NOT NULL OR s.first_response_breached_at IS NOT NULL))
               OR (s.resolution_met_at IS NULL
                   AND (s.resolution_at_risk_flagged_at IS NOT NULL OR s.resolution_breached_at IS NOT NULL)))))
    AND (sqlc.narg(scope_agent_id)::uuid IS NULL
         OR EXISTS (SELECT 1 FROM client_agents ca
                    WHERE ca.client_id = t.client_id AND ca.user_id = sqlc.narg(scope_agent_id)::uuid))
    AND (t.resolved_at IS NULL OR t.resolved_at >= @resolved_since::timestamptz)
    AND (sqlc.narg(assignee_id)::uuid IS NULL OR t.assignee_id = sqlc.narg(assignee_id)::uuid)
    AND (NOT @unassigned::boolean OR t.assignee_id IS NULL)
    AND (sqlc.narg(search_pattern)::text IS NULL
         OR t.key = sqlc.narg(search_key)::text
         OR t.subject ILIKE sqlc.narg(search_pattern)::text
         OR c.name ILIKE sqlc.narg(search_pattern)::text)
), ranked AS (
  SELECT board.*,
         row_number() OVER (PARTITION BY column_id ORDER BY priority, status_changed_at, id) AS position,
         count(*) OVER (PARTITION BY column_id) AS column_total
  FROM board
)
SELECT id, key, type, status, priority, subject, client_id, client_name, assignee_id, assignee_name,
       column_id, column_total::bigint AS column_total, sla_ticket_id,
       first_response_budget_ms, first_response_calendar, first_response_elapsed_ms,
       first_response_running_since, first_response_due_at, first_response_at_risk_flagged_at,
       first_response_breached_at, first_response_met_at,
       resolution_budget_ms, resolution_calendar, resolution_elapsed_ms,
       resolution_running_since, resolution_due_at, resolution_at_risk_flagged_at,
       resolution_breached_at, resolution_met_at
FROM ranked
WHERE position <= @column_limit::bigint
ORDER BY column_id, position;

-- name: CountBoardStats :one
-- In the viewer's scope: open tickets; tickets resolved or closed since @week_start; open
-- tickets with an unmet clock breached, or else at risk; the mean first-response time of
-- portal reports answered since @month_start (agent-logged ones are answered at 0 by rule);
-- tickets resolved since @month_start, and how many of those never breached.
SELECT count(*) FILTER (WHERE t.status = ANY(@open_statuses::text[]))::bigint AS open_tickets,
       count(*) FILTER (WHERE t.resolved_at >= @week_start::timestamptz)::bigint AS resolved_this_week,
       count(*) FILTER (WHERE t.status = ANY(@open_statuses::text[])
         AND ((s.first_response_met_at IS NULL AND s.first_response_breached_at IS NOT NULL)
           OR (s.resolution_met_at IS NULL AND s.resolution_breached_at IS NOT NULL)))::bigint AS sla_breached,
       count(*) FILTER (WHERE t.status = ANY(@open_statuses::text[])
         AND NOT ((s.first_response_met_at IS NULL AND s.first_response_breached_at IS NOT NULL)
               OR (s.resolution_met_at IS NULL AND s.resolution_breached_at IS NOT NULL))
         AND ((s.first_response_met_at IS NULL AND s.first_response_at_risk_flagged_at IS NOT NULL)
           OR (s.resolution_met_at IS NULL AND s.resolution_at_risk_flagged_at IS NOT NULL)))::bigint AS sla_at_risk,
       coalesce(sum(s.first_response_elapsed_ms) FILTER (WHERE s.first_response_met_at >= @month_start::timestamptz
         AND t.channel = @portal_channel::text), 0)::bigint AS first_response_ms_total,
       count(*) FILTER (WHERE s.first_response_met_at >= @month_start::timestamptz
         AND t.channel = @portal_channel::text)::bigint AS first_responses,
       count(*) FILTER (WHERE t.resolved_at >= @month_start::timestamptz)::bigint AS resolved_this_month,
       count(*) FILTER (WHERE t.resolved_at >= @month_start::timestamptz
         AND s.first_response_breached_at IS NULL AND s.resolution_breached_at IS NULL)::bigint AS resolved_within_sla
FROM tickets t
LEFT JOIN ticket_sla s ON s.ticket_id = t.id
-- Only rows a number can count: open tickets, and tickets resolved since the earlier start
-- (a first response this month belongs to one of those). Closed history stays unread.
WHERE (t.status = ANY(@open_statuses::text[]) OR t.resolved_at >= least(@week_start::timestamptz, @month_start::timestamptz))
  AND (sqlc.narg(scope_agent_id)::uuid IS NULL
       OR EXISTS (SELECT 1 FROM client_agents ca
                  WHERE ca.client_id = t.client_id AND ca.user_id = sqlc.narg(scope_agent_id)::uuid));

-- name: GetOldestSLAProblem :one
-- The open ticket in scope whose flagged, unmet clock fell due (or turns due) first: the
-- board banner names it.
SELECT t.key, c.name AS client_name
FROM tickets t
JOIN clients c ON c.id = t.client_id
JOIN ticket_sla s ON s.ticket_id = t.id
WHERE t.status = ANY(@open_statuses::text[])
  AND (sqlc.narg(scope_agent_id)::uuid IS NULL
       OR EXISTS (SELECT 1 FROM client_agents ca
                  WHERE ca.client_id = t.client_id AND ca.user_id = sqlc.narg(scope_agent_id)::uuid))
  AND ((s.first_response_met_at IS NULL
        AND (s.first_response_at_risk_flagged_at IS NOT NULL OR s.first_response_breached_at IS NOT NULL))
    OR (s.resolution_met_at IS NULL
        AND (s.resolution_at_risk_flagged_at IS NOT NULL OR s.resolution_breached_at IS NOT NULL)))
ORDER BY least(
  CASE WHEN s.first_response_met_at IS NULL THEN coalesce(s.first_response_breached_at, s.first_response_due_at) END,
  CASE WHEN s.resolution_met_at IS NULL THEN coalesce(s.resolution_breached_at, s.resolution_due_at) END) NULLS LAST,
  t.id
LIMIT 1;
