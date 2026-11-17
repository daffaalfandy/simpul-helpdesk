package mail

import (
	"fmt"
	htmltemplate "html/template"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/palette"
)

// Relation is why someone gets a notification email, for its footer.
type Relation string

const (
	RelationReported   Relation = "reported"   // the ticket's requester
	RelationLogged     Relation = "logged"     // an agent logged the ticket for them
	RelationWatching   Relation = "watching"   // a colleague added them
	RelationAssigned   Relation = "assigned"   // the ticket's assignee
	RelationTeam       Relation = "team"       // staff who worked on it (e.g. unassigned)
	RelationSupervisor Relation = "supervisor" // SLA breach alerts
	// RelationSupervisorDoD: supervisors hear when a Definition of Done is skipped or changed.
	RelationSupervisorDoD Relation = "supervisor_dod"
)

// NoticeData is one notification as a line of an email.
type NoticeData struct {
	Event          string
	Kind           string
	Portal         bool // the recipient is client staff (portal wording)
	ActorName      string
	StatusFrom     string
	StatusTo       string
	PriorityFrom   string
	PriorityTo     string
	Assignee       string
	Reason         string
	SLAClock       string
	SLAMinutesLeft *int
	SLADue         string
	Cause          string // why the status changed, when it wasn't a plain move by hand
	ReopenUntil    string // a resolved ticket's reopen deadline ("19 Nov")
	Code           *CodeData
	Time           string
}

// CodeData is what a code event did (code activity): provider, kind and outcome names from the
// connector, never the commit message.
type CodeData struct {
	Provider string
	Kind     string
	Ref      string
	Branch   string
	Target   string
	Outcome  string
}

// UATData is the "Does it work now?" block of a test request: the answer links open the
// ticket in the portal with that answer's dialog (owner, 16 Nov: sign in, then confirm).
type UATData struct {
	ApproveURL    string
	RejectURL     string
	AutoApproveOn string // "18 Nov"; empty when the test waits for an answer however long it takes
}

// ChangeData is another change of the same transaction ("Also changed").
type ChangeData struct {
	Change       string
	Portal       bool
	StatusFrom   string
	StatusTo     string
	PriorityFrom string
	PriorityTo   string
	Assignee     string
}

// SLALineData is the target line under a requester's email ("Fix target: 4 hours · …").
type SLALineData struct {
	Request      bool // a service request: a delivery target
	TargetValue  int
	TargetUnit   string
	PauseWaiting bool
	PauseUAT     bool
}

// QuoteData is the message an email quotes.
type QuoteData struct {
	Author      string
	FromSupport bool
	Time        string
	Body        string            // Markdown, as written: the text email shows it as is
	BodyHTML    htmltemplate.HTML // Body rendered by richtext.HTML for the HTML email
}

// TicketUpdateData fills the ticket_update template: one user's grouped changes to one ticket.
type TicketUpdateData struct {
	OrgName      string
	LogoURL      string // the organisation logo, absolute; empty without one
	Portal       bool
	Relation     Relation
	TicketKey    string
	Title        string
	Priority     string // P1 to P4
	PriorityName string
	ProjectName  string
	Main         NoticeData
	Changes      []ChangeData // folded into the main notification
	Others       []NoticeData // other notifications in this email
	Quote        *QuoteData
	SLA          *SLALineData // requesters only: the target and what doesn't count
	UAT          *UATData     // a test request while the ticket still waits for it
	TicketURL    string
	SettingsURL  string
}

// DigestTicket is one ticket's block in a summary email.
type DigestTicket struct {
	TicketKey    string
	Title        string
	Priority     string // P1 to P4
	PriorityName string
	ProjectName  string
	TicketURL    string
	Notices      []NoticeData
}

// DigestData fills the digest template: one user's hourly or daily summary.
type DigestData struct {
	OrgName     string
	LogoURL     string // the organisation logo, absolute; empty without one
	Portal      bool
	Daily       bool
	Count       int
	Tickets     []DigestTicket
	SettingsURL string
}

// PicBreachData fills the sla_breach_pic template: a P1 ticket missed its resolution target,
// told to the client's contact person (who may have no account).
type PicBreachData struct {
	OrgName      string
	LogoURL      string // the organisation logo, absolute; empty without one
	ClientName   string
	TicketKey    string
	Title        string
	Priority     string
	PriorityName string
	ProjectName  string
	TargetValue  int
	TargetUnit   string // min, h, wd or d
	PassedAt     string
	TicketURL    string
}

// color names a palette colour; email clients can't read CSS variables.
func color(name string) (htmltemplate.CSS, error) {
	c, err := palette.Hex(name)
	if err != nil {
		return "", fmt.Errorf("mail: %w", err)
	}
	return htmltemplate.CSS(c), nil
}
