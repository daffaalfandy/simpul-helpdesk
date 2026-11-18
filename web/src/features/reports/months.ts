import { DATE_LOCALE } from '../../i18n/locale'

/** A report month as the API writes it: "2026-11". */
export type MonthKey = string

const MONTH_KEY = /^(\d{4})-(0[1-9]|1[0-2])$/
const MONTHS_PER_YEAR = 12
const KEY_YEAR_DIGITS = 4
const KEY_MONTH_DIGITS = 2

export function isMonthKey(value: unknown): value is MonthKey {
  return typeof value === 'string' && MONTH_KEY.test(value)
}

function parts(key: MonthKey): { year: number; month: number } {
  const [, year, month] = MONTH_KEY.exec(key) ?? []
  return { year: Number(year), month: Number(month) }
}

function key(year: number, month: number): MonthKey {
  return `${String(year).padStart(KEY_YEAR_DIGITS, '0')}-${String(month).padStart(KEY_MONTH_DIGITS, '0')}`
}

/** The month before. */
export function previousMonth(m: MonthKey): MonthKey {
  const { year, month } = parts(m)
  return month === 1 ? key(year - 1, MONTHS_PER_YEAR) : key(year, month - 1)
}

/**
 * The months the report offers, newest first: the current month back to the later of the oldest
 * month the API serves and the first ticket's month. Without tickets only the current month.
 */
export function monthOptions(current: MonthKey, oldest: MonthKey, first: MonthKey | null): MonthKey[] {
  const out: MonthKey[] = [current]
  if (first === null) return out
  const floor = first > oldest ? first : oldest
  let m = current
  while (m > floor) {
    m = previousMonth(m)
    out.push(m)
  }
  return out
}

// Civil months: format them in UTC so the browser's time zone can't shift them.
const longFormat = new Intl.DateTimeFormat(DATE_LOCALE, { month: 'long', year: 'numeric', timeZone: 'UTC' })

function firstDay(m: MonthKey): Date {
  const { year, month } = parts(m)
  return new Date(Date.UTC(year, month - 1, 1))
}

/** "November 2026" */
export function monthLong(m: MonthKey): string {
  return longFormat.format(firstDay(m))
}

/** The report's months, both included: "2026-09" to "2026-11". */
export type MonthRange = { from: MonthKey; to: MonthKey }

/**
 * The months each end of the range offers, newest first (see monthOptions): the start no later
 * than the end and the end no earlier than the start, so the range can't be turned around. A
 * range opened from a link that starts before the first ticket keeps its start on the lists.
 */
export function rangeOptions(
  current: MonthKey,
  oldest: MonthKey,
  first: MonthKey | null,
  range: MonthRange,
): { from: MonthKey[]; to: MonthKey[] } {
  const reach = first === null || range.from < first ? range.from : first
  const months = monthOptions(current, oldest, reach)
  return { from: months.filter((m) => m <= range.to), to: months.filter((m) => m >= range.from) }
}

/**
 * The report's URL search: its first and last month. The API takes a missing end as the current
 * month, so the current month alone leaves both out.
 */
export type ReportSearch = { from?: MonthKey; to?: MonthKey }

export function validateReportSearch(search: Record<string, unknown>): ReportSearch {
  const out: ReportSearch = {}
  if (isMonthKey(search.from)) out.from = search.from
  if (isMonthKey(search.to)) out.to = search.to
  return out
}

/** The URL search for a range; nothing for the current month alone, the default. */
export function searchFromRange(range: MonthRange, current: MonthKey): ReportSearch {
  return range.from === current && range.to === current ? {} : { from: range.from, to: range.to }
}
