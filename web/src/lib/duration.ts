import type { SlaCalendar } from '../api/client'
import { MINUTES_PER_DAY, MINUTES_PER_HOUR } from './constants'

/** Units an SLA target can be entered and shown in. */
export type DurationUnit = 'minutes' | 'hours' | 'workingDays' | 'days'

export type DurationParts = { value: number; unit: DurationUnit }

/** A "day" is a working day on business hours and a calendar day on 24/7. */
export function dayUnit(calendar: SlaCalendar): DurationUnit {
  return calendar === 'always' ? 'days' : 'workingDays'
}

export function unitsFor(calendar: SlaCalendar): DurationUnit[] {
  return ['minutes', 'hours', dayUnit(calendar)]
}

export function unitMinutes(unit: DurationUnit, workingDayMinutes: number): number {
  switch (unit) {
    case 'minutes':
      return 1
    case 'hours':
      return MINUTES_PER_HOUR
    case 'workingDays':
      return workingDayMinutes
    case 'days':
      return MINUTES_PER_DAY
  }
}

/** The largest unit that divides the minutes exactly: 1080 → 2 working days (of 540). */
export function toParts(minutes: number, calendar: SlaCalendar, workingDayMinutes: number): DurationParts {
  for (const unit of [...unitsFor(calendar)].reverse()) {
    const size = unitMinutes(unit, workingDayMinutes)
    if (size > 0 && minutes >= size && minutes % size === 0) return { value: minutes / size, unit }
  }
  return { value: minutes, unit: 'minutes' }
}

export function fromParts(parts: DurationParts, workingDayMinutes: number): number {
  return Math.round(parts.value * unitMinutes(parts.unit, workingDayMinutes))
}
