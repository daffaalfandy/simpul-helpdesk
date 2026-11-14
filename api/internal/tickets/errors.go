package tickets

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound()
	}
	return err
}

func errTicketClosed() error {
	return apperr.Conflict(apperr.CodeTicketClosed, "This ticket is closed", "Closed tickets can't be changed.")
}

// errStatusChanged: the caller moved a card from a status the ticket no longer has.
func errStatusChanged() error {
	return apperr.Conflict(apperr.CodeStatusChanged, "Someone else changed this ticket's status", "")
}

// moveError maps a lifecycle rule to an API error.
func moveError(err error) error {
	if errors.Is(err, ticket.ErrTicketClosed) {
		return errTicketClosed()
	}
	return apperr.Conflict(apperr.CodeInvalidStatusChange, "This status change isn't allowed", err.Error())
}

func errInvalidAttachment() error {
	return apperr.Invalid(apperr.CodeInvalidAttachment, "One of the files is no longer available", "Upload it again.")
}

func errTooManyAttachments() error {
	return apperr.Invalid(apperr.CodeTooManyAttachments, "Too many files", "")
}

// bytesPerMiB turns MaxAttachmentBytes into the "10 MB" users see.
const bytesPerMiB = 1 << 20

func errAttachmentTooLarge() error {
	return apperr.TooLarge(apperr.CodeAttachmentTooLarge,
		fmt.Sprintf("The file is larger than %d MB", MaxAttachmentBytes/bytesPerMiB), "")
}
