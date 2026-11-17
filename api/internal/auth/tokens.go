package auth

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/reqinfo"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
)

// Personal access tokens (owner, 17 Nov) let staff connect an AI client to the MCP server. A
// token acts as its user in the organization it was made in, read only or read and write, until
// it expires or is revoked. Only its hash is stored; the raw token is shown once.

// TokenAccess is what a personal token may do. Values match personal_access_tokens.access.
type TokenAccess string

const (
	TokenRead  TokenAccess = "read"
	TokenWrite TokenAccess = "write"
)

func (a TokenAccess) Valid() bool { return a == TokenRead || a == TokenWrite }

// Token rules.
const (
	// TokenPrefix starts every personal token, so a leaked one is easy to spot and scan for.
	TokenPrefix = "simdesk_pat_"
	// MaxTokensPerUser caps a user's unexpired tokens in one organization.
	MaxTokensPerUser = 20
	// MaxTokenNameLen matches the personal_access_tokens_name CHECK.
	MaxTokenNameLen = 100
	// DefaultTokenLifetimeDays is the expiry the form preselects.
	DefaultTokenLifetimeDays = 90
	// tokenShownChars is how much of the token's end the list shows (personal_access_tokens_last_four).
	tokenShownChars = 4
	hoursPerDay     = 24
)

// TokenLifetimeDays are the expiries a token may have, in days. There is no "never".
var TokenLifetimeDays = []int{30, DefaultTokenLifetimeDays, 365}

// Audit actions and data for tokens. Entries never hold the token's name or secret.
const (
	ActionTokenCreated = "token.created"
	ActionTokenRevoked = "token.revoked"
	TargetToken        = "personal_access_token"
	auditDataAccess    = "access"
	auditDataExpiresAt = "expires_at"
)

// PersonalToken is a token as its owner sees it in the list.
type PersonalToken struct {
	ID         uuid.UUID
	Name       string
	LastFour   string
	Access     TokenAccess
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastUsedAt *time.Time
}

// TokenInput is a new token.
type TokenInput struct {
	Name         string
	Access       TokenAccess
	LifetimeDays int
}

// TokenSession is who a token signs in, and what it may do.
type TokenSession struct {
	Principal *Principal
	Access    TokenAccess
	ExpiresAt time.Time
}

// CanWrite reports whether the token may change tickets.
func (t TokenSession) CanWrite() bool { return t.Access == TokenWrite }

func errTokenLimit() error {
	return apperr.Conflict(apperr.CodeTokenLimit, "You have the most tokens allowed",
		"Revoke a token you no longer use, then make a new one.")
}

// tokenOwner is p if staff working in an organization may hold tokens there.
func tokenOwner(p *Principal) error {
	if p.OrganizationID == uuid.Nil {
		return apperr.OrganizationRequired()
	}
	if !Can(p.Role, CapAgentApp) {
		return apperr.Forbidden()
	}
	return nil
}

// CreateToken makes a token for p in the organization p works in, and returns it with the raw
// secret, which is never stored or shown again.
func (s *Service) CreateToken(ctx context.Context, p *Principal, in TokenInput) (PersonalToken, string, error) {
	if err := tokenOwner(p); err != nil {
		return PersonalToken{}, "", err
	}
	in.Name = strings.TrimSpace(in.Name)
	switch {
	case in.Name == "" || utf8.RuneCountInString(in.Name) > MaxTokenNameLen:
		return PersonalToken{}, "", apperr.InvalidRequest("name")
	case !in.Access.Valid():
		return PersonalToken{}, "", apperr.InvalidRequest("access")
	case !slices.Contains(TokenLifetimeDays, in.LifetimeDays):
		return PersonalToken{}, "", apperr.InvalidRequest("expiresInDays")
	}
	raw, _, err := NewToken()
	if err != nil {
		return PersonalToken{}, "", err
	}
	secret := TokenPrefix + raw
	now := s.now()
	var out PersonalToken
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		live, err := tx.Q.CountLivePersonalTokens(ctx, db.CountLivePersonalTokensParams{
			UserID: p.UserID, OrganizationID: p.OrganizationID, Now: now,
		})
		if err != nil {
			return err
		}
		if live >= MaxTokensPerUser {
			return errTokenLimit()
		}
		row, err := tx.Q.CreatePersonalToken(ctx, db.CreatePersonalTokenParams{
			UserID: p.UserID, OrganizationID: p.OrganizationID, Name: in.Name, TokenHash: HashToken(secret),
			LastFour: secret[len(secret)-tokenShownChars:], Access: string(in.Access),
			ExpiresAt: now.Add(time.Duration(in.LifetimeDays) * hoursPerDay * time.Hour),
		})
		if err != nil {
			return err
		}
		out = personalToken(db.ListPersonalTokensRow(row))
		return audit.Write(ctx, tx.Q, p.AuditActor().Entry(ActionTokenCreated, TargetToken, &row.ID, map[string]any{
			auditDataAccess: row.Access, auditDataExpiresAt: row.ExpiresAt,
		}))
	})
	if err != nil {
		return PersonalToken{}, "", err
	}
	return out, secret, nil
}

// ListTokens lists p's tokens in the organization p works in, newest first, expired ones too.
func (s *Service) ListTokens(ctx context.Context, p *Principal) ([]PersonalToken, error) {
	if err := tokenOwner(p); err != nil {
		return nil, err
	}
	rows, err := s.store.Q().ListPersonalTokens(ctx, db.ListPersonalTokensParams{UserID: p.UserID, OrganizationID: p.OrganizationID})
	if err != nil {
		return nil, err
	}
	out := make([]PersonalToken, len(rows))
	for i, r := range rows {
		out[i] = personalToken(r)
	}
	return out, nil
}

func personalToken(r db.ListPersonalTokensRow) PersonalToken {
	return PersonalToken{
		ID: r.ID, Name: r.Name, LastFour: r.LastFour, Access: TokenAccess(r.Access),
		CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, LastUsedAt: r.LastUsedAt,
	}
}

// RevokeToken deletes one of p's tokens in the organization p works in; anyone else's is a 404.
func (s *Service) RevokeToken(ctx context.Context, p *Principal, id uuid.UUID) error {
	if err := tokenOwner(p); err != nil {
		return err
	}
	return s.store.Tx(ctx, func(tx store.Tx) error {
		row, err := tx.Q.DeletePersonalToken(ctx, db.DeletePersonalTokenParams{ID: id, UserID: p.UserID, OrganizationID: p.OrganizationID})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		return audit.Write(ctx, tx.Q, p.AuditActor().Entry(ActionTokenRevoked, TargetToken, &row.ID, map[string]any{
			auditDataAccess: row.Access, auditDataExpiresAt: row.ExpiresAt,
		}))
	})
}

// AuthenticateToken signs a request in with a raw personal token. The token must be live, its
// user active staff, and the user still able to work in the token's organization (a removed
// membership ends it). Unknown tokens count against the caller's address.
func (s *Service) AuthenticateToken(ctx context.Context, raw string) (TokenSession, error) {
	limitKey := ""
	if ip := reqinfo.From(ctx).IP; ip.IsValid() {
		limitKey = keyPrefixTokenIP + ip.String()
		if err := s.checkLimit(ctx, limitKey, s.opts.MaxFailuresPerIP, s.opts.LoginWindow); err != nil {
			return TokenSession{}, err
		}
	}
	q := s.store.Q()
	var row db.GetTokenPrincipalRow
	err := errUnknownToken
	if strings.HasPrefix(raw, TokenPrefix) {
		row, err = q.GetTokenPrincipal(ctx, HashToken(raw))
	}
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, errUnknownToken) {
		if limitKey != "" {
			if rerr := q.RecordLoginAttempt(ctx, limitKey); rerr != nil {
				return TokenSession{}, rerr
			}
		}
		return TokenSession{}, apperr.Unauthenticated()
	}
	if err != nil {
		return TokenSession{}, err
	}
	now := s.now()
	if !now.Before(row.ExpiresAt) || !IsActive(row.Status) || Role(row.Role) == RoleRequester {
		return TokenSession{}, apperr.Unauthenticated()
	}
	p := &Principal{
		UserID: row.UserID, Email: row.Email, Name: row.Name,
		JobTitle: row.JobTitle, ClientID: row.ClientID, ClientName: row.ClientName,
	}
	if err := resolveOrganization(ctx, q, p, account{
		role: Role(row.Role), clientOrg: row.ClientOrganizationID, requestedOrgs: &row.OrganizationID,
	}); err != nil {
		return TokenSession{}, err
	}
	if p.OrganizationID != row.OrganizationID || !Can(p.Role, CapAgentApp) {
		return TokenSession{}, apperr.Unauthenticated()
	}
	if row.LastUsedAt == nil || now.Sub(*row.LastUsedAt) >= s.opts.TouchInterval {
		if err := q.TouchPersonalToken(ctx, row.TokenID); err != nil {
			s.log.WarnContext(ctx, "touch personal token", "err", err)
		}
	}
	return TokenSession{Principal: p, Access: TokenAccess(row.Access), ExpiresAt: row.ExpiresAt}, nil
}

// errUnknownToken is a token without the SimDesk prefix: never looked up.
var errUnknownToken = errors.New("auth: not a personal token")
