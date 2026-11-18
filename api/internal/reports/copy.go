package reports

import (
	_ "embed"
	"fmt"
	"math"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/report"
)

//go:embed templates/labels.txt.tmpl
var labelsText string

var labels = template.Must(template.New("labels").Parse(labelsText))

// Label blocks in templates/labels.txt.tmpl.
const (
	labelFileStem            = "file.stem"
	labelMonthPrefix         = "month."
	labelMonth               = "report.month"
	labelTitle               = "report.title"
	labelHeading             = "report.heading"
	labelGenerated           = "report.generated"
	labelTarget              = "report.target"
	labelContract            = "report.contract"
	labelByPriority          = "report.byPriority"
	labelTickets             = "report.tickets"
	labelNoTickets           = "report.noTickets"
	labelPage                = "report.page"
	labelWaitingPrefix       = "report.waiting."
	labelFirstResponseNote   = "report.firstResponseNote"
	labelKPITickets          = "kpi.tickets"
	labelKPISplit            = "kpi.split"
	labelKPIWithin           = "kpi.within"
	labelKPIWithinSub        = "kpi.withinSub"
	labelKPIBreaches         = "kpi.breaches"
	labelKPIBreachesSub      = "kpi.breachesSub"
	labelKPIBelowTarget      = "kpi.belowTarget"
	labelKPIAvgFirstResponse = "kpi.avgFirstResponse"
	labelKPIAvgFirstSub      = "kpi.avgFirstResponseSub"
	labelPercent             = "value.percent"
	labelPriorityLevel       = "value.priority"
	labelHours               = "value.hours"
	labelMinutes             = "value.minutes"
	labelNoTicketsValue      = "value.noTickets"
	labelNotSet              = "value.notSet"
	labelOpen                = "value.open"
	labelYes                 = "value.yes"
	labelNo                  = "value.no"
	labelOutcome             = "value.outcome"
	labelType                = "value.type"
	labelChannel             = "value.channel"
	labelStatus              = "value.status"
	labelCreator             = "report.creator"
	labelSheetSummary        = "sheet.summary"
	labelSheetTickets        = "sheet.tickets"
	labelColClient           = "col.client"
	labelColProject          = "col.project"
	labelColContract         = "col.contract"
	labelColTickets          = "col.tickets"
	labelColIncidents        = "col.incidents"
	labelColRequests         = "col.requests"
	labelColSplit            = "col.split"
	labelColAvgFirst         = "col.avgFirstResponse"
	labelColAvgFirstMinutes  = "col.avgFirstResponseMinutes"
	labelColWithin           = "col.within"
	labelColBreaches         = "col.breaches"
	labelColBelowTarget      = "col.belowTarget"
	labelColKey              = "col.key"
	labelColType             = "col.type"
	labelColPriority         = "col.priority"
	labelColChannel          = "col.channel"
	labelColSubject          = "col.subject"
	labelColStatus           = "col.status"
	labelColReported         = "col.reported"
	labelColReportedAt       = "col.reportedAt"
	labelColFirstResponse    = "col.firstResponse"
	labelColFirstResponseAt  = "col.firstResponseAt"
	labelColTimeToFirst      = "col.timeToFirstResponse"
	labelColResolved         = "col.resolved"
	labelColResolvedAt       = "col.resolvedAt"
	labelColFirstResponseSLA = "col.firstResponseSla"
	labelColResolutionSLA    = "col.resolutionSla"
)

// Data shapes the label blocks read.
type (
	monthArgs struct {
		Name   string
		Year   int
		ToDate bool
	}
	generatedArgs struct{ At, Zone string }
	contractArgs  struct{ No, Start, End string }
	splitArgs     struct{ Incidents, Requests int }
	pageArgs      struct{ Page, Pages int }
	priorityArgs  struct{ Code, Name string }
)

const (
	// dateTimeLayout writes times in the files: numeric, so it reads the same in any language.
	dateTimeLayout  = "2006-01-02 15:04"
	percentDecimals = 1
	slugSeparator   = '-'
	fileStemJoin    = "-"
)

// copywriter runs label blocks and keeps the first error, so a file is built straight through
// and checked once at the end.
type copywriter struct{ err error }

func (c *copywriter) text(name string, data any) string {
	if c.err != nil {
		return ""
	}
	var b strings.Builder
	if err := labels.ExecuteTemplate(&b, name, data); err != nil {
		c.err = fmt.Errorf("reports: label %s: %w", name, err)
		return ""
	}
	return b.String()
}

// month names the period, e.g. "November 2026 (to date)".
func (c *copywriter) month(p Period) string {
	return c.text(labelMonth, monthArgs{
		Name: c.text(labelMonthPrefix+strconv.Itoa(int(p.Month.Month)), nil), Year: p.Month.Year, ToDate: p.ToDate,
	})
}

// percent writes a share like the web app: one decimal at most, "95.3%", "100%".
func (c *copywriter) percent(v *float64) string {
	if v == nil {
		return c.text(labelNoTicketsValue, nil)
	}
	return c.text(labelPercent, formatPercent(*v))
}

// formatPercent writes a share like the web app: one decimal at most, "95.3", "100".
func formatPercent(v float64) string {
	scale := math.Pow10(percentDecimals)
	return strconv.FormatFloat(math.Round(v*scale)/scale, 'f', -1, 64)
}

func (c *copywriter) yesNo(v bool) string {
	if v {
		return c.text(labelYes, nil)
	}
	return c.text(labelNo, nil)
}

// waiting is the footnote on waiting time; empty without tickets.
func (c *copywriter) waiting(rows []report.Ticket) string {
	wp, ok := report.WaitingPauseOf(rows)
	if !ok {
		return ""
	}
	return c.text(labelWaitingPrefix+string(wp), nil)
}

func (c *copywriter) generated(at time.Time, loc *time.Location) string {
	return c.text(labelGenerated, generatedArgs{At: at.In(loc).Format(dateTimeLayout), Zone: loc.String()})
}

// fileName is "sla-report-2026-11.xlsx", or with a stem "sla-report-2026-11-dinas-kominfo-jambi-e-office.pdf".
func (c *copywriter) fileName(m report.Month, stem, ext string) string {
	name := c.text(labelFileStem, m.String())
	if stem != "" {
		name += fileStemJoin + stem
	}
	return name + ext
}

// span writes a time on the clock in hours and minutes, like the web app: "16m", "5h", "1h 20m".
func (c *copywriter) span(d *time.Duration) string {
	if d == nil {
		return c.text(labelNotSet, nil)
	}
	h, m := report.HoursMinutes(*d)
	if h == 0 {
		return c.text(labelMinutes, m)
	}
	if m == 0 {
		return c.text(labelHours, h)
	}
	return c.text(labelHours, h) + " " + c.text(labelMinutes, m)
}

// slug keeps ASCII letters and digits of name, lower-cased, joined by hyphens.
func slug(name string) string {
	var b strings.Builder
	pending := false
	for _, r := range strings.ToLower(name) {
		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			if pending && b.Len() > 0 {
				b.WriteRune(slugSeparator)
			}
			b.WriteRune(r)
			pending = false
			continue
		}
		pending = true
	}
	return b.String()
}

func localTime(t *time.Time, loc *time.Location) string {
	if t == nil {
		return ""
	}
	return t.In(loc).Format(dateTimeLayout)
}
