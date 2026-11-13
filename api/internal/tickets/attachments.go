package tickets

import (
	"bufio"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/storage"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
)

// Policy is what the upload form needs to check files before sending them.
type Policy struct {
	MaxBytes      int64
	MaxPerMessage int
	Extensions    []string
}

// AttachmentPolicy returns the upload limits and allowed extensions.
func AttachmentPolicy() Policy {
	return Policy{MaxBytes: MaxAttachmentBytes, MaxPerMessage: MaxAttachmentsPerMessage, Extensions: Extensions()}
}

// Upload stores one file as a pending attachment of p. It stays pending until a new ticket or
// message links it, and is purged after PendingAttachmentTTL. The file streams to disk before
// the short transaction, so no database connection waits on a slow upload.
func (s *Service) Upload(ctx context.Context, p *auth.Principal, filename string, r io.Reader) (Attachment, error) {
	if _, err := accessFor(p); err != nil {
		return Attachment{}, err
	}
	name, err := SanitizeFilename(filename)
	if err != nil {
		return Attachment{}, apperr.Invalid(apperr.CodeInvalidAttachment, "The file needs a name", "")
	}
	pending, err := s.store.Q().CountPendingAttachments(ctx, db.CountPendingAttachmentsParams{
		UploadedBy: p.UserID, NotBefore: s.now().Add(-PendingAttachmentTTL),
	})
	if err != nil {
		return Attachment{}, err
	}
	if pending >= MaxPendingAttachmentsPerUser {
		return Attachment{}, apperr.Conflict(apperr.CodeTooManyPendingAttachments,
			"Too many files waiting to be sent", "Send or remove some first.")
	}

	br := bufio.NewReaderSize(r, SniffBytes)
	head, err := br.Peek(SniffBytes)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return Attachment{}, uploadError(err)
	}
	contentType, err := Classify(name, head)
	switch {
	case errors.Is(err, ErrEmptyFile):
		return Attachment{}, apperr.Invalid(apperr.CodeAttachmentEmpty, "The file is empty", "")
	case errors.Is(err, ErrTypeNotAllowed):
		return Attachment{}, apperr.Invalid(apperr.CodeAttachmentTypeNotAllowed, "This file type isn't allowed",
			strings.Join(Extensions(), ", "))
	case err != nil:
		return Attachment{}, err
	}

	key := storage.NewKey()
	sum := sha256.New()
	size, err := s.files.Put(ctx, key, io.TeeReader(io.LimitReader(br, MaxAttachmentBytes+1), sum))
	if err != nil {
		return Attachment{}, uploadError(err)
	}
	if size > MaxAttachmentBytes {
		_ = s.files.Delete(ctx, key)
		return Attachment{}, errAttachmentTooLarge()
	}

	out := Attachment{Filename: name, ContentType: contentType, SizeBytes: size}
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		created, err := tx.Q.CreateAttachment(ctx, db.CreateAttachmentParams{
			StorageKey: key, Filename: name, ContentType: contentType, SizeBytes: size,
			Sha256: sum.Sum(nil), UploadedBy: p.UserID,
		})
		if err != nil {
			return err
		}
		out.ID, out.CreatedAt = created.ID, created.CreatedAt
		return audit.Write(ctx, tx.Q, p.AuditActor().Entry(ActionAttachmentUploaded, TargetAttachment, &created.ID,
			map[string]any{dataSizeBytes: size, dataContentType: contentType}))
	})
	if err != nil {
		_ = s.files.Delete(ctx, key)
		return Attachment{}, err
	}
	return out, nil
}

// uploadError maps a body over the request limit to the attachment error.
func uploadError(err error) error {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		return errAttachmentTooLarge()
	}
	return err
}

// Download is an attachment ready to send.
type Download struct {
	Filename    string
	ContentType string
	Size        int64
	Body        io.ReadSeekCloser
}

// Open returns a linked attachment p may see: on a ticket in scope and, for requesters, not on
// an internal note. Pending uploads are never served. Callers close Body.
func (s *Service) Open(ctx context.Context, p *auth.Principal, id uuid.UUID) (Download, error) {
	a, err := accessFor(p)
	if err != nil {
		return Download{}, err
	}
	row, err := s.store.Q().GetAttachmentForDownload(ctx, db.GetAttachmentForDownloadParams{
		ID: id, Visibility: a.visibility, ScopeClientID: a.scopeClientID, ScopeAgentID: a.scopeAgentID,
	})
	if err != nil {
		return Download{}, notFound(err)
	}
	body, size, err := s.files.Open(ctx, row.StorageKey)
	if errors.Is(err, storage.ErrNotFound) {
		return Download{}, apperr.NotFound()
	}
	if err != nil {
		return Download{}, err
	}
	return Download{Filename: row.Filename, ContentType: row.ContentType, Size: size, Body: body}, nil
}

// linkAttachments attaches p's pending uploads to a ticket (messageID nil: its description)
// or one of its messages, in the caller's transaction.
func (s *Service) linkAttachments(ctx context.Context, q *db.Queries, uploader, ticketID uuid.UUID,
	messageID *uuid.UUID, ids []uuid.UUID) ([]Attachment, error) {
	if len(ids) == 0 {
		return []Attachment{}, nil
	}
	rows, err := q.LinkAttachments(ctx, db.LinkAttachmentsParams{
		Ids: ids, UploadedBy: uploader, TicketID: &ticketID, MessageID: messageID,
		NotBefore: s.now().Add(-PendingAttachmentTTL),
	})
	if err != nil {
		return nil, err
	}
	if len(rows) != len(ids) {
		return nil, errInvalidAttachment()
	}
	out := make([]Attachment, 0, len(rows))
	for _, r := range rows {
		out = append(out, Attachment{ID: r.ID, Filename: r.Filename, ContentType: r.ContentType,
			SizeBytes: r.SizeBytes, CreatedAt: r.CreatedAt})
	}
	slices.SortFunc(out, func(a, b Attachment) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return out, nil
}

// dedupeAttachments drops repeated IDs and enforces the per-message limit.
func dedupeAttachments(ids []uuid.UUID) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	if len(out) > MaxAttachmentsPerMessage {
		return nil, errTooManyAttachments()
	}
	return out, nil
}
