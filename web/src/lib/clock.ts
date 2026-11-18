import { MINUTES_PER_DAY, MINUTES_PER_HOUR } from './constants'

/** Business hours are picked from a list of times this many minutes apart. */
export const BUSINESS_HOURS_STEP_MINUTES = 30

/** How the API writes the end of the day: a window may close at "24:00" (api/internal/calendar ParseClock). */
export const END_OF_DAY = '24:00'

/** Which end of a time window a time is for: a window can't start at 24:00 or end at 00:00. */
export type ClockEdge = 'start' | 'end'

/** "HH:MM", the API's clock format. */
const CLOCK_PATTERN = /^(\d{2}):(\d{2})$/
const CLOCK_DIGITS = 2

/** Minutes from midnight of "HH:MM" (00:00 to 24:00), or null when the text isn't such a time. */
export function parseClock(text: string): number | null {
  const match = CLOCK_PATTERN.exec(text)
  if (!match) return null
  const minutes = Number(match[1]) * MINUTES_PER_HOUR + Number(match[2])
  if (Number(match[2]) >= MINUTES_PER_HOUR || minutes > MINUTES_PER_DAY) return null
  return minutes
}

/** "HH:MM" for minutes from midnight; a full day is "24:00". */
export function clockText(minutes: number): string {
  const pad = (n: number) => String(n).padStart(CLOCK_DIGITS, '0')
  return `${pad(Math.floor(minutes / MINUTES_PER_HOUR))}:${pad(minutes % MINUTES_PER_HOUR)}`
}

/**
 * The times offered for one end of a window, every `step` minutes: 00:00 to 23:30 for a start,
 * 00:30 to 24:00 for an end (with a 30-minute step). A current time off the step (say 08:15, set
 * before the list existed) is kept in order, so opening the list never changes a saved value.
 */
export function timeOptions(edge: ClockEdge, current?: string | null, step = BUSINESS_HOURS_STEP_MINUTES): string[] {
  const first = edge === 'start' ? 0 : step
  const last = edge === 'start' ? MINUTES_PER_DAY - step : MINUTES_PER_DAY
  const minutes: number[] = []
  for (let m = first; m <= last; m += step) minutes.push(m)
  const kept = current == null ? null : parseClock(current)
  if (kept !== null && !minutes.includes(kept)) {
    minutes.push(kept)
    minutes.sort((a, b) => a - b)
  }
  return minutes.map(clockText)
}
