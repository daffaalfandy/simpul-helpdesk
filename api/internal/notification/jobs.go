package notification

import (
	"time"

	"github.com/google/uuid"
)

// Job kinds. The args live in this pure package so the ticket service can insert them
// without importing the notification service.
const (
	JobKindFanout      = "notify_fanout"
	JobKindDispatch    = "notify_dispatch"
	JobKindTicketEmail = "notify_ticket_email"
	JobKindDigest      = "notify_digest_email"
	JobKindPicEmail    = "notify_sla_pic_email"
	JobKindPurge       = "notify_purge"
)

// FanoutArgs turns one ticket transaction's audit entries into notifications.
type FanoutArgs struct {
	TicketID uuid.UUID   `json:"ticket_id"`
	AuditIDs []uuid.UUID `json:"audit_ids"`
}

func (FanoutArgs) Kind() string { return JobKindFanout }

// DispatchArgs finds pending emails that are due.
type DispatchArgs struct{}

func (DispatchArgs) Kind() string { return JobKindDispatch }

// TicketEmailArgs sends one user the pending changes to one ticket. FirstID (the oldest
// pending row) names the group, so the same group is never queued twice and a later group
// always gets its own job.
type TicketEmailArgs struct {
	UserID   uuid.UUID `json:"user_id"`
	TicketID uuid.UUID `json:"ticket_id"`
	FirstID  uuid.UUID `json:"first_id"`
}

func (TicketEmailArgs) Kind() string { return JobKindTicketEmail }

// DigestArgs sends one user's hourly or daily summary of the routine items created before
// Before. Period names the summary; FirstID (the oldest due row) names the group, so a row that
// arrives after its period's summary went out still gets one instead of waiting a whole period.
type DigestArgs struct {
	UserID  uuid.UUID `json:"user_id"`
	Period  string    `json:"period"`
	Before  time.Time `json:"before"`
	FirstID uuid.UUID `json:"first_id"`
}

func (DigestArgs) Kind() string { return JobKindDigest }

// PicEmailArgs tells a client's contact person that a P1 ticket missed its resolution target.
type PicEmailArgs struct {
	TicketID uuid.UUID `json:"ticket_id"`
}

func (PicEmailArgs) Kind() string { return JobKindPicEmail }

// PurgeArgs deletes notifications older than Retention.
type PurgeArgs struct{}

func (PurgeArgs) Kind() string { return JobKindPurge }
