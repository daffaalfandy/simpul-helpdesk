package auth

import (
	"context"
	"time"

	"github.com/riverqueue/river"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
)

const (
	JobKindPurge     = "auth_purge"
	PurgeInterval    = time.Hour
	purgeMaxAttempts = 3
)

// PurgeArgs deletes expired sessions, spent or expired reset links and old sign-in attempts.
// Sessions are otherwise deleted only when someone presents them, so abandoned ones (with their
// IP address and browser) would stay forever.
type PurgeArgs struct{}

func (PurgeArgs) Kind() string { return JobKindPurge }

// InsertOpts keeps one purge per interval.
func (PurgeArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: purgeMaxAttempts, UniqueOpts: river.UniqueOpts{ByPeriod: PurgeInterval}}
}

// PurgeJob schedules the purge. Register it only on clients that run jobs.
func PurgeJob() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(PurgeInterval),
		func() (river.JobArgs, *river.InsertOpts) { return PurgeArgs{}, nil },
		&river.PeriodicJobOpts{ID: JobKindPurge, RunOnStart: true})
}

// PurgeWorker runs the purge. Expired credentials are not audited when they go, as an expired
// session deleted at sign-in isn't either.
type PurgeWorker struct {
	river.WorkerDefaults[PurgeArgs]
	store *store.Store
	now   func() time.Time
}

func NewPurgeWorker(st *store.Store) *PurgeWorker { return &PurgeWorker{store: st, now: time.Now} }

func (w *PurgeWorker) Work(ctx context.Context, _ *river.Job[PurgeArgs]) error {
	now := w.now()
	q := w.store.Q()
	if err := q.PurgeExpiredSessions(ctx, now); err != nil {
		return err
	}
	if err := q.PurgeSpentResetTokens(ctx, now); err != nil {
		return err
	}
	return q.PruneLoginAttempts(ctx, now.Add(-loginAttemptRetention))
}
