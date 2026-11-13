package tickets

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/storage"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
)

const (
	JobKindAttachmentSweep = "purge_pending_attachments"
	// ActorNameAttachmentSweep names the job in audit entries.
	ActorNameAttachmentSweep = "attachment_sweep"
	sweepMaxAttempts         = 3
	sweepBatchSize           = 100
	// sweepMaxBatches bounds one run; the next run picks up the rest.
	sweepMaxBatches = 50
)

// PurgePendingAttachmentsArgs deletes uploads nobody linked within PendingAttachmentTTL.
type PurgePendingAttachmentsArgs struct{}

func (PurgePendingAttachmentsArgs) Kind() string { return JobKindAttachmentSweep }

// InsertOpts keeps one sweep per interval.
func (PurgePendingAttachmentsArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: sweepMaxAttempts,
		UniqueOpts:  river.UniqueOpts{ByPeriod: AttachmentSweepInterval},
	}
}

type AttachmentSweeper struct {
	river.WorkerDefaults[PurgePendingAttachmentsArgs]
	store *store.Store
	files storage.Store
	now   func() time.Time
}

func NewAttachmentSweeper(st *store.Store, files storage.Store) *AttachmentSweeper {
	return &AttachmentSweeper{store: st, files: files, now: time.Now}
}

// AttachmentSweepJob schedules the sweep. Register it only on clients that run jobs.
func AttachmentSweepJob() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(AttachmentSweepInterval),
		func() (river.JobArgs, *river.InsertOpts) { return PurgePendingAttachmentsArgs{}, nil },
		&river.PeriodicJobOpts{ID: JobKindAttachmentSweep, RunOnStart: true})
}

// Work deletes expired pending uploads, files first: an expired row can't be linked or
// downloaded, so a crash between the two steps leaves nothing reachable.
func (w *AttachmentSweeper) Work(ctx context.Context, _ *river.Job[PurgePendingAttachmentsArgs]) error {
	cutoff := w.now().Add(-PendingAttachmentTTL)
	for range sweepMaxBatches {
		n, err := w.sweepBatch(ctx, cutoff)
		if err != nil {
			return err
		}
		if n < sweepBatchSize {
			break
		}
	}
	_, err := w.files.PurgeTemp(ctx, PendingAttachmentTTL)
	return err
}

func (w *AttachmentSweeper) sweepBatch(ctx context.Context, cutoff time.Time) (int, error) {
	var n int
	err := w.store.Tx(ctx, func(tx store.Tx) error {
		rows, err := tx.Q.ListExpiredPendingAttachments(ctx, db.ListExpiredPendingAttachmentsParams{
			Cutoff: cutoff, BatchSize: sweepBatchSize,
		})
		if err != nil || len(rows) == 0 {
			return err
		}
		n = len(rows)
		ids := make([]uuid.UUID, 0, n)
		for _, r := range rows {
			if err := w.files.Delete(ctx, r.StorageKey); err != nil {
				return err
			}
			ids = append(ids, r.ID)
		}
		if err := tx.Q.DeleteAttachments(ctx, ids); err != nil {
			return err
		}
		return audit.Write(ctx, tx.Q, audit.SystemActor(ActorNameAttachmentSweep).Entry(
			ActionAttachmentsPurged, TargetAttachment, nil, map[string]any{dataAttachmentIDs: ids, dataCount: n}))
	})
	return n, err
}
