// Package tickets is the ticket service: intake, reads, agent actions, attachments and the
// activity log. accessFor is the one place that decides which tickets and messages a user sees.
package tickets

import (
	"time"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/storage"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// Audit actions and target types written by this package. Ticket entries all target the
// ticket, so its activity log is one indexed read.
const (
	ActionCreated         = "ticket.created"
	ActionStatusChanged   = "ticket.status_changed"
	ActionAssigneeChanged = "ticket.assignee_changed"
	ActionPriorityChanged = "ticket.priority_changed"
	ActionMessageAdded    = "ticket.message_added"
	ActionFirstResponded  = "ticket.first_responded"

	ActionAttachmentUploaded = "attachment.uploaded"
	ActionAttachmentsPurged  = "attachment.purged"

	TargetTicket     = "ticket"
	TargetAttachment = "attachment"
)

// Audit data keys. Audit entries can never be erased, so they hold IDs, statuses and staff
// names only: never message bodies, subjects or filenames (UU PDP).
const (
	dataKey           = "key"
	dataType          = "type"
	dataPriority      = "priority"
	dataStatus        = "status"
	dataChannel       = "channel"
	dataClient        = "client_id"
	dataRequester     = "requester_id"
	dataAssignee      = "assignee"
	dataSystem        = "system_id"
	dataCatalogItem   = "catalog_item_id"
	dataAttachmentIDs = "attachment_ids"
	dataMessageID     = "message_id"
	dataVisibility    = "visibility"
	dataCause         = "cause"
	dataSizeBytes     = "size_bytes"
	dataContentType   = "content_type"
	dataCount         = "count"
	dataRefID         = "id"
	dataRefName       = "name"
)

// Field limits, matching the database CHECK constraints.
const (
	MaxSubjectLen     = 200
	MaxDescriptionLen = 20000
	MaxMessageLen     = 20000
	MaxReasonLen      = 500
	MaxSearchLen      = 100
	DefaultPageSize   = 50
	MaxPageSize       = 200
)

type Ref struct {
	ID   uuid.UUID
	Name string
}

// Attachment is a file on a ticket's description or one of its messages.
type Attachment struct {
	ID          uuid.UUID
	Filename    string
	ContentType string
	SizeBytes   int64
	CreatedAt   time.Time
}

// Summary is a ticket in a list.
type Summary struct {
	ID         uuid.UUID
	Key        string
	Type       ticket.Type
	Status     ticket.Status
	Priority   sla.Priority
	Subject    string
	Client     Ref
	Requester  Ref
	Assignee   *Ref
	CreatedAt  time.Time
	UpdatedAt  time.Time
	ResolvedAt *time.Time
}

// Ticket is one ticket with everything its detail page shows.
type Ticket struct {
	Summary
	Channel           ticket.Channel
	Description       string
	RequesterJobTitle *string
	System            *Ref
	CatalogItem       *Ref
	CreatedBy         Ref
	LoggedByStaff     bool // an agent logged it on the requester's behalf
	Attachments       []Attachment
	SLA               ticket.EffectiveSLA
	SLATargets        []sla.Target
	FirstResponseAt   *time.Time
	ClosedAt          *time.Time
	Progress          []ticket.ProgressStep
	// AllowedStatuses are the manual status changes open to the viewer; empty for requesters.
	AllowedStatuses []ticket.Status
}

// Message is one entry of a ticket's conversation.
type Message struct {
	ID             uuid.UUID
	Author         Ref
	AuthorJobTitle *string
	FromSupport    bool // written by vendor staff rather than the client
	Visibility     ticket.Visibility
	Body           string
	Attachments    []Attachment
	CreatedAt      time.Time
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
