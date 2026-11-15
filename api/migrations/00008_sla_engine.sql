-- +goose Up

-- Each ticket's two SLA clocks, as the engine (internal/slaengine) last replayed them. A table of
-- its own because updating tickets would move the updated_at requesters see. No row means the
-- ticket hasn't been replayed yet; the SLA scan backfills it.
--
-- Per clock: the budget and the calendar it is measured on (for the current priority); working
-- time used in finished intervals; running_since while it counts; due_at and at_risk_at while it
-- runs (the scan's indexed instants); the scan's flags; met_at once met.
CREATE TABLE ticket_sla (
  ticket_id                         uuid PRIMARY KEY CONSTRAINT ticket_sla_ticket_id_fkey REFERENCES tickets (id) ON DELETE CASCADE,
  computed_at                       timestamptz NOT NULL DEFAULT now(),
  first_response_budget_ms          bigint NOT NULL CHECK (first_response_budget_ms > 0),
  first_response_calendar           text NOT NULL CHECK (first_response_calendar IN ('business_hours', 'always')),
  first_response_elapsed_ms         bigint NOT NULL DEFAULT 0 CHECK (first_response_elapsed_ms >= 0),
  first_response_running_since      timestamptz,
  first_response_due_at             timestamptz,
  first_response_at_risk_at         timestamptz,
  first_response_at_risk_flagged_at timestamptz,
  first_response_breached_at        timestamptz,
  first_response_met_at             timestamptz,
  resolution_budget_ms              bigint NOT NULL CHECK (resolution_budget_ms > 0),
  resolution_calendar               text NOT NULL CHECK (resolution_calendar IN ('business_hours', 'always')),
  resolution_elapsed_ms             bigint NOT NULL DEFAULT 0 CHECK (resolution_elapsed_ms >= 0),
  resolution_running_since          timestamptz,
  resolution_due_at                 timestamptz,
  resolution_at_risk_at             timestamptz,
  resolution_at_risk_flagged_at     timestamptz,
  resolution_breached_at            timestamptz,
  resolution_met_at                 timestamptz,
  -- Due and at-risk instants exist only while a clock runs.
  CONSTRAINT ticket_sla_first_response_running CHECK (
    first_response_running_since IS NOT NULL OR (first_response_due_at IS NULL AND first_response_at_risk_at IS NULL)),
  CONSTRAINT ticket_sla_resolution_running CHECK (
    resolution_running_since IS NOT NULL OR (resolution_due_at IS NULL AND resolution_at_risk_at IS NULL))
);
CREATE INDEX ticket_sla_first_response_at_risk_idx ON ticket_sla (first_response_at_risk_at)
  WHERE first_response_at_risk_flagged_at IS NULL AND first_response_breached_at IS NULL AND first_response_at_risk_at IS NOT NULL;
CREATE INDEX ticket_sla_first_response_due_idx ON ticket_sla (first_response_due_at)
  WHERE first_response_breached_at IS NULL AND first_response_due_at IS NOT NULL;
CREATE INDEX ticket_sla_resolution_at_risk_idx ON ticket_sla (resolution_at_risk_at)
  WHERE resolution_at_risk_flagged_at IS NULL AND resolution_breached_at IS NULL AND resolution_at_risk_at IS NOT NULL;
CREATE INDEX ticket_sla_resolution_due_idx ON ticket_sla (resolution_due_at)
  WHERE resolution_breached_at IS NULL AND resolution_due_at IS NOT NULL;

-- +goose Down
DROP TABLE ticket_sla;
