import { fromDate, isSameDay, isSameYear } from '@internationalized/date'

import { DATE_LOCALE } from '../i18n/locale'
import { JUST_NOW_MS } from './constants'

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

// Timestamps from the API are instants; show them in the organisation's time zone so
// agents and requesters see the same clock (HANDOFF §6: WIB by default).

type ZoneFormats = {
  clock: Intl.DateTimeFormat
  dayMonth: Intl.DateTimeFormat
  dayMonthYear: Intl.DateTimeFormat
  today: Intl.DateTimeFormat
}

const zoneFormats = new Map<string, ZoneFormats>()

function formatsFor(timeZone: string): ZoneFormats {
  let f = zoneFormats.get(timeZone)
  if (!f) {
    f = {
      clock: new Intl.DateTimeFormat(DATE_LOCALE, { hour: '2-digit', minute: '2-digit', hourCycle: 'h23', timeZone }),
      dayMonth: new Intl.DateTimeFormat(DATE_LOCALE, { day: 'numeric', month: 'short', timeZone }),
      dayMonthYear: new Intl.DateTimeFormat(DATE_LOCALE, { day: 'numeric', month: 'short', year: 'numeric', timeZone }),
      today: new Intl.DateTimeFormat(DATE_LOCALE, { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric', timeZone }),
    }
    zoneFormats.set(timeZone, f)
  }
  return f
}

/** "08:14" */
export function formatClock(iso: string, timeZone: string): string {
  return formatsFor(timeZone).clock.format(new Date(iso))
}

/** "12 Nov" */
export function formatDayMonth(iso: string, timeZone: string): string {
  return formatsFor(timeZone).dayMonth.format(new Date(iso))
}

/** "12 Nov 2026" */
export function formatDayMonthYear(iso: string, timeZone: string): string {
  return formatsFor(timeZone).dayMonthYear.format(new Date(iso))
}

/** "Thursday, 12 November 2026" in the organisation's time zone. */
export function formatToday(timeZone: string, now = new Date()): string {
  return formatsFor(timeZone).today.format(now)
}

/** How a timestamp should be written: "Just now", a time today, a date this year, or a full date. */
export type StampKind = 'justNow' | 'today' | 'thisYear' | 'older'

export function stampKind(iso: string, timeZone: string, now = new Date()): StampKind {
  const at = new Date(iso)
  const ageMs = now.getTime() - at.getTime()
  if (ageMs >= 0 && ageMs < JUST_NOW_MS) return 'justNow'
  const a = fromDate(at, timeZone)
  const b = fromDate(now, timeZone)
  if (isSameDay(a, b)) return 'today'
  if (isSameYear(a, b)) return 'thisYear'
  return 'older'
}

/** Indonesian zone names; Intl only knows "GMT+7". */
const ZONE_ABBREVIATIONS: Record<string, string> = {
  'Asia/Jakarta': 'WIB',
  'Asia/Pontianak': 'WIB',
  'Asia/Makassar': 'WITA',
  'Asia/Jayapura': 'WIT',
}

/** "WIB" for Asia/Jakarta, otherwise Intl's short name. */
export function zoneAbbreviation(timeZone: string): string {
  const known = ZONE_ABBREVIATIONS[timeZone]
  if (known) return known
  const part = new Intl.DateTimeFormat(DATE_LOCALE, { timeZone, timeZoneName: 'short' })
    .formatToParts(new Date())
    .find((p) => p.type === 'timeZoneName')
  return part?.value ?? timeZone
}
