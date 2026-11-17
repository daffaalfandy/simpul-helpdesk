package httpapi

import (
	"context"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/organization"
)

// logoCache keeps a logo for good: a new logo gets a new versioned URL.
const logoCache = "public, max-age=31536000, immutable"

// GetOrganization returns public branding, and whether first-run setup is still open: the
// signed-in user's organization, else the install's first (SimDesk's defaults before there is one).
func (s *server) GetOrganization(ctx context.Context, _ gen.GetOrganizationRequestObject) (gen.GetOrganizationResponseObject, error) {
	o, err := s.brandingOrganization(ctx)
	if err != nil {
		return nil, err
	}
	return gen.GetOrganization200JSONResponse(s.apiOrganization(ctx, o)), nil
}

func (s *server) brandingOrganization(ctx context.Context) (organization.Settings, error) {
	if p, ok := auth.PrincipalFrom(ctx); ok && p.OrganizationID != uuid.Nil {
		return s.organization.Get(ctx, p.OrganizationID)
	}
	o, ok, err := s.organization.First(ctx)
	if err != nil || ok {
		return o, err
	}
	return organization.Settings{
		Name: organization.ProductName, TicketPrefix: organization.DefaultTicketPrefix, Timezone: organization.DefaultTimezone,
	}, nil
}

func (s *server) UpdateOrganization(ctx context.Context, req gen.UpdateOrganizationRequestObject) (gen.UpdateOrganizationResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	o, err := s.organization.Update(ctx, p.AuditActor(), p.OrganizationID, organization.Input{
		Name: req.Body.Name, TicketPrefix: req.Body.TicketPrefix, Timezone: req.Body.Timezone,
	})
	if err != nil {
		return nil, err
	}
	return gen.UpdateOrganization200JSONResponse(s.apiOrganization(ctx, o)), nil
}

// UploadOrganizationLogo reads the first multipart part, like attachments (the route skips the
// spec validator).
func (s *server) UploadOrganizationLogo(ctx context.Context, req gen.UploadOrganizationLogoRequestObject) (gen.UploadOrganizationLogoResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	part, err := req.Body.NextPart()
	if err != nil {
		if tooLargeError(err) {
			return nil, errUploadTooLarge()
		}
		return nil, apperr.InvalidRequest(uploadNeedsFile)
	}
	defer part.Close()
	if part.FormName() != uploadFieldFile {
		return nil, apperr.InvalidRequest(uploadNeedsFile)
	}
	o, err := s.organization.SetLogo(ctx, p.AuditActor(), p.OrganizationID, part)
	if err != nil {
		return nil, err
	}
	return gen.UploadOrganizationLogo200JSONResponse(s.apiOrganization(ctx, o)), nil
}

func (s *server) RemoveOrganizationLogo(ctx context.Context, _ gen.RemoveOrganizationLogoRequestObject) (gen.RemoveOrganizationLogoResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	o, err := s.organization.RemoveLogo(ctx, p.AuditActor(), p.OrganizationID)
	if err != nil {
		return nil, err
	}
	return gen.RemoveOrganizationLogo200JSONResponse(s.apiOrganization(ctx, o)), nil
}

// GetOrganizationLogo serves the logo inline with its own type, so the sign-in page, the portal
// and emails can show it.
func (s *server) GetOrganizationLogo(ctx context.Context, _ gen.GetOrganizationLogoRequestObject) (gen.GetOrganizationLogoResponseObject, error) {
	o, err := s.brandingOrganization(ctx)
	if err != nil {
		return nil, err
	}
	f, err := s.organization.OpenLogo(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	cache := logoCache
	headers := gen.GetOrganizationLogo200ResponseHeaders{CacheControl: &cache}
	switch f.ContentType {
	case organization.LogoJPEG:
		return gen.GetOrganizationLogo200ImagejpegResponse{Body: f.Body, ContentLength: f.Size, Headers: headers}, nil
	case organization.LogoWebP:
		return gen.GetOrganizationLogo200ImagewebpResponse{Body: f.Body, ContentLength: f.Size, Headers: headers}, nil
	default:
		return gen.GetOrganizationLogo200ImagepngResponse{Body: f.Body, ContentLength: f.Size, Headers: headers}, nil
	}
}

func (s *server) apiOrganization(ctx context.Context, o organization.Settings) gen.Organization {
	// A failed read leaves setupRequired false: the sign-in page still works.
	setupRequired, _ := s.organization.SetupRequired(ctx)
	out := gen.Organization{
		Name: o.Name, Timezone: o.Timezone, TicketPrefix: o.TicketPrefix, FooterNote: o.FooterNote,
		AppUrl: s.cfg.AppURL.String(), SetupRequired: setupRequired,
	}
	if o.Logo != nil {
		url := apiPrefix + o.Logo.Path()
		out.LogoUrl = &url
	}
	return out
}
