package reports

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/palette"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/report"
)

const excelExt = ".xlsx"

// Summary sheet rows: title block, the KPI block, then the project table.
const (
	rowOrg = 1 + iota
	rowHeading
	rowGenerated
	rowTarget
	_
	rowKPILabels
	rowKPIValues
	_
	rowProjectHeader
)

const (
	rowTicketHeader = 1
	firstColumn     = 1
	// footnoteGap leaves an empty row between a table and its footnotes.
	footnoteGap = 2
)

// Excel formats and font sizes (points).
const (
	excelPercentFormat  = "0.0%"
	excelDateTimeFormat = "yyyy-mm-dd hh:mm"
	excelTitleSize      = 14
	excelHeadingSize    = 12
	excelFillPattern    = 1 // solid
	excelFillType       = "pattern"
	excelBorderBottom   = "bottom"
	excelBorderThin     = 1
	excelPaneBottomLeft = "bottomLeft"
	percentToFraction   = 100.0
)

// cellFormat is a number format a column's values need.
type cellFormat int

const (
	cellPlain cellFormat = iota
	cellPercent
	cellDateTime
)

// column is a table column: its header label block, its width in characters and its format.
type column struct {
	label  string
	arg    any
	width  float64
	format cellFormat
}

var summaryColumns = []column{
	{label: labelColClient, width: 34},
	{label: labelColProject, width: 28},
	{label: labelColContract, width: 22},
	{label: labelColTickets, width: 10},
	{label: labelColIncidents, width: 11},
	{label: labelColRequests, width: 11},
	{label: labelColAvgFirstMinutes, width: 24},
	{label: labelColWithin, width: 13, format: cellPercent},
	{label: labelColBreaches, width: 11},
	{label: labelColBelowTarget, width: 14},
}

var kpiColumns = []column{
	{label: labelKPITickets},
	{label: labelColIncidents},
	{label: labelColRequests},
	{label: labelKPIWithin, format: cellPercent},
	{label: labelKPIBreaches},
	{label: labelKPIBelowTarget},
	{label: labelColAvgFirstMinutes},
}

// ticketColumns name the time zone the times are written in.
func ticketColumns(zone string) []column {
	return []column{
		{label: labelColKey, width: 12},
		{label: labelColClient, width: 30},
		{label: labelColProject, width: 24},
		{label: labelColType, width: 16},
		{label: labelColPriority, width: 9},
		{label: labelColChannel, width: 13},
		{label: labelColSubject, width: 48},
		{label: labelColStatus, width: 18},
		{label: labelColReportedAt, arg: zone, width: 24, format: cellDateTime},
		{label: labelColFirstResponseAt, arg: zone, width: 28, format: cellDateTime},
		{label: labelColTimeToFirst, width: 26},
		{label: labelColResolvedAt, arg: zone, width: 24, format: cellDateTime},
		{label: labelColFirstResponseSLA, width: 18},
		{label: labelColResolutionSLA, width: 16},
		{label: labelColWithin, width: 11},
	}
}

// buildExcel writes the range for every project. Text goes in as plain strings, never formulas,
// so a subject starting with "=" stays text.
func buildExcel(d exportData) (Export, error) {
	c := &copywriter{}
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	styles, err := newExcelStyles(f)
	if err != nil {
		return Export{}, err
	}

	contracts := make(map[uuid.UUID]*string, len(d.Projects))
	for _, p := range d.Projects {
		contracts[p.ID] = p.ContractNo
	}
	tickets := reportTickets(d.Rows)
	rep := report.Build(reportProjects(d.Projects), tickets)

	summary := &sheet{f: f, name: c.text(labelSheetSummary, nil), styles: styles}
	summary.err = f.SetSheetName(f.GetSheetName(0), summary.name)
	summary.row(rowOrg, d.Org.Name)
	summary.style(rowOrg, firstColumn, firstColumn, styles.title)
	summary.row(rowHeading, c.text(labelHeading, c.period(d.Period)))
	summary.style(rowHeading, firstColumn, firstColumn, styles.heading)
	summary.row(rowGenerated, c.generated(d.Generated, d.Location))
	summary.row(rowTarget, c.text(labelTarget, formatPercent(report.TargetPercent)))

	summary.header(c, rowKPILabels, kpiColumns)
	t := rep.Totals
	summary.row(rowKPIValues, t.Tickets, t.Incidents, t.Requests, excelPercent(c, t.WithinPercent), t.Breaches,
		rep.ProjectsBelowTarget, excelMinutes(t.AvgFirstResponse))
	summary.formats(kpiColumns, rowKPIValues, rowKPIValues)

	summary.header(c, rowProjectHeader, summaryColumns)
	r := rowProjectHeader
	for _, g := range rep.Projects {
		r++
		contract := ""
		if no := contracts[g.ID]; no != nil {
			contract = *no
		}
		summary.row(r, g.Client.Name, g.Name, contract, g.Tickets, g.Incidents, g.Requests, excelMinutes(g.AvgFirstResponse),
			excelPercent(c, g.WithinPercent), g.Breaches, c.yesNo(g.BelowTarget()))
		if g.BelowTarget() {
			summary.style(r, len(summaryColumns), len(summaryColumns), styles.alert)
		}
	}
	summary.formats(summaryColumns, rowProjectHeader+1, r)
	summary.filter(rowProjectHeader, r, len(summaryColumns))
	summary.freezeThrough(rowProjectHeader)
	r += footnoteGap
	if note := c.waiting(tickets); note != "" {
		summary.row(r, note)
		r++
	}
	summary.row(r, c.text(labelFirstResponseNote, nil))
	summary.widths(summaryColumns)

	list := &sheet{f: f, name: c.text(labelSheetTickets, nil), styles: styles}
	if _, err := f.NewSheet(list.name); err != nil {
		return Export{}, err
	}
	cols := ticketColumns(d.Location.String())
	list.header(c, rowTicketHeader, cols)
	for i, row := range d.Rows {
		tk := tickets[i]
		done := row.ResolvedAt != nil
		list.row(rowTicketHeader+1+i,
			row.Key, row.ClientName, row.ProjectName, c.text(labelType, row.Type), row.Priority,
			c.text(labelChannel, row.Channel), row.Subject, c.text(labelStatus, row.Status),
			excelTime(&row.CreatedAt, d.Location), excelTime(row.FirstResponseAt, d.Location),
			excelMinutes(tk.FirstResponse), excelTime(row.ResolvedAt, d.Location),
			c.text(labelOutcome, report.ClockOutcome(row.FirstResponseMetAt != nil, tk.FirstResponseBreached, done)),
			c.text(labelOutcome, report.ClockOutcome(done, tk.ResolutionBreached, done)),
			c.yesNo(!tk.Breached()))
	}
	last := rowTicketHeader + len(d.Rows)
	list.formats(cols, rowTicketHeader+1, last)
	list.filter(rowTicketHeader, last, len(cols))
	list.freezeThrough(rowTicketHeader)
	list.widths(cols)

	for _, err := range []error{c.err, summary.err, list.err} {
		if err != nil {
			return Export{}, err
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return Export{}, err
	}
	return Export{Filename: c.fileName(d.Range, "", excelExt), Body: buf.Bytes()}, nil
}

// excelPercent is a share as a fraction for the percent format, or the "No tickets" text.
func excelPercent(c *copywriter, v *float64) any {
	if v == nil {
		return c.text(labelNoTicketsValue, nil)
	}
	return *v / percentToFraction
}

// excelMinutes is whole minutes on the clock; empty without a value.
func excelMinutes(d *time.Duration) any {
	if d == nil {
		return nil
	}
	return int(*d / time.Minute)
}

// excelTime is t's wall-clock time in loc. Spreadsheet times carry no zone; the header names it.
func excelTime(t *time.Time, loc *time.Location) any {
	if t == nil {
		return nil
	}
	l := t.In(loc)
	return time.Date(l.Year(), l.Month(), l.Day(), l.Hour(), l.Minute(), l.Second(), 0, time.UTC)
}

type excelStyles struct {
	title, heading, header, percent, dateTime, alert int
}

func newExcelStyles(f *excelize.File) (excelStyles, error) {
	ink, err := palette.Hex("ink")
	if err != nil {
		return excelStyles{}, err
	}
	tint, err := palette.Hex("tint")
	if err != nil {
		return excelStyles{}, err
	}
	border, err := palette.Hex("border-strong")
	if err != nil {
		return excelStyles{}, err
	}
	alert, err := palette.Hex("danger-fg")
	if err != nil {
		return excelStyles{}, err
	}
	percent, dateTime := excelPercentFormat, excelDateTimeFormat
	defs := []*excelize.Style{
		{Font: &excelize.Font{Bold: true, Size: excelTitleSize, Color: ink}},
		{Font: &excelize.Font{Bold: true, Size: excelHeadingSize, Color: ink}},
		{
			Font:   &excelize.Font{Bold: true, Color: ink},
			Fill:   excelize.Fill{Type: excelFillType, Pattern: excelFillPattern, Color: []string{tint}},
			Border: []excelize.Border{{Type: excelBorderBottom, Color: border, Style: excelBorderThin}},
		},
		{CustomNumFmt: &percent},
		{CustomNumFmt: &dateTime},
		{Font: &excelize.Font{Bold: true, Color: alert}},
	}
	ids := make([]int, len(defs))
	for i, def := range defs {
		if ids[i], err = f.NewStyle(def); err != nil {
			return excelStyles{}, err
		}
	}
	return excelStyles{title: ids[0], heading: ids[1], header: ids[2], percent: ids[3], dateTime: ids[4], alert: ids[5]}, nil
}

// sheet writes one worksheet and keeps the first error.
type sheet struct {
	f      *excelize.File
	name   string
	styles excelStyles
	err    error
}

func (s *sheet) cell(col, row int) string {
	if s.err != nil {
		return ""
	}
	name, err := excelize.CoordinatesToCellName(col, row)
	if err != nil {
		s.err = err
	}
	return name
}

func (s *sheet) row(r int, values ...any) {
	start := s.cell(firstColumn, r)
	if s.err == nil {
		s.err = s.f.SetSheetRow(s.name, start, &values)
	}
}

func (s *sheet) header(c *copywriter, r int, cols []column) {
	values := make([]any, len(cols))
	for i, col := range cols {
		values[i] = c.text(col.label, col.arg)
	}
	s.row(r, values...)
	s.style(r, firstColumn, len(cols), s.styles.header)
}

// formats applies each column's number format to rows from..to.
func (s *sheet) formats(cols []column, from, to int) {
	for i, col := range cols {
		switch col.format {
		case cellPercent:
			s.cellsStyle(from, to, firstColumn+i, s.styles.percent)
		case cellDateTime:
			s.cellsStyle(from, to, firstColumn+i, s.styles.dateTime)
		}
	}
}

// style applies a style to columns from..to of row r.
func (s *sheet) style(r, from, to, style int) {
	a, b := s.cell(from, r), s.cell(to, r)
	if s.err == nil {
		s.err = s.f.SetCellStyle(s.name, a, b, style)
	}
}

// cellsStyle applies a style to one column over rows from..to.
func (s *sheet) cellsStyle(from, to, col, style int) {
	if to < from {
		return
	}
	a, b := s.cell(col, from), s.cell(col, to)
	if s.err == nil {
		s.err = s.f.SetCellStyle(s.name, a, b, style)
	}
}

func (s *sheet) filter(headerRow, lastRow, cols int) {
	ref := fmt.Sprintf("%s:%s", s.cell(firstColumn, headerRow), s.cell(cols, lastRow))
	if s.err == nil {
		s.err = s.f.AutoFilter(s.name, ref, nil)
	}
}

// freezeThrough keeps rows 1..header in view while the rest scrolls.
func (s *sheet) freezeThrough(header int) {
	top := s.cell(firstColumn, header+1)
	if s.err == nil {
		s.err = s.f.SetPanes(s.name, &excelize.Panes{
			Freeze: true, YSplit: header, TopLeftCell: top, ActivePane: excelPaneBottomLeft,
		})
	}
}

func (s *sheet) widths(cols []column) {
	for i, col := range cols {
		if s.err != nil {
			return
		}
		name, err := excelize.ColumnNumberToName(firstColumn + i)
		if err != nil {
			s.err = err
			return
		}
		s.err = s.f.SetColWidth(s.name, name, name, col.width)
	}
}
