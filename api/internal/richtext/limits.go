package richtext

import (
	"regexp"
	"strings"
)

// Limits on a body's structure (M10 security review). GFM fills every short table row with
// empty cells up to the header's column count, and every nested quote or list is one more level
// of recursion, so a 20 000-character body could otherwise turn into millions of table cells or
// thousands of levels when it is rendered, in an email or in the web app. Bodies past these
// limits are refused when written (see TooComplex); bodies stored before render as plain text.
// The web app's lib/richTextLimits.ts holds the same rules.
const (
	MaxTableCells   = 5000
	MaxNestingDepth = 32
)

const (
	tablePipe        = "|"
	orderedMarkEnds  = ".)"
	bulletMarks      = "-*+"
	quoteMark        = '>'
	maxOrderedDigits = 9 // CommonMark: an ordered list number has at most nine digits
)

// delimiterRow is a GFM table's second line, such as "| --- | :-: |".
var delimiterRow = regexp.MustCompile(`^\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?$`)

// TooComplex reports whether md has more table cells or deeper nesting than the limits allow.
// It reads the source line by line, never building the tree it guards, and counts generously:
// a table-like line inside a code block counts too.
func TooComplex(md string) bool {
	cells, columns := 0, 0
	for line := range strings.SplitSeq(md, lineBreak) {
		line = strings.TrimSpace(line)
		if nesting(line) > MaxNestingDepth {
			return true
		}
		switch {
		case line == "":
			columns = 0 // a blank line ends a table
		case strings.Contains(line, tablePipe) && delimiterRow.MatchString(line):
			columns = strings.Count(strings.Trim(line, tablePipe), tablePipe) + 1
			cells += columns // the header row
		case columns > 0:
			cells += columns // every row gets the header's cell count
		}
		if cells > MaxTableCells {
			return true
		}
	}
	return false
}

// nesting counts the quote and list markers that open a line ("> - 1. text" is three levels),
// up to one past the limit.
func nesting(line string) int {
	depth := 0
	for {
		line = strings.TrimLeft(line, " \t")
		switch {
		case line == "" || depth > MaxNestingDepth:
			return depth
		case line[0] == quoteMark:
			line = line[1:]
		case len(line) > 1 && strings.IndexByte(bulletMarks, line[0]) >= 0 && isSpace(line[1]):
			line = line[2:]
		default:
			n := orderedMarkerLen(line)
			if n == 0 {
				return depth
			}
			line = line[n:]
		}
		depth++
	}
}

// orderedMarkerLen is the length of an ordered list marker ("12. ", "3) ") opening line, or 0.
func orderedMarkerLen(line string) int {
	digits := 0
	for digits < len(line) && digits <= maxOrderedDigits && line[digits] >= '0' && line[digits] <= '9' {
		digits++
	}
	if digits == 0 || digits > maxOrderedDigits || digits+1 >= len(line) {
		return 0
	}
	if strings.IndexByte(orderedMarkEnds, line[digits]) < 0 || !isSpace(line[digits+1]) {
		return 0
	}
	return digits + 2
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' }
