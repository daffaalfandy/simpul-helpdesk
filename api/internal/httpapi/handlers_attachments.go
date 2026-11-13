package httpapi

import (
	"context"
	"mime"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

const (
	uploadFieldFile          = "file"
	dispositionAttachment    = "attachment"
	dispositionFilenameParam = "filename"
	// downloadCSP stops a downloaded file from running anything if a browser renders it anyway.
	downloadCSP     = "default-src 'none'; sandbox"
	uploadNeedsFile = "send one file in the \"" + uploadFieldFile + "\" form field"
)

func (s *server) GetAttachmentPolicy(ctx context.Context, _ gen.GetAttachmentPolicyRequestObject) (gen.GetAttachmentPolicyResponseObject, error) {
	if _, err := signedIn(ctx); err != nil {
		return nil, err
	}
	p := tickets.AttachmentPolicy()
	return gen.GetAttachmentPolicy200JSONResponse{MaxBytes: p.MaxBytes, MaxPerMessage: p.MaxPerMessage, Extensions: p.Extensions}, nil
}

// UploadAttachment streams the first part of a multipart body to storage. This route skips
// the spec validator (see streamedRoutes), so the part is checked here and in the service.
func (s *server) UploadAttachment(ctx context.Context, req gen.UploadAttachmentRequestObject) (gen.UploadAttachmentResponseObject, error) {
	p, err := signedIn(ctx)
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
	if part.FormName() != uploadFieldFile || part.FileName() == "" {
		return nil, apperr.InvalidRequest(uploadNeedsFile)
	}
	a, err := s.tickets.Upload(ctx, p, part.FileName(), part)
	if err != nil {
		return nil, err
	}
	return gen.UploadAttachment201JSONResponse(apiAttachment(a)), nil
}

// DownloadAttachment sends a file as a download, never inline.
func (s *server) DownloadAttachment(ctx context.Context, req gen.DownloadAttachmentRequestObject) (gen.DownloadAttachmentResponseObject, error) {
	p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	d, err := s.tickets.Open(ctx, p, req.AttachmentId)
	if err != nil {
		return nil, err
	}
	disposition := mime.FormatMediaType(dispositionAttachment, map[string]string{dispositionFilenameParam: d.Filename})
	if disposition == "" {
		disposition = dispositionAttachment
	}
	cache, csp := cacheNoStore, downloadCSP
	return gen.DownloadAttachment200ApplicationoctetStreamResponse{
		Body: d.Body, ContentLength: d.Size,
		Headers: gen.DownloadAttachment200ResponseHeaders{
			ContentDisposition: &disposition, CacheControl: &cache, ContentSecurityPolicy: &csp,
		},
	}, nil
}
