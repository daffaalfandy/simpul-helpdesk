import type { TFunction } from 'i18next'

import type { Duration, DurationUnit, SlaCalendar } from '../api/client'
import { toParts } from './duration'

const LONG_KEY = { min: 'minutes', h: 'hours', wd: 'workingDays', d: 'days' } as const satisfies Record<DurationUnit, string>

/** A project target in words: "15 min", "4 hours", "1 working day", "2 days". */
export function formatTarget(t: TFunction, d: Duration): string {
  return t(`duration.${LONG_KEY[d.unit]}`, { count: d.value })
}

/** A project target for chips: "15 min", "2 h", "1 wd", "2 d". */
export function formatTargetShort(t: TFunction, d: Duration): string {
  return t(`durationShort.${d.unit}`, { count: d.value })
}

/** "2 working days", "15 min", "1 day". */
export function formatDuration(t: TFunction, minutes: number, calendar: SlaCalendar, workingDayMinutes: number): string {
  const { value, unit } = toParts(minutes, calendar, workingDayMinutes)
  return t(`duration.${unit}`, { count: value })
}
