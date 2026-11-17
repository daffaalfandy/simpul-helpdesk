package tickets

import (
	"context"
	"log/slog"
	"sync"
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
	// lifecycleMaxBatches bounds one run's reading; the next run continues after it.
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
// audit entries, SLA replay and notifications, like a change by hand. It reads the candidates
// without locks and locks only those that are due, re-checking there. Each organization runs on
// its own calendar. One run reads at most lifecycleMaxBatches pages per organization; the next
// run continues after the last ticket it read there and wraps around at the end, so a long tail
// of tickets that are never due can't starve the rest.
type LifecycleScanner struct {
	river.WorkerDefaults[LifecycleScanArgs]
	svc *Service
	log *slog.Logger
	now func() time.Time

	mu   sync.Mutex
	next map[uuid.UUID]*uuid.UUID // per organization, where the next run starts; nil for the first ticket
}

func NewLifecycleScanner(svc *Service, log *slog.Logger) *LifecycleScanner {
	return &LifecycleScanner{svc: svc, log: log, now: time.Now, next: map[uuid.UUID]*uuid.UUID{}}
}

// SetClock replaces the scanner's clock (tests).
func (w *LifecycleScanner) SetClock(now func() time.Time) { w.now = now }

func (w *LifecycleScanner) Work(ctx context.Context, _ *river.Job[LifecycleScanArgs]) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.now()
	orgs, err := w.svc.store.Q().ListOrganizationIDs(ctx)
	if err != nil {
		return err
	}
	for _, org := range orgs {
		if err := w.scan(ctx, org, now); err != nil {
			return err
		}
	}
	return nil
}

// scan moves one organization's overdue tickets, from where its last run stopped.
func (w *LifecycleScanner) scan(ctx context.Context, orgID uuid.UUID, now time.Time) error {
	schedules, err := w.svc.sla.Schedules(ctx, orgID)
	if err != nil {
		return err
	}
	after := w.next[orgID]
	for range lifecycleMaxBatches {
		rows, err := w.svc.store.Q().ListLifecycleCandidates(ctx, db.ListLifecycleCandidatesParams{
			OrganizationID: orgID, Statuses: statusStrings(lifecycleStatuses), ChangedBefore: now.Add(-schedules.WorkingDay),
			After: after, RowLimit: lifecycleBatchSize,
		})
		if err != nil {
			return err
		}
		for _, r := range rows {
			due, err := deadlinePassed(ticket.Status(r.Status), r.ResolvedAt, r.StatusChangedAt, r.DodConfig, r.DodOverride, schedules, now)
			if err == nil && due {
				err = w.svc.passDeadline(ctx, schedules, r.ID, now)
			}
			if err != nil {
				// One broken ticket never holds back the others; the next run tries it again.
				w.log.ErrorContext(ctx, "lifecycle scan: ticket skipped", "ticket_id", r.ID, "error", err)
			}
		}
		if len(rows) < lifecycleBatchSize {
			delete(w.next, orgID) // read to the end: the next run starts over
			return nil
		}
		last := rows[len(rows)-1].ID
		after = &last
	}
	w.next[orgID] = after
	return nil
}

// deadlinePassed reports whether a ticket is past its client deadline: In UAT past its
// auto-approve time, or resolved past its reopen window. Both are strictly after the instant.
func deadlinePassed(status ticket.Status, resolvedAt *time.Time, statusChangedAt time.Time, projectDoD, override []byte,
	schedules sla.Schedules, now time.Time) (bool, error) {
	switch status {
	case ticket.StatusInUAT:
		eff, _, err := dod.ParseEffective(projectDoD, override)
		if err != nil {
			return false, err
		}
		at, ok := dod.AutoApproveAt(eff, statusChangedAt, schedules)
		return ok && now.After(at), nil
	case ticket.StatusResolved:
		return resolvedAt != nil && now.After(ticket.ReopenUntil(*resolvedAt, schedules)), nil
	}
	return false, nil
}

// passDeadline re-checks one ticket under its lock and moves it if a client deadline has
// passed: an overdue test is approved, an ended reopen window closes the ticket.
func (s *Service) passDeadline(ctx context.Context, schedules sla.Schedules, id uuid.UUID, now time.Time) error {
	return s.store.Tx(ctx, func(tx store.Tx) error {
		row, err := tx.Q.GetTicketForLifecycle(ctx, id)
		if err != nil {
			return err
		}
		from := ticket.Status(row.Status)
		due, err := deadlinePassed(from, row.ResolvedAt, row.StatusChangedAt, row.DodConfig, row.DodOverride, schedules, now)
		if err != nil || !due {
			return err // the ticket moved on since it was listed, or its deadline moved
		}
		cs := newChangeSet(row.ID)
		actor := audit.SystemActor(ActorNameLifecycle).In(row.OrganizationID)
		move := statusMove{from: from, to: ticket.StatusClosed, cause: ticket.CauseAutoClosed}
		if from == ticket.StatusInUAT {
			eff, _, err := dod.ParseEffective(row.DodConfig, row.DodOverride)
			if err != nil {
				return err
			}
			if err := tickClientItems(ctx, tx.Q, cs, actor, eff, nil); err != nil {
				return err
			}
			move = statusMove{from: from, to: ticket.StatusResolved, cause: ticket.CauseUATAutoApproved}
		}
		if err := s.writeStatus(ctx, tx, cs, actor, move); err != nil {
			return err
		}
		return s.commit(ctx, tx, cs)
	})
}
