package auth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"simdesk/internal/db"
	"simdesk/internal/mail"
	"simdesk/internal/store"
)

const (
	// JobKindPasswordReset identifies the reset-email job in the queue.
	JobKindPasswordReset = "send_password_reset"
	resetJobMaxAttempts  = 5
	resetJobUniqueWindow = 5 * time.Minute

	// ResetPasswordPath is the web app route that handles reset links.
	ResetPasswordPath = "reset-password"
	// ResetTokenFragment prefixes the token in the link's URL fragment.
	ResetTokenFragment = "#token="
)

// SendPasswordResetArgs queues a password-reset email. The raw token is never
// stored in job args (River keeps args after completion): the worker mints it.
type SendPasswordResetArgs struct {
	UserID uuid.UUID `json:"user_id"`
}

// Kind identifies the job type.
func (SendPasswordResetArgs) Kind() string { return JobKindPasswordReset }

// InsertOpts de-duplicates repeated requests for the same user.
func (SendPasswordResetArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: resetJobMaxAttempts,
		UniqueOpts:  river.UniqueOpts{ByArgs: true, ByPeriod: resetJobUniqueWindow},
	}
}

// ResetEmailWorker mints a reset token and emails the link.
type ResetEmailWorker struct {
	river.WorkerDefaults[SendPasswordResetArgs]
	store  *store.Store
	mailer mail.Sender
	appURL *url.URL
	ttl    time.Duration
	now    func() time.Time
}

// NewResetEmailWorker builds the worker. appURL is the public web app URL.
func NewResetEmailWorker(st *store.Store, mailer mail.Sender, appURL *url.URL, ttl time.Duration) *ResetEmailWorker {
	return &ResetEmailWorker{store: st, mailer: mailer, appURL: appURL, ttl: ttl, now: time.Now}
}

// Work runs the job. A retry mints a fresh token and invalidates older ones.
func (w *ResetEmailWorker) Work(ctx context.Context, job *river.Job[SendPasswordResetArgs]) error {
	var msg *mail.Message
	err := w.store.Tx(ctx, func(tx store.Tx) error {
		user, err := tx.Q.GetUserByID(ctx, job.Args.UserID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if !IsActive(user.Status) {
			return nil
		}
		org, err := tx.Q.GetOrganization(ctx)
		if err != nil {
			return err
		}
		if err := tx.Q.InvalidateUserResetTokens(ctx, user.ID); err != nil {
			return err
		}
		raw, hash, err := NewToken()
		if err != nil {
			return err
		}
		if err := tx.Q.CreatePasswordResetToken(ctx, db.CreatePasswordResetTokenParams{
			TokenHash: hash, UserID: user.ID, ExpiresAt: w.now().Add(w.ttl),
		}); err != nil {
			return err
		}
		m, err := mail.Render(mail.TemplatePasswordReset, user.Email, user.Name, mail.PasswordResetData{
			Name: user.Name, OrgName: org.Name, Link: ResetLink(w.appURL, raw), ExpiresIn: mail.HumanDuration(w.ttl),
		})
		if err != nil {
			return err
		}
		msg = &m
		return nil
	})
	if err != nil {
		return err
	}
	if msg == nil {
		return nil // user gone or not active
	}
	if err := w.mailer.Send(ctx, *msg); err != nil {
		return fmt.Errorf("send reset email: %w", err)
	}
	return nil
}

// ResetLink builds the reset URL. The token goes in the fragment so it never
// reaches server logs or Referer headers.
func ResetLink(appURL *url.URL, rawToken string) string {
	return appURL.JoinPath(ResetPasswordPath).String() + ResetTokenFragment + rawToken
}
