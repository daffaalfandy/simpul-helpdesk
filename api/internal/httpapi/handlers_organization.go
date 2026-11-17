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

// GetInstall tells the sign-in and setup pages whether first-run setup is still open.
func (s *server) GetInstall(ctx context.Context, _ gen.GetInstallRequestObject) (gen.GetInstallResponseObject, error) {
	open, err := s.organization.SetupRequired(ctx)
	if err != nil {
		return nil, err
	}
	return gen.GetInstall200JSONResponse{SetupRequired: open, AppUrl: s.cfg.AppURL.String()}, nil
}

// GetOrganization returns the organization the request works in.
func (s *server) GetOrganization(ctx context.Context, _ gen.GetOrganizationRequestObject) (gen.GetOrganizationResponseObject, error) {
	p, err := signedInOrg(ctx)
	if err != nil {
		return nil, err
	}
	o, err := s.organization.Get(ctx, p.OrganizationID)
	if err != nil {
		return nil, err
	}
	return gen.GetOrganization200JSONResponse(s.apiOrganization(o)), nil
}

// ListOrganizations lists every organization (admin only).
func (s *server) ListOrganizations(ctx context.Context, _ gen.ListOrganizationsRequestObject) (gen.ListOrganizationsResponseObject, error) {
	if _, err := auth.Require(ctx, auth.CapOrganizationsManage); err != nil {
		return nil, err
	}
	list, err := s.organization.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListOrganizations200JSONResponse, len(list))
	for i, o := range list {
		out[i] = apiOrganizationSummary(o)
	}
	return out, nil
}

// CreateOrganization adds a vendor organization (admin only).
func (s *server) CreateOrganization(ctx context.Context, req gen.CreateOrganizationRequestObject) (gen.CreateOrganizationResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapOrganizationsManage)
	if err != nil {
		return nil, err
	}
	o, err := s.organization.Create(ctx, p.AuditActor(), organization.Input{
		Name: req.Body.Name, TicketPrefix: req.Body.TicketPrefix, Timezone: req.Body.Timezone,
	})
	if err != nil {
		return nil, err
	}
	return gen.CreateOrganization201JSONResponse(apiOrganizationSummary(organization.Summary{Settings: o})), nil
}

func apiOrganizationSummary(o organization.Summary) gen.OrganizationSummary {
	out := gen.OrganizationSummary{
		Id: o.ID, Name: o.Name, TicketPrefix: o.TicketPrefix, Timezone: o.Timezone,
		MemberCount: o.MemberCount, ClientCount: o.ClientCount,
	}
	if o.Logo != nil {
		url := apiPrefix + o.Logo.Path(o.ID)
		out.LogoUrl = &url
	}
	return out
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
	return gen.UpdateOrganization200JSONResponse(s.apiOrganization(o)), nil
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
	return gen.UploadOrganizationLogo200JSONResponse(s.apiOrganization(o)), nil
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
	return gen.RemoveOrganizationLogo200JSONResponse(s.apiOrganization(o)), nil
}

// GetOrganizationLogoById serves an organization's logo inline with its own type, so the app
// header, the portal and emails can show it.
func (s *server) GetOrganizationLogoById(ctx context.Context, req gen.GetOrganizationLogoByIdRequestObject) (gen.GetOrganizationLogoByIdResponseObject, error) {
	f, err := s.organization.OpenLogo(ctx, req.OrganizationId)
	if err != nil {
		return nil, err
	}
	cache := logoCache
	headers := gen.GetOrganizationLogoById200ResponseHeaders{CacheControl: &cache}
	switch f.ContentType {
	case organization.LogoJPEG:
		return gen.GetOrganizationLogoById200ImagejpegResponse{Body: f.Body, ContentLength: f.Size, Headers: headers}, nil
	case organization.LogoWebP:
		return gen.GetOrganizationLogoById200ImagewebpResponse{Body: f.Body, ContentLength: f.Size, Headers: headers}, nil
	default:
		return gen.GetOrganizationLogoById200ImagepngResponse{Body: f.Body, ContentLength: f.Size, Headers: headers}, nil
	}
}

// GetOrganizationLogo serves the branding organization's logo: the address emails sent before
// organizations existed point to.
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

func (s *server) apiOrganization(o organization.Settings) gen.Organization {
	out := gen.Organization{
		Id: o.ID, Name: o.Name, Timezone: o.Timezone, TicketPrefix: o.TicketPrefix, FooterNote: o.FooterNote,
		AppUrl: s.cfg.AppURL.String(),
	}
	if o.Logo != nil {
		url := apiPrefix + o.Logo.Path(o.ID)
		out.LogoUrl = &url
	}
	return out
}
