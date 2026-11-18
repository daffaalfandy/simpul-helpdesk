package reports

import (
	_ "embed"
	"strconv"
	"strings"
	"unicode"

	"github.com/signintech/gopdf"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/palette"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/report"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
)

// Plus Jakarta Sans (SIL Open Font License, fonts/OFL.txt), the app's typeface (HANDOFF §14.1).
var (
	//go:embed fonts/PlusJakartaSans-Regular.ttf
	fontRegular []byte
	//go:embed fonts/PlusJakartaSans-Bold.ttf
	fontBold []byte
	//go:embed fonts/PlusJakartaSans-ExtraBold.ttf
	fontExtraBold []byte
)

const pdfExt = ".pdf"

// Font families. gopdf styles stop at bold, so each weight is a family of its own.
const (
	familyRegular   = "jakarta"
	familyBold      = "jakarta-bold"
	familyExtraBold = "jakarta-extrabold"
	// glyphMissing stands in for characters the font lacks (e.g. emoji in a subject).
	glyphMissing = '?'
	ellipsis     = "…"
	// infoSeparator joins the parts of the file's title in its document properties.
	infoSeparator = " · "
)

// Page geometry in points: A4 portrait, as gopdf.PageSizeA4.
const (
	a4Width         = 595.0
	a4Height        = 842.0
	pageMargin      = 40.0
	footerHeight    = 34.0 // a rule and up to three lines of notes
	footerTop       = a4Height - pageMargin - footerHeight
	contentBottom   = footerTop - sectionGap/2
	contentWidth    = a4Width - 2*pageMargin
	lineGap         = 4.0
	sectionGap      = 22.0
	ruleWidth       = 0.75
	kpiGap          = 8.0
	kpiCount        = 4
	kpiHeight       = 62.0
	kpiPadding      = 10.0
	kpiRadius       = 6.0
	kpiRadiusPoints = 8
	headerRowHeight = 18.0
	rowHeight       = 20.0
	cellPaddingX    = 5.0
	tableLineHeight = 11.0
	cellMaxLines    = 2
	footerLineGap   = 9.0
)

// Font sizes in points.
const (
	sizeOrg       = 9.0
	sizeMeta      = 7.5
	sizeTitle     = 22.0
	sizeClient    = 15.0
	sizeProject   = 12.0
	sizeMonth     = 11.0
	sizeContract  = 8.5
	sizeKPILabel  = 8.0
	sizeKPIValue  = 18.0
	sizeKPIEmpty  = 11.0
	sizeKPISub    = 7.5
	sizeSection   = 11.0
	sizeTableHead = 7.5
	sizeTable     = 8.5
	sizeTicket    = 8.0 // the ticket list, a little smaller so dates and subjects fit
	sizeFooter    = 7.0
)

// Vertical steps between header lines, in points.
const (
	stepTitle    = 18.0
	stepClient   = 30.0
	stepProject  = 20.0
	stepMonth    = 18.0
	stepContract = 15.0
	stepRule     = 14.0
	stepKPILabel = 14.0
	stepKPIValue = 24.0
	stepSection  = 18.0
)

// pdfColumn is a table column: its header label block and width (0 takes what is left).
type pdfColumn struct {
	label string
	width float64
}

var priorityColumns = []pdfColumn{
	{label: labelColPriority},
	{label: labelColTickets, width: 46},
	{label: labelColSplit, width: 86},
	{label: labelColAvgFirst, width: 86},
	{label: labelColWithin, width: 62},
	{label: labelColBreaches, width: 52},
}

var ticketListColumns = []pdfColumn{
	{label: labelColKey, width: 56},
	{label: labelColReported, width: 82},
	{label: labelColPriority, width: 42},
	{label: labelColSubject},
	{label: labelColFirstResponse, width: 66},
	{label: labelColResolved, width: 82},
	{label: labelColWithin, width: 52},
}

// pdfCell is one cell's text and look.
type pdfCell struct {
	lines  []string
	family string
	color  string
}

// buildPDF writes one project's month: the summary, a row per priority and every ticket.
// priorities names the organization's priority levels; a level without a name shows its code.
func buildPDF(d exportData, project db.ListReportProjectsRow, priorities map[sla.Priority]string) (Export, error) {
	body, _, err := renderPDF(d, project, priorities)
	if err != nil {
		return Export{}, err
	}
	c := &copywriter{}
	return Export{Filename: c.fileName(d.Month, pdfStem(project), pdfExt), Body: body}, c.err
}

// pdfStem names the file after the client and the project, or the project's id when neither
// name has an ASCII letter or digit.
func pdfStem(p db.ListReportProjectsRow) string {
	var parts []string
	for _, name := range []string{p.ClientName, p.Name} {
		if s := slug(name); s != "" {
			parts = append(parts, s)
		}
	}
	if len(parts) == 0 {
		return p.ID.String()
	}
	return strings.Join(parts, fileStemJoin)
}

// renderPDF returns the file and its page count.
func renderPDF(d exportData, project db.ListReportProjectsRow, priorities map[sla.Priority]string) ([]byte, int, error) {
	w, err := newPDFWriter()
	if err != nil {
		return nil, 0, err
	}
	c := w.c
	tickets := reportTickets(d.Rows)
	figures := report.Summarize(tickets)
	month := c.month(d.Period)
	w.pdf.SetInfo(gopdf.PdfInfo{
		Title:   c.text(labelHeading, month) + infoSeparator + project.ClientName + infoSeparator + project.Name,
		Author:  d.Org.Name,
		Creator: c.text(labelCreator, nil), Producer: c.text(labelCreator, nil),
		CreationDate: d.Generated,
	})
	w.pdf.AddPage()
	w.y = pageMargin

	// Who and when.
	w.write(familyBold, sizeOrg, "muted", pageMargin, contentWidth, d.Org.Name, gopdf.Left)
	w.write(familyRegular, sizeMeta, "muted", pageMargin, contentWidth, c.generated(d.Generated, d.Location), gopdf.Right)
	w.y += stepTitle
	w.write(familyExtraBold, sizeTitle, "ink", pageMargin, contentWidth, c.text(labelTitle, nil), gopdf.Left)
	w.y += stepClient
	w.font(familyExtraBold, sizeClient)
	w.write(familyExtraBold, sizeClient, "ink", pageMargin, contentWidth, w.fit(project.ClientName, contentWidth), gopdf.Left)
	w.y += stepProject
	w.font(familyBold, sizeProject)
	w.write(familyBold, sizeProject, "ink-2", pageMargin, contentWidth, w.fit(project.Name, contentWidth), gopdf.Left)
	w.y += stepMonth
	w.write(familyRegular, sizeMonth, "ink-2", pageMargin, contentWidth, month, gopdf.Left)
	if contract := c.text(labelContract, contractFor(project)); contract != "" {
		w.y += stepContract
		w.write(familyRegular, sizeContract, "muted", pageMargin, contentWidth, contract, gopdf.Left)
	}
	w.y += stepRule
	w.rule(w.y, "border")
	w.y += sectionGap / 2

	w.kpis(figures)
	w.y += kpiHeight + sectionGap

	if figures.Tickets == 0 {
		w.write(familyRegular, sizeMonth, "ink-2", pageMargin, contentWidth, c.text(labelNoTickets, month), gopdf.Left)
	} else {
		w.section(c.text(labelByPriority, nil))
		widths := columnWidths(priorityColumns)
		w.tableHeader(priorityColumns, widths)
		for _, g := range report.ByPriority(tickets) {
			w.tableRow(widths, sizeTable, w.priorityCells(g, priorities[g.Priority]))
		}
		w.y += sectionGap
		w.section(c.text(labelTickets, nil))
		widths = columnWidths(ticketListColumns)
		w.tableHeader(ticketListColumns, widths)
		for i, row := range d.Rows {
			cells := w.ticketCells(row, tickets[i], d, widths)
			if w.y+rowHeightFor(cells) > contentBottom {
				w.newPage()
				w.tableHeader(ticketListColumns, widths)
			}
			w.tableRow(widths, sizeTicket, cells)
		}
	}

	pages := w.pdf.GetNumberOfPages()
	w.footers(pages, c.waiting(tickets), d.Org.FooterNote)
	if w.err != nil {
		return nil, 0, w.err
	}
	if c.err != nil {
		return nil, 0, c.err
	}
	body, err := w.pdf.GetBytesPdfReturnErr()
	return body, pages, err
}

// Columns of the ticket list that need their width: the subject wraps, first response is cut.
const (
	subjectColumn       = 3
	firstResponseColumn = 4
)

// contractFor is the project's client's contract.
func contractFor(p db.ListReportProjectsRow) contractArgs {
	var a contractArgs
	if p.ContractNo != nil {
		a.No = *p.ContractNo
	}
	if p.ContractStart != nil && p.ContractEnd != nil {
		a.Start, a.End = p.ContractStart.String(), p.ContractEnd.String()
	}
	return a
}

// columnWidths gives the flexible column (width 0) whatever the fixed ones leave.
func columnWidths(cols []pdfColumn) []float64 {
	fixed := 0.0
	for _, col := range cols {
		fixed += col.width
	}
	out := make([]float64, len(cols))
	for i, col := range cols {
		out[i] = col.width
		if col.width == 0 {
			out[i] = contentWidth - fixed
		}
	}
	return out
}

// pdfWriter draws on the PDF and keeps the first error.
type pdfWriter struct {
	pdf *gopdf.GoPdf
	c   *copywriter
	y   float64
	err error
}

func newPDFWriter() (*pdfWriter, error) {
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4, Unit: gopdf.UnitPT})
	opt := gopdf.TtfOption{OnGlyphNotFoundSubstitute: func(rune) rune { return glyphMissing }}
	for family, data := range map[string][]byte{familyRegular: fontRegular, familyBold: fontBold, familyExtraBold: fontExtraBold} {
		if err := pdf.AddTTFFontDataWithOption(family, data, opt); err != nil {
			return nil, err
		}
	}
	return &pdfWriter{pdf: pdf, c: &copywriter{}}, nil
}

func (w *pdfWriter) font(family string, size float64) {
	if w.err == nil {
		w.err = w.pdf.SetFont(family, "", size)
	}
}

func (w *pdfWriter) rgb(name string) (uint8, uint8, uint8) {
	r, g, b, err := palette.RGB(name)
	if err != nil && w.err == nil {
		w.err = err
	}
	return r, g, b
}

// write puts one line of text in a box of the given width at the current y.
func (w *pdfWriter) write(family string, size float64, color string, x, width float64, s string, align int) {
	w.font(family, size)
	w.pdf.SetTextColor(w.rgb(color))
	w.pdf.SetXY(x, w.y)
	if w.err == nil {
		w.err = w.pdf.CellWithOption(&gopdf.Rect{W: width, H: size}, clean(s), gopdf.CellOption{Align: align | gopdf.Top})
	}
}

func (w *pdfWriter) measure(s string) float64 {
	if w.err != nil {
		return 0
	}
	width, err := w.pdf.MeasureTextWidth(s)
	if err != nil {
		w.err = err
	}
	return width
}

// fit shortens s with an ellipsis until it fits width, in the current font.
func (w *pdfWriter) fit(s string, width float64) string {
	s = clean(s)
	if w.measure(s) <= width {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && w.err == nil {
		runes = runes[:len(runes)-1]
		t := strings.TrimRightFunc(string(runes), unicode.IsSpace) + ellipsis
		if w.measure(t) <= width {
			return t
		}
	}
	return ellipsis
}

// wrap breaks s into at most maxLines lines of width; the last one ends in an ellipsis when cut.
func (w *pdfWriter) wrap(s string, width float64, maxLines int) []string {
	s = clean(s)
	if w.err != nil {
		return nil
	}
	lines, err := w.pdf.SplitTextWithWordWrap(s, width)
	if err != nil {
		w.err = err
		return nil
	}
	if len(lines) > maxLines {
		lines = append(lines[:maxLines-1], w.fit(strings.Join(lines[maxLines-1:], " "), width))
	}
	return lines
}

func (w *pdfWriter) rule(y float64, color string) {
	w.pdf.SetStrokeColor(w.rgb(color))
	w.pdf.SetLineWidth(ruleWidth)
	w.pdf.Line(pageMargin, y, pageMargin+contentWidth, y)
}

func (w *pdfWriter) newPage() {
	w.pdf.AddPage()
	w.y = pageMargin
}

func (w *pdfWriter) section(title string) {
	w.write(familyBold, sizeSection, "ink", pageMargin, contentWidth, title, gopdf.Left)
	w.y += stepSection
}

// kpis draws the four boxes of the report page: tickets, within SLA, breaches, first response.
func (w *pdfWriter) kpis(f report.Figures) {
	c := w.c
	within, withinColor := c.percent(f.WithinPercent), "ink"
	if f.WithinPercent != nil {
		withinColor = "success-fg"
		if f.BelowTarget() {
			withinColor = "danger-fg"
		}
	}
	boxes := []struct {
		label, value, sub, color string
		empty                    bool
	}{
		{c.text(labelKPITickets, nil), itoa(f.Tickets),
			c.text(labelKPISplit, splitArgs{Incidents: f.Incidents, Requests: f.Requests}), "ink", false},
		{c.text(labelKPIWithin, nil), within, c.text(labelKPIWithinSub, formatPercent(report.TargetPercent)),
			withinColor, f.WithinPercent == nil},
		{c.text(labelKPIBreaches, nil), itoa(f.Breaches), c.text(labelKPIBreachesSub, nil), "ink", false},
		{c.text(labelKPIAvgFirstResponse, nil), c.span(f.AvgFirstResponse), c.text(labelKPIAvgFirstSub, nil),
			"ink", f.AvgFirstResponse == nil},
	}
	boxWidth := (contentWidth - kpiGap*(kpiCount-1)) / kpiCount
	top := w.y
	inner := boxWidth - 2*kpiPadding
	for i, b := range boxes {
		x := pageMargin + float64(i)*(boxWidth+kpiGap)
		w.pdf.SetStrokeColor(w.rgb("border"))
		w.pdf.SetLineWidth(ruleWidth)
		if w.err == nil {
			w.err = w.pdf.Rectangle(x, top, x+boxWidth, top+kpiHeight, "D", kpiRadius, kpiRadiusPoints)
		}
		w.y = top + kpiPadding
		w.write(familyBold, sizeKPILabel, "muted", x+kpiPadding, inner, b.label, gopdf.Left)
		w.y += stepKPILabel
		if b.empty {
			w.write(familyBold, sizeKPIEmpty, "muted", x+kpiPadding, inner, b.value, gopdf.Left)
		} else {
			w.write(familyExtraBold, sizeKPIValue, b.color, x+kpiPadding, inner, b.value, gopdf.Left)
		}
		w.y += stepKPIValue
		w.font(familyRegular, sizeKPISub)
		w.write(familyRegular, sizeKPISub, "muted", x+kpiPadding, inner, w.fit(b.sub, inner), gopdf.Left)
	}
	w.y = top
}

func (w *pdfWriter) tableHeader(cols []pdfColumn, widths []float64) {
	w.pdf.SetFillColor(w.rgb("tint"))
	w.pdf.RectFromUpperLeftWithStyle(pageMargin, w.y, contentWidth, headerRowHeight, "F")
	x := pageMargin
	top := w.y
	for i, col := range cols {
		w.y = top + (headerRowHeight-sizeTableHead)/2
		w.write(familyBold, sizeTableHead, "muted", x+cellPaddingX, widths[i]-2*cellPaddingX, w.c.text(col.label, nil), gopdf.Left)
		x += widths[i]
	}
	w.y = top + headerRowHeight
}

// rowHeightFor is a row's height: one line at least, more when a cell wraps.
func rowHeightFor(cells []pdfCell) float64 {
	lines := 1
	for _, c := range cells {
		lines = max(lines, len(c.lines))
	}
	return max(rowHeight, float64(lines)*tableLineHeight+2*lineGap)
}

func (w *pdfWriter) tableRow(widths []float64, size float64, cells []pdfCell) {
	top := w.y
	x := pageMargin
	for i, cell := range cells {
		w.y = top + lineGap + (tableLineHeight-size)/2
		for _, line := range cell.lines {
			w.write(cell.family, size, cell.color, x+cellPaddingX, widths[i]-2*cellPaddingX, line, gopdf.Left)
			w.y += tableLineHeight
		}
		x += widths[i]
	}
	w.y = top + rowHeightFor(cells)
	w.rule(w.y, "border-subtle")
}

func (w *pdfWriter) priorityCells(g report.PriorityGroup, name string) []pdfCell {
	c := w.c
	widths := columnWidths(priorityColumns)
	w.font(familyBold, sizeTable)
	label := w.fit(c.text(labelPriorityLevel, priorityArgs{Code: string(g.Priority), Name: name}), widths[0]-2*cellPaddingX)
	within := plain(c.percent(g.WithinPercent))
	if g.BelowTarget() {
		within = pdfCell{lines: []string{c.percent(g.WithinPercent)}, family: familyBold, color: "danger-fg"}
	}
	return []pdfCell{
		{lines: []string{label}, family: familyBold, color: "ink"},
		plain(itoa(g.Tickets)),
		plain(itoa(g.Incidents) + " / " + itoa(g.Requests)),
		plain(c.span(g.AvgFirstResponse)),
		within,
		plain(itoa(g.Breaches)),
	}
}

func (w *pdfWriter) ticketCells(row db.ListReportTicketsRow, t report.Ticket, d exportData, widths []float64) []pdfCell {
	c := w.c
	done := row.ResolvedAt != nil
	first := c.text(labelOutcome, report.ClockOutcome(row.FirstResponseMetAt != nil, t.FirstResponseBreached, done))
	if t.FirstResponse != nil {
		first += " · " + c.span(t.FirstResponse)
	}
	resolved := localTime(row.ResolvedAt, d.Location)
	if resolved == "" {
		resolved = c.text(labelOpen, nil)
	}
	within := pdfCell{lines: []string{c.yesNo(true)}, family: familyBold, color: "success-fg"}
	if t.Breached() {
		within = pdfCell{lines: []string{c.yesNo(false)}, family: familyBold, color: "danger-fg"}
	}
	w.font(familyRegular, sizeTicket)
	return []pdfCell{
		{lines: []string{row.Key}, family: familyBold, color: "ink"},
		plain(localTime(&row.CreatedAt, d.Location)),
		plain(row.Priority),
		{lines: w.wrap(row.Subject, widths[subjectColumn]-2*cellPaddingX, cellMaxLines), family: familyRegular, color: "ink"},
		{lines: w.wrap(first, widths[firstResponseColumn]-2*cellPaddingX, cellMaxLines), family: familyRegular, color: "ink-2"},
		plain(resolved),
		within,
	}
}

func plain(s string) pdfCell {
	return pdfCell{lines: []string{s}, family: familyRegular, color: "ink-2"}
}

// footers draws each page's foot once the page count is known: the notes and "Page n of N".
func (w *pdfWriter) footers(pages int, waiting string, footerNote *string) {
	notes := []string{}
	if waiting != "" {
		notes = append(notes, waiting)
	}
	notes = append(notes, w.c.text(labelFirstResponseNote, nil))
	if footerNote != nil && *footerNote != "" {
		notes = append(notes, *footerNote)
	}
	for page := 1; page <= pages; page++ {
		if w.err == nil {
			w.err = w.pdf.SetPage(page)
		}
		w.rule(footerTop, "border")
		w.y = footerTop + lineGap
		label := w.c.text(labelPage, pageArgs{Page: page, Pages: pages})
		w.write(familyBold, sizeFooter, "muted", pageMargin, contentWidth, label, gopdf.Right)
		for _, note := range notes {
			w.font(familyRegular, sizeFooter)
			w.write(familyRegular, sizeFooter, "muted", pageMargin, contentWidth-w.measure(label)-kpiGap,
				w.fit(note, contentWidth-w.measure(label)-kpiGap), gopdf.Left)
			w.y += footerLineGap
		}
	}
}

// clean puts text on one line: subjects and names may hold tabs or line breaks.
func clean(s string) string {
	return strings.Join(strings.FieldsFunc(s, unicode.IsSpace), " ")
}

func itoa(n int) string { return strconv.Itoa(n) }
