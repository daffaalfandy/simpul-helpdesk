package auth

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/organization"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/reqinfo"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
)

const (
	// ActionSetupCompleted is first-run setup: the organisation named and its first admin made.
	ActionSetupCompleted = "organization.setup_completed"

	// actorNameSetup names the system actor of the first-run form, as "cli" names the CLI's.
	actorNameSetup = "setup"

	auditDataName = "name"
)

// SetupInput is the first-run form: the organisation's name and its first administrator.
type SetupInput struct {
	OrganizationName string
	Name             string
	Email            string
	Password         string
}

// Setup runs first-run setup: it names the organisation, creates the first administrator and
// signs them in. It works once; after that, and after `simdesk user create --role admin`, it
// answers setup_closed. Refused attempts count against the caller's address like failed sign-ins.
func (s *Service) Setup(ctx context.Context, in SetupInput) (*Principal, NewSession, error) {
	in.OrganizationName = strings.TrimSpace(in.OrganizationName)
	in.Name = strings.TrimSpace(in.Name)
	in.Email = NormalizeEmail(in.Email)
	if in.OrganizationName == "" || utf8.RuneCountInString(in.OrganizationName) > organization.MaxNameLen {
		return nil, NewSession{}, apperr.Invalid(apperr.CodeInvalidOrganization, "Enter the organisation's name", "")
	}
	if err := ValidateNewUser(in.Email, in.Name, ""); err != nil {
		return nil, NewSession{}, err
	}
	if err := ValidatePassword(in.Password); err != nil {
		return nil, NewSession{}, err
	}
	info := reqinfo.From(ctx)
	limitKey := ""
	if info.IP.IsValid() {
		limitKey = keyPrefixSetupIP + info.IP.String()
		if err := s.checkLimit(ctx, limitKey, s.opts.MaxFailuresPerIP, s.opts.LoginWindow); err != nil {
			return nil, NewSession{}, err
		}
	}
	hash, err := s.hasher.Hash(ctx, in.Password)
	if err != nil {
		return nil, NewSession{}, err
	}
	var p *Principal
	var sess NewSession
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		if err := tx.Q.LockInstall(ctx); err != nil {
			return err
		}
		done, err := tx.Q.GetInstall(ctx)
		if err != nil {
			return err
		}
		if done != nil {
			return apperr.Conflict(apperr.CodeSetupClosed, "This install is already set up", "Sign in instead.")
		}
		id, err := tx.Q.CreateUser(ctx, db.CreateUserParams{Email: in.Email, Name: in.Name, PasswordHash: &hash, Role: string(RoleAdmin)})
		if store.IsViolation(err, store.SQLStateUniqueViolation, ConstraintUserEmail) {
			return apperr.Conflict(apperr.CodeEmailTaken, "Someone already uses this email", "")
		}
		if err != nil {
			return err
		}
		orgID, err := setupOrganization(ctx, tx, in.OrganizationName)
		if err != nil {
			return err
		}
		if err := tx.Q.CompleteSetup(ctx); err != nil {
			return err
		}
		system := audit.Entry{ActorKind: audit.ActorSystem, ActorName: actorNameSetup}
		created, completed := system, system
		created.Action, created.TargetType, created.TargetID = ActionUserCreated, TargetUser, &id
		created.Data = map[string]any{auditDataRole: RoleAdmin}
		completed.OrganizationID = &orgID
		completed.Action, completed.TargetType, completed.TargetID = ActionSetupCompleted, organization.TargetOrganization, &orgID
		completed.Data = map[string]any{auditDataName: in.OrganizationName}
		for _, e := range []audit.Entry{created, completed} {
			if err := audit.Write(ctx, tx.Q, e); err != nil {
				return err
			}
		}
		user, err := tx.Q.GetUserByID(ctx, id)
		if err != nil {
			return err
		}
		if p, err = principalOf(ctx, tx.Q, user); err != nil {
			return err
		}
		sess, err = s.startSession(ctx, tx, p, info)
		return err
	})
	if apperr.IsCode(err, apperr.CodeSetupClosed) && limitKey != "" {
		if rerr := s.store.Q().RecordLoginAttempt(ctx, limitKey); rerr != nil {
			return nil, NewSession{}, rerr
		}
	}
	if err != nil {
		return nil, NewSession{}, err
	}
	return p, sess, nil
}

// setupOrganization names the install's organization: the one an upgraded install already has,
// or a new one with the defaults.
func setupOrganization(ctx context.Context, tx store.Tx, name string) (uuid.UUID, error) {
	first, err := tx.Q.GetFirstOrganization(ctx)
	if err == nil {
		return first.ID, tx.Q.UpdateOrganizationSettings(ctx, db.UpdateOrganizationSettingsParams{
			ID: first.ID, Name: name, TicketPrefix: first.TicketPrefix, Timezone: first.Timezone,
		})
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	return organization.CreateTx(ctx, tx, organization.Input{
		Name: name, TicketPrefix: organization.DefaultTicketPrefix, Timezone: organization.DefaultTimezone,
	})
}
