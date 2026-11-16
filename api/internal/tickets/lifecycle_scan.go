package tickets

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// Lifecycle scan: the client's deadlines (HANDOFF §6). A test nobody answered is approved, and a
// resolved ticket nobody reopened closes.
const (
	// ActorNameLifecycle names the scan in audit entries.
	ActorNameLifecycle = "lifecycle"

	JobKindLifecycleScan  = "lifecycle_scan"
	LifecycleScanInterval = time.Minute
	lifecycleScanAttempts = 3
	lifecycleBatchSize    = 100
	// lifecycleMaxBatches bounds one run; the next run continues.
	lifecycleMaxBatches = 50
)

// lifecycleStatuses are the statuses with a client deadline.
var lifecycleStatuses = []ticket.Status{ticket.StatusInUAT, ticket.StatusResolved}

// LifecycleScanArgs approves overdue tests and closes tickets whose reopen window ended.
type LifecycleScanArgs struct{}

func (LifecycleScanArgs) Kind() string { return JobKindLifecycleScan }

// InsertOpts keeps one scan per interval.
func (LifecycleScanArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: lifecycleScanAttempts, UniqueOpts: river.UniqueOpts{ByPeriod: LifecycleScanInterval}}
}

// LifecycleScanJob runs the scan every minute. Register it only on clients that run jobs.
func LifecycleScanJob() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(LifecycleScanInterval),
		func() (river.JobArgs, *river.InsertOpts) { return LifecycleScanArgs{}, nil },
		&river.PeriodicJobOpts{ID: JobKindLifecycleScan, RunOnStart: true})
}

// LifecycleScanner moves tickets past a client deadline, each in its own transaction with its
// audit entries, SLA replay and notifications, like a change by hand.
type LifecycleScanner struct {
	river.WorkerDefaults[LifecycleScanArgs]
	svc *Service
	log *slog.Logger
	now func() time.Time
}

func NewLifecycleScanner(svc *Service, log *slog.Logger) *LifecycleScanner {
	return &LifecycleScanner{svc: svc, log: log, now: time.Now}
}

// SetClock replaces the scanner's clock (tests).
func (w *LifecycleScanner) SetClock(now func() time.Time) { w.now = now }

func (w *LifecycleScanner) Work(ctx context.Context, _ *river.Job[LifecycleScanArgs]) error {
	now := w.now()
	schedules, err := w.svc.sla.Schedules(ctx)
	if err != nil {
		return err
	}
	var after *uuid.UUID
	for range lifecycleMaxBatches {
		ids, err := w.svc.store.Q().ListLifecycleCandidates(ctx, db.ListLifecycleCandidatesParams{
			Statuses: statusStrings(lifecycleStatuses), ChangedBefore: now.Add(-schedules.WorkingDay),
			After: after, RowLimit: lifecycleBatchSize,
		})
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := w.svc.passDeadline(ctx, schedules, id, now); err != nil {
				// One broken ticket never holds back the others; the next run tries it again.
				w.log.ErrorContext(ctx, "lifecycle scan: ticket skipped", "ticket_id", id, "error", err)
			}
		}
		if len(ids) < lifecycleBatchSize {
			return nil
		}
		last := ids[len(ids)-1]
		after = &last
	}
	return nil
}

// passDeadline re-checks one ticket under its lock and moves it if a client deadline has
// passed: an overdue test is approved, an ended reopen window closes the ticket.
func (s *Service) passDeadline(ctx context.Context, schedules sla.Schedules, id uuid.UUID, now time.Time) error {
	return s.store.Tx(ctx, func(tx store.Tx) error {
		row, err := tx.Q.GetTicketForLifecycle(ctx, id)
		if err != nil {
			return err
		}
		cs := newChangeSet(row.ID)
		actor := audit.SystemActor(ActorNameLifecycle)
		switch from := ticket.Status(row.Status); from {
		case ticket.StatusInUAT:
			sd, err := parseDoD(row.DodConfig, row.DodOverride)
			if err != nil {
				return err
			}
			eff, _ := sd.effective()
			at, ok := dod.AutoApproveAt(eff, row.StatusChangedAt, schedules)
			if !ok || !now.After(at) {
				return nil
			}
			if err := tickClientItems(ctx, tx.Q, cs, actor, eff, nil); err != nil {
				return err
			}
			if err := s.writeStatus(ctx, tx, cs, actor, statusMove{
				from: from, to: ticket.StatusResolved, cause: ticket.CauseUATAutoApproved,
			}); err != nil {
				return err
			}
		case ticket.StatusResolved:
			if row.ResolvedAt == nil || !now.After(ticket.ReopenUntil(*row.ResolvedAt, schedules)) {
				return nil
			}
			if err := s.writeStatus(ctx, tx, cs, actor, statusMove{
				from: from, to: ticket.StatusClosed, cause: ticket.CauseAutoClosed,
			}); err != nil {
				return err
			}
		default:
			return nil // the ticket moved on since it was listed
		}
		return s.commit(ctx, tx, cs)
	})
}
