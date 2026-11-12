import { DATE_LOCALE } from '../i18n/locale'

// Dates from the API are civil dates ("2026-03-01"); format them in UTC so the
// browser's time zone can't shift the day.
const UTC = 'UTC'

function parseISODate(iso: string): Date {
  return new Date(`${iso}T00:00:00Z`)
}

const monthFormat = new Intl.DateTimeFormat(DATE_LOCALE, { month: 'short', timeZone: UTC })
const monthYearFormat = new Intl.DateTimeFormat(DATE_LOCALE, { month: 'short', year: 'numeric', timeZone: UTC })
const longDateFormat = new Intl.DateTimeFormat(DATE_LOCALE, {
  weekday: 'short',
  day: 'numeric',
  month: 'short',
  year: 'numeric',
  timeZone: UTC,
})

/** "Mar 2026" */
export function formatMonthYear(iso: string): string {
  return monthYearFormat.format(parseISODate(iso))
}

/** "Mon, 1 Jun 2026" */
export function formatLongDate(iso: string): string {
  return longDateFormat.format(parseISODate(iso))
}

/** A contract period with both ends: "Jan–Dec 2026" or "Mar 2026–Feb 2027". */
export function formatPeriod(start: string, end: string): string {
  const a = parseISODate(start)
  const b = parseISODate(end)
  if (a.getUTCFullYear() === b.getUTCFullYear()) {
    return `${monthFormat.format(a)}–${monthYearFormat.format(b)}`
  }
  return `${monthYearFormat.format(a)}–${monthYearFormat.format(b)}`
}

/** Today's year in the browser. */
export function currentYear(now = new Date()): number {
  return now.getFullYear()
}
