package tickets

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/notification"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/slaengine"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// SLA scan: audit actions it writes, and how often it runs.
const (
	ActionSLAAtRisk   = "ticket.sla_at_risk"
	ActionSLABreached = "ticket.sla_breached"
	// ActorNameSLA names the scan in audit entries.
	ActorNameSLA = "sla"

	JobKindSLAScan  = "sla_scan"
	SLAScanInterval = time.Minute
	slaScanAttempts = 3
	// slaScanMaxBatches bounds one run's backfill; the next run continues.
	slaScanMaxBatches = 50

	dataClock  = "clock"
	dataDueAt  = "due_at"
	dataLeftMs = "left_ms"
)

// SLAScanArgs flags clocks that turned at risk or breached and alerts about them.
type SLAScanArgs struct{}

func (SLAScanArgs) Kind() string { return JobKindSLAScan }

// InsertOpts keeps one scan per interval.
func (SLAScanArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: slaScanAttempts, UniqueOpts: river.UniqueOpts{ByPeriod: SLAScanInterval}}
}

// SLAScanJob runs the scan every minute. Register it only on clients that run jobs.
func SLAScanJob() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(SLAScanInterval),
		func() (river.JobArgs, *river.InsertOpts) { return SLAScanArgs{}, nil },
		&river.PeriodicJobOpts{ID: JobKindSLAScan, RunOnStart: true})
}

// SLAScanner backfills tickets the engine hasn't run for (silently), then flags clocks whose
// due or at-risk time has passed, writing an audit entry and queuing alerts for each. Breaches go
// first, so a clock that crossed both since the last run alerts once, as breached.
type SLAScanner struct {
	river.WorkerDefaults[SLAScanArgs]
	store *store.Store
	sla   *sla.Service
	jobs  JobInserter
	log   *slog.Logger
	now   func() time.Time
}

func NewSLAScanner(st *store.Store, slaSvc *sla.Service, jobs JobInserter, log *slog.Logger) *SLAScanner {
	return &SLAScanner{store: st, sla: slaSvc, jobs: jobs, log: log, now: time.Now}
}

// SetClock replaces the scanner's clock (tests).
func (w *SLAScanner) SetClock(now func() time.Time) { w.now = now }

func (w *SLAScanner) Work(ctx context.Context, _ *river.Job[SLAScanArgs]) error {
	now := w.now()
	schedules, err := w.sla.Schedules(ctx)
	if err != nil {
		return err
	}
	if err := w.backfill(ctx, schedules, now); err != nil {
		return err
	}
	q := w.store.Q()
	breaches, err := q.ListSLABreaches(ctx, db.ListSLABreachesParams{
		FirstResponseClock: string(slaengine.ClockFirstResponse), ResolutionClock: string(slaengine.ClockResolution),
		Now: now, RowLimit: slaBatchSize,
	})
	if err != nil {
		return err
	}
	for _, b := range breaches {
		w.logFailure(ctx, b.TicketID, w.flag(ctx, schedules, b.TicketID, slaengine.Clock(b.Clock), true, now))
	}
	atRisk, err := q.ListSLAAtRisk(ctx, db.ListSLAAtRiskParams{
		FirstResponseClock: string(slaengine.ClockFirstResponse), ResolutionClock: string(slaengine.ClockResolution),
		Now: now, RowLimit: slaBatchSize,
	})
	if err != nil {
		return err
	}
	for _, r := range atRisk {
		w.logFailure(ctx, r.TicketID, w.flag(ctx, schedules, r.TicketID, slaengine.Clock(r.Clock), false, now))
	}
	return nil
}

// logFailure records a ticket the scan couldn't handle and lets the scan go on, so one broken
// ticket never stops every other ticket's alerts. The next run tries it again.
func (w *SLAScanner) logFailure(ctx context.Context, ticketID uuid.UUID, err error) {
	if err != nil {
		w.log.ErrorContext(ctx, "sla scan: ticket skipped", "ticket_id", ticketID, "error", err)
	}
}

// backfill replays tickets the engine hasn't seen. A ticket that fails is logged and skipped
// for this run (it stays unreplayed), and the rest go on.
func (w *SLAScanner) backfill(ctx context.Context, schedules sla.Schedules, now time.Time) error {
	var after *uuid.UUID
	for range slaScanMaxBatches {
		ids, err := w.store.Q().ListUnreplayedTicketIDs(ctx, db.ListUnreplayedTicketIDsParams{After: after, RowLimit: slaBatchSize})
		if err != nil {
			return err
		}
		for _, id := range ids {
			w.logFailure(ctx, id, replayLocked(ctx, w.store, schedules, id, now))
		}
		if len(ids) < slaBatchSize {
			return nil
		}
		last := ids[len(ids)-1]
		after = &last
	}
	return nil
}

// replayLocked replays one ticket under its row lock, like any ticket change.
func replayLocked(ctx context.Context, st *store.Store, schedules sla.Schedules, id uuid.UUID, now time.Time) error {
	return st.Tx(ctx, func(tx store.Tx) error {
		if _, err := tx.Q.LockTicketForSLA(ctx, id); err != nil {
			return err
		}
		return ReplaySLA(ctx, tx.Q, schedules, id, now)
	})
}

// flag marks one clock at risk or breached, if it still is under the ticket lock, and queues
// the alerts: the notification fan-out, and for a P1 resolution breach the client contact's
// email when the project asks for it.
func (w *SLAScanner) flag(ctx context.Context, schedules sla.Schedules, ticketID uuid.UUID, clock slaengine.Clock, breach bool, now time.Time) error {
	return w.store.Tx(ctx, func(tx store.Tx) error {
		if _, err := tx.Q.LockTicketForSLA(ctx, ticketID); err != nil {
			return err
		}
		row, err := tx.Q.GetTicketSLA(ctx, ticketID)
		if err != nil {
			return err
		}
		t, err := tx.Q.GetTicketForAlert(ctx, ticketID)
		if err != nil {
			return err
		}
		fr, res := storedClocks(row)
		stored, flagAt, flaggedAt := &fr, &row.FirstResponseAtRiskFlaggedAt, row.FirstResponseAtRiskAt
		dueAt, breachedAt := row.FirstResponseDueAt, &row.FirstResponseBreachedAt
		if clock == slaengine.ClockResolution {
			stored, flagAt, flaggedAt = &res, &row.ResolutionAtRiskFlaggedAt, row.ResolutionAtRiskAt
			dueAt, breachedAt = row.ResolutionDueAt, &row.ResolutionBreachedAt
		}
		action := ActionSLAAtRisk
		switch {
		case breach && *breachedAt == nil && dueAt != nil && dueAt.Before(now):
			*breachedAt, action = dueAt, ActionSLABreached
		case !breach && *flagAt == nil && *breachedAt == nil && flaggedAt != nil && !flaggedAt.After(now):
			*flagAt = flaggedAt
		default:
			return nil // a change got there first
		}
		if err := upsertClocks(ctx, tx.Q, row); err != nil {
			return err
		}
		view := slaengine.ViewAt(*stored, schedules, ticket.Status(t.Status).IsOpen(), now)
		cs := newChangeSet(ticketID)
		if err := cs.write(ctx, tx.Q, audit.SystemActor(ActorNameSLA).Entry(action, TargetTicket, &ticketID, map[string]any{
			dataClock: clock, dataDueAt: dueAt, dataLeftMs: view.Remaining.Milliseconds(),
		})); err != nil {
			return err
		}
		if _, err := w.jobs.InsertTx(ctx, tx.PG, notification.FanoutArgs{TicketID: ticketID, AuditIDs: cs.auditIDs},
			&river.InsertOpts{MaxAttempts: fanoutMaxAttempts}); err != nil {
			return err
		}
		if action == ActionSLABreached && clock == slaengine.ClockResolution && w.alertsPIC(t) {
			_, err := w.jobs.InsertTx(ctx, tx.PG, notification.PicEmailArgs{TicketID: ticketID},
				&river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}})
			return err
		}
		return nil
	})
}

// alertsPIC reports whether a P1 resolution breach emails the client's contact person: the
// project switched it on and the client has one.
func (w *SLAScanner) alertsPIC(t db.GetTicketForAlertRow) bool {
	if sla.Priority(t.Priority) != sla.P1 || t.PicEmail == nil || *t.PicEmail == "" {
		return false
	}
	snap, err := parseSnapshot(t.SlaSnapshot)
	return err == nil && snap.NotifyPicOnP1Breach
}

// upsertClocks writes a ticket_sla row back as it is.
func upsertClocks(ctx context.Context, q *db.Queries, r db.TicketSla) error {
	return q.UpsertTicketSLA(ctx, db.UpsertTicketSLAParams(r))
}
