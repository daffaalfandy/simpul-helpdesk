import type { Weekday } from '../api/client'

/** Weekdays in display order (Monday first). */
export const WEEK: readonly Weekday[] = ['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun']

/** Splits days into runs of consecutive days in display order: [mon..fri] → [[mon, fri]]. */
export function weekdayRuns(days: readonly Weekday[]): Array<{ first: Weekday; last: Weekday }> {
  const runs: Array<{ first: Weekday; last: Weekday }> = []
  let current: { first: Weekday; last: Weekday } | null = null
  for (const day of WEEK) {
    if (days.includes(day)) {
      if (current) current.last = day
      else current = { first: day, last: day }
    } else if (current) {
      runs.push(current)
      current = null
    }
  }
  if (current) runs.push(current)
  return runs
}

export function closedDays(open: readonly Weekday[]): Weekday[] {
  return WEEK.filter((d) => !open.includes(d))
}
