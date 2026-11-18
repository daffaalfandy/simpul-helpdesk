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

/** The report's URL search: the month, left out for the current one. */
export type ReportSearch = { month?: MonthKey }

export function validateReportSearch(search: Record<string, unknown>): ReportSearch {
  return isMonthKey(search.month) ? { month: search.month } : {}
}
