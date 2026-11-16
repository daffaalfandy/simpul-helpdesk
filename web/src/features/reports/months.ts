import { DATE_LOCALE } from '../../i18n/locale'

/** A report month as the API writes it: "2026-11". */
export type MonthKey = string

const MONTH_KEY = /^(\d{4})-(0[1-9]|1[0-2])$/
const MONTHS_PER_YEAR = 12
const KEY_YEAR_DIGITS = 4
const KEY_MONTH_DIGITS = 2

/** How many recent months get a button of their own (Reports mock); older ones are in a list. */
export const RECENT_MONTHS = 3

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
 * The month switcher: up to RECENT_MONTHS buttons ending at the current month, oldest first, and
 * every older month since the first ticket, newest first. Months before the first ticket are left
 * out; without tickets only the current month is offered.
 */
export function monthChoices(current: MonthKey, first: MonthKey | null): { recent: MonthKey[]; earlier: MonthKey[] } {
  const recent: MonthKey[] = [current]
  let m = current
  while (first !== null && recent.length < RECENT_MONTHS && m > first) {
    m = previousMonth(m)
    recent.unshift(m)
  }
  const earlier: MonthKey[] = []
  m = recent[0] ?? current
  while (first !== null && m > first) {
    m = previousMonth(m)
    earlier.push(m)
  }
  return { recent, earlier }
}

// Civil months: format them in UTC so the browser's time zone can't shift them.
const shortFormat = new Intl.DateTimeFormat(DATE_LOCALE, { month: 'short', timeZone: 'UTC' })
const longFormat = new Intl.DateTimeFormat(DATE_LOCALE, { month: 'long', year: 'numeric', timeZone: 'UTC' })

function firstDay(m: MonthKey): Date {
  const { year, month } = parts(m)
  return new Date(Date.UTC(year, month - 1, 1))
}

/** "Nov" */
export function monthShort(m: MonthKey): string {
  return shortFormat.format(firstDay(m))
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
