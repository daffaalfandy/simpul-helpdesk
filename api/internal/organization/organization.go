// Package organization keeps the install's vendor organizations: each one's name, ticket prefix,
// time zone and logo (Settings › General), and whether first-run setup has been completed
// (install-wide).
package organization

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/storage"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// Audit actions; entries target the organization.
const (
	ActionCreated      = "organization.created"
	ActionUpdated      = "organization.updated"
	ActionLogoUpdated  = "organization.logo_updated"
	ActionLogoRemoved  = "organization.logo_removed"
	TargetOrganization = "organization"
)

const (
	dataName     = "name"
	dataPrefix   = "ticket_prefix"
	dataTimezone = "timezone"
	dataType     = "content_type"
	dataSize     = "size"
)

// Field limits, matching the database CHECK constraints.
const MaxNameLen = 200

// MaxLogoBytes caps an uploaded logo: plenty for a header image, small enough for every email.
const MaxLogoBytes = 512 << 10

// sniffBytes is how much of a file http.DetectContentType reads.
const sniffBytes = 512

// Logo types. Values match the organization.logo_content_type CHECK. No SVG: email clients
// don't show it, and it can carry scripts (owner, 17 Nov).
const (
	LogoPNG  = "image/png"
	LogoJPEG = "image/jpeg"
	LogoWebP = "image/webp"
)

var LogoTypes = []string{LogoPNG, LogoJPEG, LogoWebP}

// LogoPath is where an organization's logo is served (its id fills the %s), versioned by when
// it changed so caches never show an old one. It is relative to APIBasePath. The older
// "/organization/logo" keeps serving the first organization's, for emails sent before.
const LogoPath = "/organizations/%s/logo"

// APIBasePath is the API's URL prefix, as the server mounts it (httpapi).
const APIBasePath = "/api/v1"

// logoVersionParam is the query parameter that carries the version.
const logoVersionParam = "?v="

// localZone is what time.LoadLocation reads as the server's own zone; never an organisation's.
const localZone = "Local"

// ProductName brands what belongs to no organization: the sign-in page, setup, and an install
// that has no organization yet.
const ProductName = "SimDesk"

// Defaults for a new organization: Indonesia's western time zone, where most clients are, and
// the mock's HD-24871 keys.
const (
	DefaultTimezone     = "Asia/Jakarta"
	DefaultTicketPrefix = "HD"
)

// constraintTicketPrefix keeps ticket prefixes, and so ticket keys, unique across the install.
const constraintTicketPrefix = "organizations_ticket_prefix_key"

// Settings is an organization as Settings › General shows it.
type Settings struct {
	ID           uuid.UUID
	Name         string
	TicketPrefix string
	Timezone     string
	FooterNote   *string
	Logo         *Logo
}

// Logo is the stored logo.
type Logo struct {
	Key         string
	ContentType string
	UpdatedAt   time.Time
}

// Version is the logo's cache-busting version, for its URL.
func (l Logo) Version() string { return strconv.FormatInt(l.UpdatedAt.UnixMilli(), 10) }

// Path is the logo's URL path under the API base, with its version.
func (l Logo) Path(orgID uuid.UUID) string {
	return fmt.Sprintf(LogoPath, orgID) + logoVersionParam + l.Version()
}

// LogoOf is an organization's stored logo; nil without one.
func LogoOf(key, contentType *string, updatedAt *time.Time) *Logo {
	if key == nil || contentType == nil || updatedAt == nil {
		return nil
	}
	return &Logo{Key: *key, ContentType: *contentType, UpdatedAt: *updatedAt}
}

// LogoURL is the logo's absolute address under appURL, for emails; "" without a logo.
func LogoURL(appURL *url.URL, row db.GetOrganizationRow) string {
	logo := settingsOf(row).Logo
	if logo == nil || appURL == nil {
		return ""
	}
	return strings.TrimSuffix(appURL.String(), "/") + APIBasePath + logo.Path(row.ID)
}

// Input is what an admin changes in Settings › General.
type Input struct {
	Name         string
	TicketPrefix string
	Timezone     string
}

type Service struct {
	store *store.Store
	files storage.Store
	sla   *sla.Service
	now   func() time.Time
}

func NewService(st *store.Store, files storage.Store, slaSvc *sla.Service) *Service {
	return &Service{store: st, files: files, sla: slaSvc, now: time.Now}
}

// Get returns an organization.
func (s *Service) Get(ctx context.Context, orgID uuid.UUID) (Settings, error) {
	row, err := s.store.Q().GetOrganization(ctx, orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Settings{}, apperr.NotFound()
	}
	if err != nil {
		return Settings{}, err
	}
	return settingsOf(row), nil
}

// First returns the organization made first; ok is false on an install without one.
func (s *Service) First(ctx context.Context) (Settings, bool, error) {
	row, err := s.store.Q().GetFirstOrganization(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return Settings{}, false, nil
	}
	if err != nil {
		return Settings{}, false, err
	}
	return settingsOf(db.GetOrganizationRow(row)), true, nil
}

// SetupRequired reports whether first-run setup is still open: until an administrator exists.
func (s *Service) SetupRequired(ctx context.Context) (bool, error) {
	done, err := s.store.Q().GetInstall(ctx)
	return done == nil, err
}

func settingsOf(row db.GetOrganizationRow) Settings {
	out := Settings{
		ID: row.ID, Name: row.Name, TicketPrefix: row.TicketPrefix, Timezone: row.Timezone, FooterNote: row.FooterNote,
	}
	out.Logo = LogoOf(row.LogoKey, row.LogoContentType, row.LogoUpdatedAt)
	return out
}

// Update changes the name, ticket prefix and time zone. A new prefix applies to new tickets;
// existing keys never change. A new time zone recomputes open tickets' SLA clocks, as calendar
// edits do.
func (s *Service) Update(ctx context.Context, actor audit.Actor, orgID uuid.UUID, in Input) (Settings, error) {
	in = normalize(in)
	if err := validate(in); err != nil {
		return Settings{}, err
	}
	var out Settings
	err := s.store.Tx(ctx, func(tx store.Tx) error {
		if err := tx.Q.LockOrganization(ctx, orgID); err != nil {
			return err
		}
		old, err := tx.Q.GetOrganization(ctx, orgID)
		if err != nil {
			return notFound(err)
		}
		changes := audit.Changes{}
		audit.Track(changes, dataName, old.Name, in.Name)
		audit.Track(changes, dataPrefix, old.TicketPrefix, in.TicketPrefix)
		audit.Track(changes, dataTimezone, old.Timezone, in.Timezone)
		if len(changes) > 0 {
			if err := tx.Q.UpdateOrganizationSettings(ctx, db.UpdateOrganizationSettingsParams{
				ID: orgID, Name: in.Name, TicketPrefix: in.TicketPrefix, Timezone: in.Timezone,
			}); err != nil {
				return err
			}
			if err := audit.Write(ctx, tx.Q, actor.Entry(ActionUpdated, TargetOrganization, &orgID, changes)); err != nil {
				return err
			}
			if old.Timezone != in.Timezone {
				if err := s.sla.Recompute(ctx, tx, orgID); err != nil {
					return err
				}
			}
		}
		row, err := tx.Q.GetOrganization(ctx, orgID)
		out = settingsOf(row)
		return err
	})
	return out, prefixError(err)
}

// Create adds an organization with the default priorities and business hours, under the install
// lock so two at once can't race for a prefix.
func (s *Service) Create(ctx context.Context, actor audit.Actor, in Input) (Settings, error) {
	in = normalize(in)
	if err := validate(in); err != nil {
		return Settings{}, err
	}
	var id uuid.UUID
	err := s.store.Tx(ctx, func(tx store.Tx) error {
		var err error
		if id, err = CreateTx(ctx, tx, in); err != nil {
			return err
		}
		return audit.Write(ctx, tx.Q, actor.In(id).Entry(ActionCreated, TargetOrganization, &id, map[string]any{
			dataName: in.Name, dataPrefix: in.TicketPrefix, dataTimezone: in.Timezone,
		}))
	})
	if err != nil {
		return Settings{}, prefixError(err)
	}
	return s.Get(ctx, id)
}

// CreateTx inserts a validated organization with its defaults inside tx (first-run setup uses it
// too). The caller audits.
func CreateTx(ctx context.Context, tx store.Tx, in Input) (uuid.UUID, error) {
	if err := tx.Q.LockInstall(ctx); err != nil {
		return uuid.Nil, err
	}
	id, err := tx.Q.CreateOrganization(ctx, db.CreateOrganizationParams{Name: in.Name, TicketPrefix: in.TicketPrefix, Timezone: in.Timezone})
	if err != nil {
		return uuid.Nil, err
	}
	if err := tx.Q.InsertDefaultPriorities(ctx, id); err != nil {
		return uuid.Nil, err
	}
	return id, tx.Q.InsertDefaultBusinessHours(ctx, id)
}

// normalize trims the input and upper-cases the prefix.
func normalize(in Input) Input {
	in.Name = strings.TrimSpace(in.Name)
	in.TicketPrefix = ticket.NormalizeKey(in.TicketPrefix)
	in.Timezone = strings.TrimSpace(in.Timezone)
	return in
}

// Validate checks an organization's name, prefix and time zone after trimming them.
func Validate(in Input) (Input, error) {
	in = normalize(in)
	return in, validate(in)
}

// prefixError maps another organization's prefix to a 409.
func prefixError(err error) error {
	if store.IsViolation(err, store.SQLStateUniqueViolation, constraintTicketPrefix) {
		return apperr.Conflict(apperr.CodeTicketPrefixTaken, "Another organization uses this ticket prefix", "")
	}
	return err
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound()
	}
	return err
}

func validate(in Input) error {
	var fields []apperr.FieldError
	if in.Name == "" || utf8.RuneCountInString(in.Name) > MaxNameLen {
		fields = append(fields, apperr.FieldError{Field: dataName, Message: fmt.Sprintf("1 to %d characters", MaxNameLen)})
	}
	if !ticket.ValidKey(ticket.FormatKey(in.TicketPrefix, 1)) {
		fields = append(fields, apperr.FieldError{Field: dataPrefix, Message: "a letter, then up to 9 letters or digits"})
	}
	if _, err := sla.ParseLocation(in.Timezone); err != nil || in.Timezone == "" || in.Timezone == localZone {
		fields = append(fields, apperr.FieldError{Field: dataTimezone, Message: "a time zone such as Asia/Jakarta"})
	}
	if len(fields) == 0 {
		return nil
	}
	e := apperr.Invalid(apperr.CodeInvalidOrganization, "Check the organisation settings", "")
	e.Fields = fields
	return e
}

// SetLogo stores a new logo (PNG, JPEG or WebP, at most MaxLogoBytes) and removes the old file
// once the change is saved.
func (s *Service) SetLogo(ctx context.Context, actor audit.Actor, orgID uuid.UUID, r io.Reader) (Settings, error) {
	br := bufio.NewReaderSize(r, sniffBytes)
	head, err := br.Peek(sniffBytes)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return Settings{}, err
	}
	if len(head) == 0 {
		return Settings{}, apperr.Invalid(apperr.CodeLogoTypeNotAllowed, "Choose a PNG, JPEG or WebP image", "")
	}
	contentType := http.DetectContentType(head)
	if !slices.Contains(LogoTypes, contentType) {
		return Settings{}, apperr.Invalid(apperr.CodeLogoTypeNotAllowed, "Choose a PNG, JPEG or WebP image", "")
	}
	key := storage.NewKey()
	size, err := s.files.Put(ctx, key, io.LimitReader(br, MaxLogoBytes+1))
	if err != nil {
		return Settings{}, err
	}
	if size > MaxLogoBytes {
		_ = s.files.Delete(ctx, key)
		return Settings{}, apperr.TooLarge(apperr.CodeLogoTooLarge, "The logo is too large", fmt.Sprintf("At most %d KB.", MaxLogoBytes>>10))
	}
	out, old, err := s.swapLogo(ctx, actor, orgID, &key, &contentType, size)
	if err != nil {
		_ = s.files.Delete(ctx, key)
		return Settings{}, err
	}
	s.deleteFile(ctx, old)
	return out, nil
}

// RemoveLogo goes back to the name's initials.
func (s *Service) RemoveLogo(ctx context.Context, actor audit.Actor, orgID uuid.UUID) (Settings, error) {
	out, old, err := s.swapLogo(ctx, actor, orgID, nil, nil, 0)
	if err != nil {
		return Settings{}, err
	}
	s.deleteFile(ctx, old)
	return out, nil
}

// swapLogo saves the new logo (nil removes it) and returns the old file's key.
func (s *Service) swapLogo(ctx context.Context, actor audit.Actor, orgID uuid.UUID, key, contentType *string, size int64) (Settings, *string, error) {
	var out Settings
	var old *string
	err := s.store.Tx(ctx, func(tx store.Tx) error {
		if err := tx.Q.LockOrganization(ctx, orgID); err != nil {
			return err
		}
		prev, err := tx.Q.GetOrganization(ctx, orgID)
		if err != nil {
			return notFound(err)
		}
		old = prev.LogoKey
		if key == nil && old == nil {
			out = settingsOf(prev)
			return nil
		}
		var at *time.Time
		entry := actor.Entry(ActionLogoRemoved, TargetOrganization, &orgID, nil)
		if key != nil {
			now := s.now()
			at = &now
			entry = actor.Entry(ActionLogoUpdated, TargetOrganization, &orgID, map[string]any{dataType: *contentType, dataSize: size})
		}
		if err := tx.Q.SetOrganizationLogo(ctx, db.SetOrganizationLogoParams{ID: orgID, LogoKey: key, LogoContentType: contentType, LogoUpdatedAt: at}); err != nil {
			return err
		}
		if err := audit.Write(ctx, tx.Q, entry); err != nil {
			return err
		}
		row, err := tx.Q.GetOrganization(ctx, orgID)
		out = settingsOf(row)
		return err
	})
	return out, old, err
}

// deleteFile removes a replaced logo; a failure only leaves an unreferenced file behind.
func (s *Service) deleteFile(ctx context.Context, key *string) {
	if key != nil {
		_ = s.files.Delete(ctx, *key)
	}
}

// LogoFile is the logo as served.
type LogoFile struct {
	Body        io.ReadSeekCloser
	Size        int64
	ContentType string
}

// OpenLogo opens an organization's logo for serving; 404 without one. Callers close Body.
func (s *Service) OpenLogo(ctx context.Context, orgID uuid.UUID) (LogoFile, error) {
	settings, err := s.Get(ctx, orgID)
	if err != nil {
		return LogoFile{}, err
	}
	if settings.Logo == nil {
		return LogoFile{}, apperr.NotFound()
	}
	body, size, err := s.files.Open(ctx, settings.Logo.Key)
	if errors.Is(err, storage.ErrNotFound) {
		return LogoFile{}, apperr.NotFound()
	}
	if err != nil {
		return LogoFile{}, err
	}
	return LogoFile{Body: body, Size: size, ContentType: settings.Logo.ContentType}, nil
}
