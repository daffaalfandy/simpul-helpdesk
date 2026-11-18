import { endOfMonth, fromDate, parseDate, startOfMonth, toCalendarDate, type CalendarDate } from '@internationalized/date'

/**
 * The days a ticket was reported on, both included, as the API's `from` and `to`
 * ("2026-11-01"), read in the organization's time zone (owner, 18 Nov).
 */
export type DateRange = { from: string; to: string }

/** A range kept in the URL. The current month is the default and stays out of it. */
export type DateRangeSearch = { from?: string; to?: string }

/** Today's date in the time zone. */
export function todayIn(timeZone: string, now = new Date()): CalendarDate {
  return toCalendarDate(fromDate(now, timeZone))
}

function monthOf(day: CalendarDate): DateRange {
  return { from: startOfMonth(day).toString(), to: endOfMonth(day).toString() }
}

/** From the 1st to the last day of this month in the time zone. */
export function currentMonthRange(timeZone: string, now = new Date()): DateRange {
  return monthOf(todayIn(timeZone, now))
}

/** The whole month before this one in the time zone. */
export function lastMonthRange(timeZone: string, now = new Date()): DateRange {
  return monthOf(todayIn(timeZone, now).subtract({ months: 1 }))
}

/** The calendar date written as "2026-11-01", or null for anything else (including "2026-02-30"). */
export function parseDay(value: unknown): CalendarDate | null {
  if (typeof value !== 'string') return null
  try {
    const day = parseDate(value)
    return day.toString() === value ? day : null
  } catch {
    return null
  }
}

/** A range from the URL: both days, real dates, the last not before the first. Anything else is dropped. */
export function validateDateRangeSearch(search: Record<string, unknown>): DateRangeSearch {
  const from = parseDay(search.from)
  const to = parseDay(search.to)
  if (!from || !to || to.compare(from) < 0) return {}
  return { from: from.toString(), to: to.toString() }
}

export function isSameRange(a: DateRange, b: DateRange): boolean {
  return a.from === b.from && a.to === b.to
}

/** The range the URL asks for, or this month. */
export function rangeFromSearch(search: DateRangeSearch, timeZone: string, now = new Date()): DateRange {
  return search.from && search.to ? { from: search.from, to: search.to } : currentMonthRange(timeZone, now)
}

/** The range as the URL keeps it: nothing for this month. */
export function searchFromRange(range: DateRange, timeZone: string, now = new Date()): DateRangeSearch {
  return isSameRange(range, currentMonthRange(timeZone, now)) ? {} : { from: range.from, to: range.to }
}

/** The search with its range replaced and every other key kept. */
export function withRange<S extends DateRangeSearch>(search: S, range: DateRange, timeZone: string, now = new Date()): S {
  const next = { ...search }
  delete next.from
  delete next.to
  return { ...next, ...searchFromRange(range, timeZone, now) }
}
