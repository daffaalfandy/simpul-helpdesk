package mail

import (
	"fmt"
	htmltemplate "html/template"
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
	Time           string
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
	Body        string
}

// TicketUpdateData fills the ticket_update template: one user's grouped changes to one ticket.
type TicketUpdateData struct {
	OrgName      string
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

// palette mirrors web/src/styles/tokens.css: email clients can't read CSS variables.
var palette = map[string]htmltemplate.CSS{
	"bg":            "#F5F8FC",
	"surface":       "#FFFFFF",
	"tint":          "#E8EEF7",
	"border":        "#DFE7F2",
	"border-subtle": "#E6EDF6",
	"border-strong": "#CFD9E8",
	"ink":           "#14213D",
	"ink-2":         "#334155",
	"muted":         "#52607A",
	"primary":       "#1D4ED8",
	"on-primary":    "#FFFFFF",
	"warning-bg":    "#FDF0CF",
	"warning-fg":    "#6E4D00",
	"info-bg":       "#EEF4FF",
	"info-fg":       "#1E3A8A",
}

func color(name string) (htmltemplate.CSS, error) {
	c, ok := palette[name]
	if !ok {
		return "", fmt.Errorf("mail: no colour %q", name)
	}
	return c, nil
}
