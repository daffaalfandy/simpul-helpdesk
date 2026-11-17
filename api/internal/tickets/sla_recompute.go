package tickets

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// slaBatchSize is how many tickets one recompute or scan step takes at a time.
const slaBatchSize = 100

// SLARecomputer replays every open ticket's clocks on the current calendar. Breaches already
// flagged stay (owner, 15 Nov).
type SLARecomputer struct {
	river.WorkerDefaults[sla.RecomputeArgs]
	store *store.Store
	sla   *sla.Service
	log   *slog.Logger
	now   func() time.Time
}

func NewSLARecomputer(st *store.Store, slaSvc *sla.Service, log *slog.Logger) *SLARecomputer {
	return &SLARecomputer{store: st, sla: slaSvc, log: log, now: time.Now}
}

func (w *SLARecomputer) Work(ctx context.Context, job *river.Job[sla.RecomputeArgs]) error {
	orgs := []uuid.UUID{job.Args.OrganizationID}
	if job.Args.OrganizationID == uuid.Nil {
		var err error
		if orgs, err = w.store.Q().ListOrganizationIDs(ctx); err != nil {
			return err
		}
	}
	for _, org := range orgs {
		if err := w.recompute(ctx, org); err != nil {
			return err
		}
	}
	return nil
}

// recompute replays one organization's open tickets on its current calendar.
func (w *SLARecomputer) recompute(ctx context.Context, orgID uuid.UUID) error {
	cals := sla.NewCalendars()
	var after *uuid.UUID
	for {
		ids, err := w.store.Q().ListOpenTicketIDsAfter(ctx, db.ListOpenTicketIDsAfterParams{
			OrganizationID: orgID, OpenStatuses: statusStrings(ticket.OpenStatuses), After: after, RowLimit: slaBatchSize,
		})
		if err != nil {
			return err
		}
		for _, id := range ids {
			// One ticket that can't be replayed must not stop the others.
			if err := replayLocked(ctx, w.store, cals, id, w.now()); err != nil {
				w.log.ErrorContext(ctx, "sla recompute: ticket skipped", "ticket_id", id, "error", err)
			}
		}
		if len(ids) < slaBatchSize {
			return nil
		}
		last := ids[len(ids)-1]
		after = &last
	}
}
