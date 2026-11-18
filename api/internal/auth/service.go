// Package auth handles passwords, sessions, password resets and role checks.
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/reqinfo"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
)

// Audit actions written by this package.
const (
	ActionLogin                  = "auth.login"
	ActionLoginFailed            = "auth.login_failed"
	ActionLogout                 = "auth.logout"
	ActionPasswordResetRequested = "auth.password_reset_requested"
	ActionPasswordReset          = "auth.password_reset"
	ActionUserCreated            = "user.created"
	ActionUserInvited            = "user.invited"
	ActionInviteSent             = "user.invite_sent"
	ActionInviteAccepted         = "user.invite_accepted"
	ActionRoleChanged            = "user.role_changed"
	ActionUserSuspended          = "user.suspended"
	ActionUserReactivated        = "user.reactivated"
	ActionMemberAdded            = "user.member_added"
	ActionMemberRemoved          = "user.member_removed"

	TargetUser    = "user"
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

	emailKey := hashedKey(keyPrefixLoginEmail, email)
	limits := []rateLimit{{emailKey, s.opts.MaxFailuresPerEmail, s.opts.LoginWindow}}
	if ipKey := ipLimitKey(keyPrefixLoginIP, info.IP); ipKey != "" {
		limits = append(limits, rateLimit{ipKey, s.opts.MaxFailuresPerIP, s.opts.LoginWindow})
	}
	// The attempt is recorded before the password is checked and dropped again only on success,
	// so parallel guesses are counted one by one instead of all passing an empty count.
	reserved, err := s.reserveAttempts(ctx, limits)
	if err != nil {
		return nil, NewSession{}, err
	}

	user, err := s.store.Q().GetUserByEmail(ctx, email)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, NewSession{}, err
	}
	found := err == nil
	if !found || user.PasswordHash == nil || !IsActive(user.Status) {
		s.hasher.VerifyDummy(ctx, password)
		return nil, NewSession{}, s.loginFailed(ctx, found, user.ID, user.Name)
	}
	ok, rehash, err := s.hasher.Verify(ctx, password, *user.PasswordHash)
	if err != nil && !errors.Is(err, ErrMalformedHash) {
		return nil, NewSession{}, err
	}
	if !ok {
		return nil, NewSession{}, s.loginFailed(ctx, true, user.ID, user.Name)
	}

	var newHash string
	if rehash {
		if newHash, err = s.hasher.Hash(ctx, password); err != nil {
			return nil, NewSession{}, err
		}
	}
	p := &Principal{
		UserID: user.ID, Email: user.Email, Name: user.Name,
		JobTitle: user.JobTitle, ClientID: user.ClientID, ClientName: user.ClientName,
	}
	if err := resolveOrganization(ctx, s.store.Q(), p, account{
		role: Role(user.Role), clientOrg: user.ClientOrganizationID, lastOrg: user.LastOrganizationID,
	}); err != nil {
		return nil, NewSession{}, err
	}
	var sess NewSession
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		if newHash != "" {
			if err := tx.Q.UpdateUserPasswordHash(ctx, db.UpdateUserPasswordHashParams{ID: user.ID, PasswordHash: &newHash}); err != nil {
				return err
			}
		}
		if err := tx.Q.ClearLoginAttempts(ctx, emailKey); err != nil {
			return err
		}
		for _, id := range reserved {
			if err := tx.Q.DeleteLoginAttempt(ctx, id); err != nil {
				return err
			}
		}
		sess, err = s.startSession(ctx, tx, p, info)
		return err
	})
	if err != nil {
		return nil, NewSession{}, err
	}
	return p, sess, nil
}

// startSession signs p in inside tx: a new session row, the last sign-in time and the audit
// entry. Sign-in, an accepted invite and first-run setup all start sessions this way.
func (s *Service) startSession(ctx context.Context, tx store.Tx, p *Principal, info reqinfo.Info) (NewSession, error) {
	raw, tokenHash, err := NewToken()
	if err != nil {
		return NewSession{}, err
	}
	now := s.now()
	id, err := tx.Q.CreateSession(ctx, db.CreateSessionParams{
		TokenHash: tokenHash,
		UserID:    p.UserID,
		ExpiresAt: now.Add(min(s.opts.SessionIdle, s.opts.SessionAbsolute)),
		Ip:        ipPtr(info),
		UserAgent: strPtr(info.UserAgent),
	})
	if err != nil {
		return NewSession{}, err
	}
	p.SessionID = id
	if err := tx.Q.TouchUserLogin(ctx, p.UserID); err != nil {
		return NewSession{}, err
	}
	if err := audit.Write(ctx, tx.Q, audit.Entry{
		ActorUserID: &p.UserID, ActorName: p.Name, Action: ActionLogin, TargetType: TargetUser, TargetID: &p.UserID,
	}); err != nil {
		return NewSession{}, err
	}
	return NewSession{Token: raw, CookieExpires: now.Add(s.opts.SessionAbsolute)}, nil
}

// loginFailed audits a refused sign-in. The attempt itself was recorded by reserveAttempts.
func (s *Service) loginFailed(ctx context.Context, userKnown bool, userID uuid.UUID, name string) error {
	e := audit.Entry{Action: ActionLoginFailed, ActorKind: audit.ActorSystem}
	if userKnown {
		e.TargetType, e.TargetID, e.ActorName = TargetUser, &userID, name
	}
	if err := audit.Write(ctx, s.store.Q(), e); err != nil {
		return err
	}
	return apperr.InvalidCredentials()
}

// rateLimit allows max attempts per key within window.
type rateLimit struct {
	key    string
	max    int
	window time.Duration
}

// reserveAttempts records one attempt against every key, unless a key is already at its limit:
// then nothing is recorded and the answer is rate_limited. Each key is counted and recorded
// under its own lock, so parallel requests can't all pass the count before any is recorded.
// It returns the recorded attempts, for the caller to drop if the attempt succeeds.
func (s *Service) reserveAttempts(ctx context.Context, limits []rateLimit) ([]int64, error) {
	sorted := slices.SortedFunc(slices.Values(limits), func(a, b rateLimit) int { return strings.Compare(a.key, b.key) })
	var ids []int64
	err := s.store.Tx(ctx, func(tx store.Tx) error {
		ids = ids[:0]
		for _, l := range sorted {
			if err := store.LockKey(ctx, tx, store.LockClassRateLimit, l.key); err != nil {
				return err
			}
		}
		for _, l := range sorted {
			if err := s.limitReached(ctx, tx.Q, l); err != nil {
				return err
			}
		}
		for _, l := range sorted {
			id, err := tx.Q.ReserveLoginAttempt(ctx, l.key)
			if err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return nil
	})
	return ids, err
}

func (s *Service) checkLimit(ctx context.Context, key string, limit int, window time.Duration) error {
	return s.limitReached(ctx, s.store.Q(), rateLimit{key, limit, window})
}

// limitReached answers rate_limited when l's key already has l.max attempts in its window.
func (s *Service) limitReached(ctx context.Context, q *db.Queries, l rateLimit) error {
	now := s.now()
	stats, err := q.LoginAttemptStats(ctx, db.LoginAttemptStatsParams{Key: l.key, Since: now.Add(-l.window)})
	if err != nil {
		return err
	}
	if int(stats.Attempts) < l.max {
		return nil
	}
	return apperr.RateLimited(max(stats.Oldest.Add(l.window).Sub(now), minRetryAfter))
}

// refuseToken counts a wrong or spent link against the caller's address (limitKey; "" when the
// address is unknown) and answers invalid_token.
func (s *Service) refuseToken(ctx context.Context, limitKey string) error {
	if limitKey != "" {
		if err := s.store.Q().RecordLoginAttempt(ctx, limitKey); err != nil {
			return err
		}
	}
	return apperr.InvalidToken()
}

// Authenticate resolves a session token to the signed-in user, working in the organization
// requested (nil: their last one).
func (s *Service) Authenticate(ctx context.Context, token string, requested *uuid.UUID) (*Principal, error) {
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
	p := &Principal{
		SessionID: row.SessionID, UserID: row.UserID, Email: row.Email, Name: row.Name,
		JobTitle: row.JobTitle, ClientID: row.ClientID, ClientName: row.ClientName,
	}
	if err := resolveOrganization(ctx, s.store.Q(), p, account{
		role: Role(row.Role), clientOrg: row.ClientOrganizationID, lastOrg: row.LastOrganizationID, requestedOrgs: requested,
	}); err != nil {
		return nil, err
	}
	return p, nil
}

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
	limits := []rateLimit{{hashedKey(keyPrefixReset, email), s.opts.MaxResetsPerEmail, s.opts.ResetWindow}}
	if ipKey := ipLimitKey(keyPrefixResetRequestIP, reqinfo.From(ctx).IP); ipKey != "" {
		limits = append(limits, rateLimit{ipKey, s.opts.MaxFailuresPerIP, s.opts.ResetWindow})
	}
	if _, err := s.reserveAttempts(ctx, limits); apperr.IsCode(err, apperr.CodeRateLimited) {
		return nil // silently drop; the response is the same
	} else if err != nil {
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
	limitKey := ipLimitKey(keyPrefixResetIP, reqinfo.From(ctx).IP)
	if limitKey != "" {
		if err := s.checkLimit(ctx, limitKey, s.opts.MaxFailuresPerIP, s.opts.LoginWindow); err != nil {
			return err
		}
	}
	// The link is checked before the password is hashed, so made-up links cost no hashing.
	tokenHash, active := HashToken(token), string(UserStatusActive)
	_, err := s.store.Q().GetPasswordResetTokenUser(ctx, db.GetPasswordResetTokenUserParams{TokenHash: tokenHash, Active: active})
	if errors.Is(err, pgx.ErrNoRows) {
		return s.refuseToken(ctx, limitKey)
	}
	if err != nil {
		return err
	}
	hash, err := s.hasher.Hash(ctx, newPassword)
	if err != nil {
		return err
	}
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		userID, err := tx.Q.ConsumePasswordResetToken(ctx, db.ConsumePasswordResetTokenParams{TokenHash: tokenHash, Active: active})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.InvalidToken()
		}
		if err != nil {
			return err
		}
		user, err := tx.Q.GetUserByID(ctx, userID)
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
		// Someone locked out by wrong guesses can sign in with the new password at once.
		if err := tx.Q.ClearLoginAttempts(ctx, hashedKey(keyPrefixLoginEmail, user.Email)); err != nil {
			return err
		}
		return audit.Write(ctx, tx.Q, audit.Entry{
			ActorUserID: &userID, Action: ActionPasswordReset, TargetType: TargetUser, TargetID: &userID,
		})
	})
	if apperr.IsCode(err, apperr.CodeInvalidToken) {
		return s.refuseToken(ctx, limitKey)
	}
	return err
}

type NewUser struct {
	Email      string
	Name       string
	Password   string
	Role       Role
	ClientName string // required for requesters
	JobTitle   string
	// OrganizationID is where a supervisor or agent works, and where a requester's client is.
	// Administrators work in every organization.
	OrganizationID uuid.UUID
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
		if in.Role != RoleAdmin && in.OrganizationID == uuid.Nil {
			return apperr.OrganizationRequired()
		}
		var clientID *uuid.UUID
		if in.ClientName != "" {
			c, err := tx.Q.GetClientByName(ctx, db.GetClientByNameParams{OrganizationID: in.OrganizationID, Name: in.ClientName})
			if errors.Is(err, pgx.ErrNoRows) {
				return apperr.Invalid(apperr.CodeClientNotFound, "No client with that name", in.ClientName)
			}
			if err != nil {
				return err
			}
			clientID = &c.ID
		}
		account := in.Role
		if slices.Contains(MemberRoles, in.Role) {
			account = RoleStaff
		}
		id, err = tx.Q.CreateUser(ctx, db.CreateUserParams{
			Email: in.Email, Name: in.Name, PasswordHash: &hash, Role: string(account),
			ClientID: clientID, JobTitle: strPtr(in.JobTitle),
		})
		if err != nil {
			return err
		}
		if account == RoleStaff {
			if err := tx.Q.UpsertMembership(ctx, db.UpsertMembershipParams{UserID: id, OrganizationID: in.OrganizationID, Role: string(in.Role)}); err != nil {
				return err
			}
		}
		if err := audit.Write(ctx, tx.Q, audit.Entry{
			ActorKind: audit.ActorSystem, ActorName: audit.ActorNameCLI, Action: ActionUserCreated,
			TargetType: TargetUser, TargetID: &id, Data: map[string]any{auditDataRole: in.Role},
		}); err != nil {
			return err
		}
		// The first administrator, from the CLI, closes first-run setup as the form would.
		if in.Role == RoleAdmin {
			return tx.Q.CompleteSetup(ctx)
		}
		return nil
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
