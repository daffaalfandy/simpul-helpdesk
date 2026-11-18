/**
 * Limits on a body's structure, the same as the API's `richtext` package (limits.go). GFM fills
 * every short table row with empty cells up to the header's column count, and every nested quote
 * or list is one more level of recursion, so a 20 000-character body could otherwise turn into
 * millions of table cells or thousands of levels and freeze or crash the page. The API refuses
 * new bodies past these limits; older ones render as plain text.
 */
export const MAX_TABLE_CELLS = 5000
export const MAX_NESTING_DEPTH = 32

/** CommonMark: an ordered list number has at most nine digits. */
const MAX_ORDERED_DIGITS = 9
const TABLE_PIPE = '|'
const QUOTE_MARK = '>'
const BULLET_MARKS = '-*+'
const ORDERED_MARK_ENDS = '.)'

/** A GFM table's second line, such as "| --- | :-: |". */
const DELIMITER_ROW = /^\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?$/
const LEADING_SPACE = /^[ \t]+/
const EDGE_PIPES = /^\|+|\|+$/g

const isSpace = (c: string | undefined) => c === ' ' || c === '\t'
const isDigit = (c: string | undefined) => c !== undefined && c >= '0' && c <= '9'

/** The length of an ordered list marker ("12. ", "3) ") opening line, or 0. */
function orderedMarkerLength(line: string): number {
  let digits = 0
  while (digits <= MAX_ORDERED_DIGITS && isDigit(line[digits])) digits++
  if (digits === 0 || digits > MAX_ORDERED_DIGITS) return 0
  if (!ORDERED_MARK_ENDS.includes(line[digits] ?? '') || !isSpace(line[digits + 1])) return 0
  return digits + 2
}

/** The quote and list markers that open a line ("> - 1. text" is three levels), counted up to one past the limit. */
function nesting(line: string): number {
  let depth = 0
  let rest = line
  for (;;) {
    rest = rest.replace(LEADING_SPACE, '')
    if (rest === '' || depth > MAX_NESTING_DEPTH) return depth
    if (rest[0] === QUOTE_MARK) {
      rest = rest.slice(1)
    } else if (BULLET_MARKS.includes(rest.charAt(0)) && isSpace(rest[1])) {
      rest = rest.slice(2)
    } else {
      const n = orderedMarkerLength(rest)
      if (n === 0) return depth
      rest = rest.slice(n)
    }
    depth++
  }
}

/**
 * Whether a body has more table cells or deeper nesting than the limits allow. It reads the
 * source line by line, never building the tree it guards, and counts generously.
 */
export function isTooComplex(markdown: string): boolean {
  let cells = 0
  let columns = 0
  for (const raw of markdown.split('\n')) {
    const line = raw.trim()
    if (nesting(line) > MAX_NESTING_DEPTH) return true
    if (line === '') {
      columns = 0
    } else if (line.includes(TABLE_PIPE) && DELIMITER_ROW.test(line)) {
      columns = line.replace(EDGE_PIPES, '').split(TABLE_PIPE).length
      cells += columns
    } else if (columns > 0) {
      cells += columns
    }
    if (cells > MAX_TABLE_CELLS) return true
  }
  return false
}
