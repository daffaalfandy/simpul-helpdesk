package httpapi_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/priorities"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/projects"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

func strs[T ~string](vs ...T) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = string(v)
	}
	return out
}

// TestTicketContractMatchesDomain keeps the spec's enums and limits in step with the Go
// domain, so a new status or a changed limit can't reach only one side.
func TestTicketContractMatchesDomain(t *testing.T) {
	spec, err := gen.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	schemas := spec.Components.Schemas
	enum := func(name string) []string {
		var out []string
		for _, v := range schemas[name].Value.Enum {
			out = append(out, fmt.Sprint(v))
		}
		return out
	}
	enums := map[string][]string{
		"TicketType":         strs(ticket.Types...),
		"TicketStatus":       strs(ticket.Statuses...),
		"TicketChannel":      strs(ticket.Channels...),
		"MessageVisibility":  strs(ticket.VisibilityPublic, ticket.VisibilityInternal),
		"TicketProgressStep": strs(ticket.Steps...),
		"TicketProgressState": strs(ticket.StateDone, ticket.StateCurrent, ticket.StateUpcoming,
			ticket.StateSkipped),
		"TicketStatusCause":    strs(ticket.CauseAssignment, ticket.CauseRequesterReply),
		"TicketAssigneeFilter": strs(tickets.AssigneeMe, tickets.AssigneeUnassigned),
		"TicketActivityAction": strs(tickets.ActivityCreated, tickets.ActivityStatusChanged,
			tickets.ActivityAssigneeChanged, tickets.ActivityPriorityChanged, tickets.ActivityReplyAdded,
			tickets.ActivityNoteAdded, tickets.ActivityFirstResponded),
		"Priority":     strs(sla.Priorities...),
		"SlaCalendar":  strs(sla.CalendarBusinessHours, sla.CalendarAlways),
		"DurationUnit": strs(sla.Units...),
		"AtRiskPercent": func() []string {
			out := make([]string, len(sla.AtRiskPercents))
			for i, p := range sla.AtRiskPercents {
				out[i] = fmt.Sprint(int(p))
			}
			return out
		}(),
	}
	for name, want := range enums {
		if got := enum(name); !slices.Equal(got, want) {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}

	limits := []struct {
		schema, field string
		maxLen        int
		maxItems      int
	}{
		{"TicketInput", "subject", tickets.MaxSubjectLen, 0},
		{"TicketInput", "description", tickets.MaxDescriptionLen, 0},
		{"TicketInput", "attachmentIds", 0, tickets.MaxAttachmentsPerMessage},
		{"TicketMessageInput", "body", tickets.MaxMessageLen, 0},
		{"TicketMessageInput", "attachmentIds", 0, tickets.MaxAttachmentsPerMessage},
		{"TicketPriorityInput", "reason", tickets.MaxReasonLen, 0},
		{"ProjectCreate", "name", projects.MaxNameLen, 0},
		{"ProjectCreate", "description", projects.MaxDescriptionLen, 0},
		{"ProjectUpdate", "name", projects.MaxNameLen, 0},
		{"ProjectUpdate", "description", projects.MaxDescriptionLen, 0},
		{"ProjectSla", "targets", 0, len(sla.Priorities)},
		{"PriorityLevel", "name", priorities.MaxNameLen, 0},
		{"PriorityLevel", "description", priorities.MaxDescriptionLen, 0},
	}
	for _, l := range limits {
		prop := schemas[l.schema].Value.Properties[l.field].Value
		if l.maxLen > 0 && (prop.MaxLength == nil || int(*prop.MaxLength) != l.maxLen) {
			t.Errorf("%s.%s maxLength = %v, want %d", l.schema, l.field, prop.MaxLength, l.maxLen)
		}
		if l.maxItems > 0 && (prop.MaxItems == nil || int(*prop.MaxItems) != l.maxItems) {
			t.Errorf("%s.%s maxItems = %v, want %d", l.schema, l.field, prop.MaxItems, l.maxItems)
		}
	}
}
