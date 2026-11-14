-- The agent board. Staff only, so only the agent scope applies (see tickets.sql).

-- name: ListBoardCards :many
-- A ticket with a status in @statuses lands in the column at the same index of @columns.
-- Done tickets show only once resolved_at >= @resolved_since (open ones have no resolved_at).
-- Each column keeps its first @column_limit cards, P1 first (priority codes sort as text),
-- then the longest in the column, and reports how many it holds in total.
WITH board AS (
  SELECT t.id, t.key, t.type, t.status, t.priority, t.subject, t.status_changed_at,
         t.client_id, c.name AS client_name, t.assignee_id, a.name AS assignee_name,
         (@columns::text[])[array_position(@statuses::text[], t.status)]::text AS column_id
  FROM tickets t
  JOIN clients c ON c.id = t.client_id
  LEFT JOIN users a ON a.id = t.assignee_id
  WHERE t.status = ANY(@statuses::text[])
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
       column_id, column_total::bigint AS column_total
FROM ranked
WHERE position <= @column_limit::bigint
ORDER BY column_id, position;

-- name: CountBoardStats :one
-- Open tickets, and tickets resolved or closed since @week_start, in the viewer's scope.
SELECT count(*) FILTER (WHERE t.status = ANY(@open_statuses::text[]))::bigint AS open_tickets,
       count(*) FILTER (WHERE t.resolved_at >= @week_start::timestamptz)::bigint AS resolved_this_week
FROM tickets t
WHERE sqlc.narg(scope_agent_id)::uuid IS NULL
   OR EXISTS (SELECT 1 FROM client_agents ca
              WHERE ca.client_id = t.client_id AND ca.user_id = sqlc.narg(scope_agent_id)::uuid);
