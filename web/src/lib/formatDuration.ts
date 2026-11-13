import type { TFunction } from 'i18next'

import type { SlaCalendar } from '../api/client'
import { toParts } from './duration'

/** "2 working days", "15 min", "1 day". */
export function formatDuration(t: TFunction, minutes: number, calendar: SlaCalendar, workingDayMinutes: number): string {
  const { value, unit } = toParts(minutes, calendar, workingDayMinutes)
  return t(`duration.${unit}`, { count: value })
}
