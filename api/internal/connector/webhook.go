package connector

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/gitevent"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
)

// MaxDeliveryBytes caps a webhook body. A push carries every commit message, so this is well
// above what a normal push needs; GitHub's own limit is 25 MB.
const MaxDeliveryBytes = 5 << 20

// Background jobs.
const (
	JobKindProcess        = "webhook_process"
	JobKindPurge          = "webhook_purge"
	processMaxAttempts    = 10
	PurgeInterval         = 24 * time.Hour
	DeliveryRetention     = 30 * 24 * time.Hour
	purgeBatch            = 1000
	purgeMaxBatches       = 100
	ActionDeliveriesPurge = "webhook_delivery.purged"
	ActorNamePurge        = "webhook_purge"
	TargetDelivery        = "webhook_delivery"
	dataCount             = "count"
)

// Receipt says what became of a delivery.
type Receipt string

const (
	ReceiptQueued    Receipt = "queued"    // stored; the job reads it
	ReceiptDuplicate Receipt = "duplicate" // received before
	ReceiptPing      Receipt = "ping"      // the provider checking the webhook
	ReceiptIgnored   Receipt = "ignored"   // an event the connector doesn't read
)

// ProcessArgs reads one stored delivery.
type ProcessArgs struct {
	DeliveryID uuid.UUID `json:"delivery_id"`
}

func (ProcessArgs) Kind() string { return JobKindProcess }

func (ProcessArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: processMaxAttempts}
}

// PurgeArgs deletes delivery payloads older than DeliveryRetention.
type PurgeArgs struct{}

func (PurgeArgs) Kind() string { return JobKindPurge }

// PurgeJob runs the purge daily. Register it only on clients that run jobs.
func PurgeJob() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(PurgeInterval),
		func() (river.JobArgs, *river.InsertOpts) { return PurgeArgs{}, nil },
		&river.PeriodicJobOpts{ID: JobKindPurge, RunOnStart: true})
}

// Receive checks and stores one webhook delivery; a job reads it. The secret is checked before
// anything in the body is trusted. Refusals are recorded on the repository so the
// Integrations page can say what is wrong.
func (s *Service) Receive(ctx context.Context, repoID uuid.UUID, h http.Header, body []byte) (Receipt, error) {
	q := s.store.Q()
	repo, err := q.GetRepository(ctx, repoID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && repo.Hidden) {
		return "", apperr.NotFound()
	}
	if err != nil {
		return "", err
	}
	provider := gitevent.Provider(repo.Provider)
	secret, err := s.open(repo.ID, repo.WebhookSecret)
	if err != nil {
		return "", err // ENCRYPTION_KEY changed: an operator problem, logged by the caller
	}
	if err := gitevent.Verify(provider, h, body, secret); err != nil {
		return "", s.refuse(ctx, repo.ID, ErrorBadSignature, apperr.InvalidSignature())
	}
	event := gitevent.EventName(provider, h)
	if !gitevent.Supported(provider, event) {
		return ReceiptIgnored, q.RecordDelivery(ctx, db.RecordDeliveryParams{ID: repo.ID})
	}
	payload, err := gitevent.JSON(body)
	if err != nil {
		return "", s.refuse(ctx, repo.ID, ErrorMalformed, apperr.InvalidRequest(err.Error()))
	}
	d, err := gitevent.Parse(provider, event, payload)
	if err != nil {
		return "", s.refuse(ctx, repo.ID, ErrorMalformed, apperr.InvalidRequest(err.Error()))
	}
	if !sameRepository(repo, d.Repository) {
		return "", s.refuse(ctx, repo.ID, ErrorRepositoryMismatch, apperr.Conflict(apperr.CodeRepositoryMismatch,
			"This webhook belongs to another repository", "Check the repository the webhook was added to."))
	}
	receipt := ReceiptPing
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		if err := tx.Q.RecordDelivery(ctx, db.RecordDeliveryParams{ID: repo.ID, ExternalID: nonEmpty(d.Repository.ExternalID)}); err != nil {
			return err
		}
		if d.Ping {
			return nil
		}
		id, err := tx.Q.InsertDelivery(ctx, db.InsertDeliveryParams{
			RepositoryID: repo.ID, DeliveryID: gitevent.DeliveryID(provider, h, body), EventName: event, Payload: payload,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			receipt = ReceiptDuplicate
			return nil
		}
		if err != nil {
			return err
		}
		receipt = ReceiptQueued
		_, err = s.jobs.InsertTx(ctx, tx.PG, ProcessArgs{DeliveryID: id}, nil)
		return err
	})
	return receipt, err
}

// RefuseTooLarge records a body over MaxDeliveryBytes for a repository that exists.
func (s *Service) RefuseTooLarge(ctx context.Context, repoID uuid.UUID) error {
	return s.refuse(ctx, repoID, ErrorTooLarge, apperr.TooLarge(apperr.CodeRequestTooLarge, "The delivery is too large", ""))
}

// refuse records why a delivery was refused and returns the error for the sender.
func (s *Service) refuse(ctx context.Context, repoID uuid.UUID, reason DeliveryError, err error) error {
	r := string(reason)
	if rerr := s.store.Q().RecordDeliveryError(ctx, db.RecordDeliveryErrorParams{ID: repoID, LastError: &r}); rerr != nil {
		return rerr
	}
	return err
}

// sameRepository reports whether a delivery came from the connected repository: by the
// provider's id once it is known (it survives renames), by name before.
func sameRepository(repo db.GetRepositoryRow, from gitevent.Repository) bool {
	if repo.ExternalID != nil && from.ExternalID != "" {
		return *repo.ExternalID == from.ExternalID
	}
	return strings.EqualFold(repo.FullName, from.FullName)
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// PurgeWorker deletes delivery payloads older than DeliveryRetention. Code events keep what
// tickets need; their link to the delivery is cleared.
type PurgeWorker struct {
	river.WorkerDefaults[PurgeArgs]
	store *store.Store
	now   func() time.Time
}

func NewPurgeWorker(st *store.Store) *PurgeWorker { return &PurgeWorker{store: st, now: time.Now} }

func (w *PurgeWorker) Work(ctx context.Context, _ *river.Job[PurgeArgs]) error {
	cutoff := w.now().Add(-DeliveryRetention)
	for range purgeMaxBatches {
		var n int64
		err := w.store.Tx(ctx, func(tx store.Tx) error {
			var err error
			n, err = tx.Q.DeleteOldDeliveries(ctx, db.DeleteOldDeliveriesParams{ReceivedBefore: cutoff, Batch: purgeBatch})
			if err != nil || n == 0 {
				return err
			}
			return audit.Write(ctx, tx.Q, audit.SystemActor(ActorNamePurge).Entry(
				ActionDeliveriesPurge, TargetDelivery, nil, map[string]any{dataCount: n}))
		})
		if err != nil || n < purgeBatch {
			return err
		}
	}
	return nil
}
