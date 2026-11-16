package notification

import (
	"time"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// Data is what a notification stores about its changes: statuses, priorities, staff names and
// the priority reason. Never message bodies or subjects: the inbox and emails read those from
// the ticket, so erasing them there erases them everywhere.
type Data struct {
	Kind           Kind          `json:"kind,omitempty"`
	StatusFrom     ticket.Status `json:"status_from,omitempty"`
	StatusTo       ticket.Status `json:"status_to,omitempty"`
	PriorityFrom   sla.Priority  `json:"priority_from,omitempty"`
	PriorityTo     sla.Priority  `json:"priority_to,omitempty"`
	Reason         string        `json:"reason,omitempty"`
	Assignee       string        `json:"assignee,omitempty"` // the new assignee's name
	Cause          ticket.Cause  `json:"cause,omitempty"`
	AutoApproveAt  *time.Time    `json:"auto_approve_at,omitempty"`
	ReopenUntil    *time.Time    `json:"reopen_until,omitempty"`
	SLAClock       string        `json:"sla_clock,omitempty"`
	SLADueAt       *time.Time    `json:"sla_due_at,omitempty"`
	SLALeftMinutes *int          `json:"sla_left_minutes,omitempty"`
	Code           *Code         `json:"code,omitempty"`
	Also           []Also        `json:"also,omitempty"`
}

// Also is another change of the same transaction, listed under "Also changed".
type Also struct {
	Change       ChangeKind    `json:"change"`
	StatusFrom   ticket.Status `json:"status_from,omitempty"`
	StatusTo     ticket.Status `json:"status_to,omitempty"`
	PriorityFrom sla.Priority  `json:"priority_from,omitempty"`
	PriorityTo   sla.Priority  `json:"priority_to,omitempty"`
	Assignee     string        `json:"assignee,omitempty"`
	Code         *Code         `json:"code,omitempty"`
}

// DataFor is the stored form of a delivery.
func DataFor(d Delivery) Data {
	c := d.Change
	data := Data{
		Kind: d.Kind, StatusFrom: c.StatusFrom, StatusTo: c.StatusTo, Reason: c.Reason,
		SLAClock: c.SLAClock, SLADueAt: c.SLADueAt, Cause: c.Cause, AutoApproveAt: c.AutoApproveAt,
		ReopenUntil: c.ReopenUntil, Code: c.Code,
	}
	if c.Priority != nil {
		data.PriorityFrom, data.PriorityTo = c.Priority.From, c.Priority.To
	}
	if c.AssigneeTo != nil {
		data.Assignee = c.AssigneeTo.Name
	}
	if c.SLALeft != nil {
		minutes := int(c.SLALeft.Minutes())
		data.SLALeftMinutes = &minutes
	}
	for _, o := range d.Also {
		a := Also{Change: o.Kind, StatusFrom: o.StatusFrom, StatusTo: o.StatusTo, Code: o.Code}
		if o.Priority != nil {
			a.PriorityFrom, a.PriorityTo = o.Priority.From, o.Priority.To
		}
		if o.AssigneeTo != nil {
			a.Assignee = o.AssigneeTo.Name
		}
		data.Also = append(data.Also, a)
	}
	return data
}
