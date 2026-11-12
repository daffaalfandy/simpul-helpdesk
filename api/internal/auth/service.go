// Package auth handles passwords, sessions, password resets and role checks.
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"simdesk/internal/apperr"
	"simdesk/internal/audit"
	"simdesk/internal/db"
	"simdesk/internal/reqinfo"
	"simdesk/internal/store"
)

// Audit actions written by this package.
const (
	ActionLogin                  = "auth.login"
	ActionLoginFailed            = "auth.login_failed"
	ActionLogout                 = "auth.logout"
	ActionPasswordResetRequested = "auth.password_reset_requested"
	ActionPasswordReset          = "auth.password_reset"
	ActionUserCreated            = "user.created"

	// TargetUser is the audit target type for users.
	TargetUser = "user"
	// actorCLI names the actor for changes made from the command line.
	actorCLI = "cli"
	// auditDataRole is the audit data key holding a role.
	auditDataRole = "role"
)

// Default session and rate-limit settings.
const (
	DefaultSessionIdle         = 7 * 24 * time.Hour
	DefaultSessionAbsolute     = 30 * 24 * time.Hour
	DefaultTouchInterval       = 5 * time.Minute
	DefaultLoginWindow         = 15 * time.Minute
	DefaultMaxFailuresPerEmail = 5
	DefaultMaxFailuresPerIP    = 20
	DefaultResetWindow         = time.Hour
	DefaultMaxResetsPerEmail   = 3
	DefaultResetTokenTTL       = time.Hour

	// loginAttemptRetention is how long attempt rows are kept before pruning.
	loginAttemptRetention = 24 * time.Hour
	// minRetryAfter is the smallest Retry-After sent with a 429.
	minRetryAfter = time.Second
	// emailAt is the character every email address must contain.
	emailAt = "@"
)

// Options tune sessions and rate limits.
type Options struct {
	SessionIdle         time.Duration // session ends after this long without activity
	SessionAbsolute     time.Duration // session ends this long after sign-in, regardless of activity
	TouchInterval       time.Duration // how often last_seen_at is refreshed
	LoginWindow         time.Duration
	MaxFailuresPerEmail int
	MaxFailuresPerIP    int
	ResetWindow         time.Duration
	MaxResetsPerEmail   int
	ResetTokenTTL       time.Duration
}

// DefaultOptions are the production defaults.
func DefaultOptions() Options {
	return Options{
		SessionIdle:         DefaultSessionIdle,
		SessionAbsolute:     DefaultSessionAbsolute,
		TouchInterval:       DefaultTouchInterval,
		LoginWindow:         DefaultLoginWindow,
		MaxFailuresPerEmail: DefaultMaxFailuresPerEmail,
		MaxFailuresPerIP:    DefaultMaxFailuresPerIP,
		ResetWindow:         DefaultResetWindow,
		MaxResetsPerEmail:   DefaultMaxResetsPerEmail,
		ResetTokenTTL:       DefaultResetTokenTTL,
	}
}

// JobInserter enqueues background jobs inside a transaction (a River client).
type JobInserter interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// Service implements sign-in, sessions and password resets.
type Service struct {
	store  *store.Store
	hasher *Hasher
	jobs   JobInserter
	opts   Options
	log    *slog.Logger
	now    func() time.Time
}

// NewService builds the auth service.
func NewService(st *store.Store, hasher *Hasher, jobs JobInserter, opts Options, log *slog.Logger) *Service {
	return &Service{store: st, hasher: hasher, jobs: jobs, opts: opts, log: log, now: time.Now}
}

// NewSession is a freshly issued session token.
type NewSession struct {
	Token string
	// CookieExpires is the absolute end of the session; the server also ends
	// it earlier after SessionIdle without activity.
	CookieExpires time.Time
}

// Login checks credentials and starts a session.
func (s *Service) Login(ctx context.Context, email, password string) (*Principal, NewSession, error) {
	email = NormalizeEmail(email)
	info := reqinfo.From(ctx)
	now := s.now()

	type limit struct {
		key string
		max int
	}
	limits := []limit{{hashedKey(keyPrefixLoginEmail, email), s.opts.MaxFailuresPerEmail}}
	if info.IP.IsValid() {
		limits = append(limits, limit{keyPrefixLoginIP + info.IP.String(), s.opts.MaxFailuresPerIP})
	}
	keys := make([]string, len(limits))
	for i, l := range limits {
		keys[i] = l.key
		if err := s.checkLimit(ctx, l.key, l.max, s.opts.LoginWindow); err != nil {
			return nil, NewSession{}, err
		}
	}
	emailLimitKey := keys[0]

	user, err := s.store.Q().GetUserByEmail(ctx, email)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, NewSession{}, err
	}
	found := err == nil
	if !found || user.PasswordHash == nil || !IsActive(user.Status) {
		s.hasher.VerifyDummy(ctx, password)
		return nil, NewSession{}, s.loginFailed(ctx, keys, found, user.ID, user.Name)
	}
	ok, rehash, err := s.hasher.Verify(ctx, password, *user.PasswordHash)
	if err != nil && !errors.Is(err, ErrMalformedHash) {
		return nil, NewSession{}, err
	}
	if !ok {
		return nil, NewSession{}, s.loginFailed(ctx, keys, true, user.ID, user.Name)
	}

	var newHash string
	if rehash {
		if newHash, err = s.hasher.Hash(ctx, password); err != nil {
			return nil, NewSession{}, err
		}
	}
	raw, tokenHash, err := NewToken()
	if err != nil {
		return nil, NewSession{}, err
	}
	p := &Principal{
		UserID: user.ID, Email: user.Email, Name: user.Name, Role: Role(user.Role),
		JobTitle: user.JobTitle, ClientID: user.ClientID, ClientName: user.ClientName,
	}
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		id, err := tx.Q.CreateSession(ctx, db.CreateSessionParams{
			TokenHash: tokenHash,
			UserID:    user.ID,
			ExpiresAt: now.Add(min(s.opts.SessionIdle, s.opts.SessionAbsolute)),
			Ip:        ipPtr(info),
			UserAgent: strPtr(info.UserAgent),
		})
		if err != nil {
			return err
		}
		p.SessionID = id
		if newHash != "" {
			if err := tx.Q.UpdateUserPasswordHash(ctx, db.UpdateUserPasswordHashParams{ID: user.ID, PasswordHash: &newHash}); err != nil {
				return err
			}
		}
		if err := tx.Q.TouchUserLogin(ctx, user.ID); err != nil {
			return err
		}
		if err := tx.Q.ClearLoginAttempts(ctx, emailLimitKey); err != nil {
			return err
		}
		return audit.Write(ctx, tx.Q, audit.Entry{
			ActorUserID: &user.ID, ActorName: user.Name, Action: ActionLogin,
			TargetType: TargetUser, TargetID: &user.ID,
		})
	})
	if err != nil {
		return nil, NewSession{}, err
	}
	return p, NewSession{Token: raw, CookieExpires: now.Add(s.opts.SessionAbsolute)}, nil
}

func (s *Service) loginFailed(ctx context.Context, keys []string, userKnown bool, userID uuid.UUID, name string) error {
	err := s.store.Tx(ctx, func(tx store.Tx) error {
		for _, key := range keys {
			if err := tx.Q.RecordLoginAttempt(ctx, key); err != nil {
				return err
			}
		}
		if err := tx.Q.PruneLoginAttempts(ctx, s.now().Add(-loginAttemptRetention)); err != nil {
			return err
		}
		e := audit.Entry{Action: ActionLoginFailed, ActorKind: audit.ActorSystem}
		if userKnown {
			e.TargetType, e.TargetID, e.ActorName = TargetUser, &userID, name
		}
		return audit.Write(ctx, tx.Q, e)
	})
	if err != nil {
		return err
	}
	return apperr.InvalidCredentials()
}

func (s *Service) checkLimit(ctx context.Context, key string, limit int, window time.Duration) error {
	now := s.now()
	stats, err := s.store.Q().LoginAttemptStats(ctx, db.LoginAttemptStatsParams{Key: key, Since: now.Add(-window)})
	if err != nil {
		return err
	}
	if int(stats.Attempts) < limit {
		return nil
	}
	return apperr.RateLimited(max(stats.Oldest.Add(window).Sub(now), minRetryAfter))
}

// Authenticate resolves a session token to the signed-in user.
func (s *Service) Authenticate(ctx context.Context, token string) (*Principal, error) {
	if token == "" {
		return nil, apperr.Unauthenticated()
	}
	row, err := s.store.Q().GetSessionPrincipal(ctx, HashToken(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.Unauthenticated()
	}
	if err != nil {
		return nil, err
	}
	now := s.now()
	if !now.Before(row.ExpiresAt) || !IsActive(row.Status) || row.SessionCreatedAt.Before(row.PasswordChangedAt) {
		if err := s.store.Q().DeleteSession(ctx, row.SessionID); err != nil {
			s.log.WarnContext(ctx, "delete stale session", "err", err)
		}
		return nil, apperr.Unauthenticated()
	}
	if now.Sub(row.LastSeenAt) >= s.opts.TouchInterval {
		expires := now.Add(s.opts.SessionIdle)
		if absolute := row.SessionCreatedAt.Add(s.opts.SessionAbsolute); absolute.Before(expires) {
			expires = absolute
		}
		if err := s.store.Q().TouchSession(ctx, db.TouchSessionParams{ID: row.SessionID, ExpiresAt: expires}); err != nil {
			s.log.WarnContext(ctx, "touch session", "err", err)
		}
	}
	return &Principal{
		SessionID: row.SessionID, UserID: row.UserID, Email: row.Email, Name: row.Name,
		Role: Role(row.Role), JobTitle: row.JobTitle, ClientID: row.ClientID, ClientName: row.ClientName,
	}, nil
}

// Logout ends the given session.
func (s *Service) Logout(ctx context.Context, p *Principal) error {
	return s.store.Tx(ctx, func(tx store.Tx) error {
		if err := tx.Q.DeleteSession(ctx, p.SessionID); err != nil {
			return err
		}
		return audit.Write(ctx, tx.Q, audit.Entry{
			ActorUserID: &p.UserID, ActorName: p.Name, Action: ActionLogout,
			TargetType: TargetUser, TargetID: &p.UserID,
		})
	})
}

// RequestPasswordReset queues a reset email if the address belongs to an active
// user. It reports success either way so callers can't probe for accounts.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	email = NormalizeEmail(email)
	key := hashedKey(keyPrefixReset, email)
	stats, err := s.store.Q().LoginAttemptStats(ctx, db.LoginAttemptStatsParams{Key: key, Since: s.now().Add(-s.opts.ResetWindow)})
	if err != nil {
		return err
	}
	if int(stats.Attempts) >= s.opts.MaxResetsPerEmail {
		return nil // silently drop; the response is the same
	}
	if err := s.store.Q().RecordLoginAttempt(ctx, key); err != nil {
		return err
	}
	user, err := s.store.Q().GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !IsActive(user.Status)) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.store.Tx(ctx, func(tx store.Tx) error {
		if _, err := s.jobs.InsertTx(ctx, tx.PG, SendPasswordResetArgs{UserID: user.ID}, nil); err != nil {
			return fmt.Errorf("enqueue reset email: %w", err)
		}
		return audit.Write(ctx, tx.Q, audit.Entry{
			ActorKind: audit.ActorSystem, Action: ActionPasswordResetRequested,
			TargetType: TargetUser, TargetID: &user.ID, ActorName: user.Name,
		})
	})
}

// ResetPassword sets a new password using a reset token and ends all the user's sessions.
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}
	if token == "" {
		return apperr.InvalidToken()
	}
	hash, err := s.hasher.Hash(ctx, newPassword)
	if err != nil {
		return err
	}
	return s.store.Tx(ctx, func(tx store.Tx) error {
		userID, err := tx.Q.ConsumePasswordResetToken(ctx, HashToken(token))
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.InvalidToken()
		}
		if err != nil {
			return err
		}
		if err := tx.Q.SetUserPassword(ctx, db.SetUserPasswordParams{ID: userID, PasswordHash: &hash}); err != nil {
			return err
		}
		if err := tx.Q.InvalidateUserResetTokens(ctx, userID); err != nil {
			return err
		}
		if err := tx.Q.DeleteUserSessions(ctx, userID); err != nil {
			return err
		}
		return audit.Write(ctx, tx.Q, audit.Entry{
			ActorUserID: &userID, Action: ActionPasswordReset, TargetType: TargetUser, TargetID: &userID,
		})
	})
}

// NewUser describes a user to create.
type NewUser struct {
	Email      string
	Name       string
	Password   string
	Role       Role
	ClientName string // required for requesters
	JobTitle   string
}

// CreateUser adds a user (used by the CLI; invites come later).
func (s *Service) CreateUser(ctx context.Context, in NewUser) (uuid.UUID, error) {
	in.Email = NormalizeEmail(in.Email)
	in.Name = strings.TrimSpace(in.Name)
	switch {
	case !strings.Contains(in.Email, emailAt):
		return uuid.Nil, apperr.Invalid(apperr.CodeInvalidEmail, "Enter a valid email address", "")
	case in.Name == "":
		return uuid.Nil, apperr.Invalid(apperr.CodeInvalidName, "Enter a name", "")
	case !in.Role.Valid():
		return uuid.Nil, apperr.Invalid(apperr.CodeInvalidRole, "Unknown role", string(in.Role))
	case in.Role == RoleRequester && in.ClientName == "":
		return uuid.Nil, apperr.Invalid(apperr.CodeClientRequired, "Requesters must belong to a client", "")
	case in.Role != RoleRequester && in.ClientName != "":
		return uuid.Nil, apperr.Invalid(apperr.CodeClientNotAllowed, "Only requesters belong to a client", "")
	}
	if err := ValidatePassword(in.Password); err != nil {
		return uuid.Nil, err
	}
	hash, err := s.hasher.Hash(ctx, in.Password)
	if err != nil {
		return uuid.Nil, err
	}
	var id uuid.UUID
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		var clientID *uuid.UUID
		if in.ClientName != "" {
			c, err := tx.Q.GetClientByName(ctx, in.ClientName)
			if errors.Is(err, pgx.ErrNoRows) {
				return apperr.Invalid(apperr.CodeClientNotFound, "No client with that name", in.ClientName)
			}
			if err != nil {
				return err
			}
			clientID = &c.ID
		}
		id, err = tx.Q.CreateUser(ctx, db.CreateUserParams{
			Email: in.Email, Name: in.Name, PasswordHash: &hash, Role: string(in.Role),
			ClientID: clientID, JobTitle: strPtr(in.JobTitle),
		})
		if err != nil {
			return err
		}
		return audit.Write(ctx, tx.Q, audit.Entry{
			ActorKind: audit.ActorSystem, ActorName: actorCLI, Action: ActionUserCreated,
			TargetType: TargetUser, TargetID: &id, Data: map[string]any{auditDataRole: in.Role},
		})
	})
	return id, err
}

func ipPtr(info reqinfo.Info) *netip.Addr {
	if !info.IP.IsValid() {
		return nil
	}
	ip := info.IP
	return &ip
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
