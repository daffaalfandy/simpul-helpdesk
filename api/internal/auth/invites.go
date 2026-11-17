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

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/mail"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/organization"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/reqinfo"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
)

const (
	JobKindInvite        = "send_invite"
	inviteJobMaxAttempts = 5
	// inviteJobUniqueWindow folds a double click on "Resend invite" into one email.
	inviteJobUniqueWindow = time.Minute

	// DefaultInviteTTL is how long an invite link works (owner, 17 Nov: a week to get to it).
	DefaultInviteTTL = 7 * 24 * time.Hour

	// AcceptInvitePath is the web app route that handles invite links; the token goes in the
	// fragment, like reset links.
	AcceptInvitePath = "accept-invite"
)

// SendInviteArgs queues an invite email. Like resets, the worker mints the token, so it never
// sits in job args.
type SendInviteArgs struct {
	UserID    uuid.UUID  `json:"user_id"`
	InvitedBy *uuid.UUID `json:"invited_by,omitempty"`
}

func (SendInviteArgs) Kind() string { return JobKindInvite }

func (SendInviteArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: inviteJobMaxAttempts,
		UniqueOpts:  river.UniqueOpts{ByArgs: true, ByPeriod: inviteJobUniqueWindow},
	}
}

// InviteEmailWorker mints an invite token and emails the set-password link.
type InviteEmailWorker struct {
	river.WorkerDefaults[SendInviteArgs]
	store  *store.Store
	mailer mail.Sender
	appURL *url.URL
	ttl    time.Duration
	now    func() time.Time
}

func NewInviteEmailWorker(st *store.Store, mailer mail.Sender, appURL *url.URL, ttl time.Duration) *InviteEmailWorker {
	return &InviteEmailWorker{store: st, mailer: mailer, appURL: appURL, ttl: ttl, now: time.Now}
}

// Work runs the job. Each run mints a fresh link and makes the earlier ones stop working. An
// account that already set its password, or was suspended meanwhile, gets nothing.
func (w *InviteEmailWorker) Work(ctx context.Context, job *river.Job[SendInviteArgs]) error {
	var msg *mail.Message
	err := w.store.Tx(ctx, func(tx store.Tx) error {
		user, err := tx.Q.GetUserByID(ctx, job.Args.UserID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if UserStatus(user.Status) != UserStatusInvited {
			return nil
		}
		first, err := tx.Q.GetFirstOrganization(ctx)
		if err != nil {
			return err
		}
		org := db.GetOrganizationRow(first)
		inviter := ""
		if job.Args.InvitedBy != nil {
			by, err := tx.Q.GetUserByID(ctx, *job.Args.InvitedBy)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			inviter = by.Name
		}
		if err := tx.Q.InvalidateUserInvites(ctx, user.ID); err != nil {
			return err
		}
		raw, hash, err := NewToken()
		if err != nil {
			return err
		}
		if err := tx.Q.CreateUserInvite(ctx, db.CreateUserInviteParams{
			TokenHash: hash, UserID: user.ID, InvitedBy: job.Args.InvitedBy, ExpiresAt: w.now().Add(w.ttl),
		}); err != nil {
			return err
		}
		client := ""
		if user.ClientName != nil {
			client = *user.ClientName
		}
		m, err := mail.Render(mail.TemplateInvite, user.Email, user.Name, mail.InviteData{
			Name: user.Name, OrgName: org.Name, LogoURL: organization.LogoURL(w.appURL, org), InviterName: inviter,
			Role: user.Role, ClientName: client, Link: InviteLink(w.appURL, raw), ExpiresIn: mail.HumanDuration(w.ttl),
		})
		if err != nil {
			return err
		}
		msg = &m
		return nil
	})
	if err != nil || msg == nil {
		return err
	}
	if err := w.mailer.Send(ctx, *msg); err != nil {
		return fmt.Errorf("send invite email: %w", err)
	}
	return nil
}

// InviteLink builds the set-password URL; the token is in the fragment so it never reaches
// server logs or Referer headers.
func InviteLink(appURL *url.URL, rawToken string) string {
	return appURL.JoinPath(AcceptInvitePath).String() + ResetTokenFragment + rawToken
}

// QueueInvite sends a user their invite from inside the transaction that created or changed
// them. by is the admin who asked; nil for the system.
func (s *Service) QueueInvite(ctx context.Context, tx store.Tx, userID uuid.UUID, by *uuid.UUID) error {
	if _, err := s.jobs.InsertTx(ctx, tx.PG, SendInviteArgs{UserID: userID, InvitedBy: by}, nil); err != nil {
		return fmt.Errorf("enqueue invite email: %w", err)
	}
	return nil
}

// AcceptInvite sets the first password from an invite link, activates the account and signs it
// in. A wrong or used token counts against the caller's address like a failed sign-in.
func (s *Service) AcceptInvite(ctx context.Context, token, password string) (*Principal, NewSession, error) {
	if err := ValidatePassword(password); err != nil {
		return nil, NewSession{}, err
	}
	info := reqinfo.From(ctx)
	limitKey := ""
	if info.IP.IsValid() {
		limitKey = keyPrefixInviteIP + info.IP.String()
		if err := s.checkLimit(ctx, limitKey, s.opts.MaxFailuresPerIP, s.opts.LoginWindow); err != nil {
			return nil, NewSession{}, err
		}
	}
	hash, err := s.hasher.Hash(ctx, password)
	if err != nil {
		return nil, NewSession{}, err
	}
	var p *Principal
	var sess NewSession
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		userID, err := tx.Q.ConsumeUserInvite(ctx, db.ConsumeUserInviteParams{TokenHash: HashToken(token), Invited: string(UserStatusInvited)})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.InvalidToken()
		}
		if err != nil {
			return err
		}
		if err := tx.Q.SetUserPassword(ctx, db.SetUserPasswordParams{ID: userID, PasswordHash: &hash}); err != nil {
			return err
		}
		if err := tx.Q.SetUserStatus(ctx, db.SetUserStatusParams{ID: userID, Status: string(UserStatusActive)}); err != nil {
			return err
		}
		if err := tx.Q.InvalidateUserInvites(ctx, userID); err != nil {
			return err
		}
		user, err := tx.Q.GetUserByID(ctx, userID)
		if err != nil {
			return err
		}
		p = principalOf(user)
		if err := audit.Write(ctx, tx.Q, p.AuditActor().Entry(ActionInviteAccepted, TargetUser, &userID, nil)); err != nil {
			return err
		}
		sess, err = s.startSession(ctx, tx, p, info)
		return err
	})
	if apperr.IsCode(err, apperr.CodeInvalidToken) {
		if limitKey != "" {
			if rerr := s.store.Q().RecordLoginAttempt(ctx, limitKey); rerr != nil {
				return nil, NewSession{}, rerr
			}
		}
		return nil, NewSession{}, err
	}
	if err != nil {
		return nil, NewSession{}, err
	}
	return p, sess, nil
}

func principalOf(u db.GetUserByIDRow) *Principal {
	return &Principal{
		UserID: u.ID, Email: u.Email, Name: u.Name, Role: Role(u.Role),
		JobTitle: u.JobTitle, ClientID: u.ClientID, ClientName: u.ClientName,
	}
}
