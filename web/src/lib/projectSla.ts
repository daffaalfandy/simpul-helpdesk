import type { Duration, DurationUnit, ProjectTarget } from '../api/client'
import { MINUTES_PER_DAY, MINUTES_PER_HOUR } from './constants'

/** Units a project target can be written in, smallest first (matches the API's DurationUnit). */
export const DURATION_UNITS: readonly DurationUnit[] = ['min', 'h', 'wd', 'd']

/** The smallest value a target can have. */
export const MIN_TARGET_VALUE = 1

/** The target fields of one priority, in the order the SLA card shows them. */
export const TARGET_FIELDS = ['firstResponse', 'resolve', 'deliver'] as const
export type TargetField = (typeof TARGET_FIELDS)[number]

/**
 * Minutes for comparing targets: a working day is one business-hours window, a day is 24 hours.
 * Mirrors api/internal/sla Duration.Minutes.
 */
export function durationMinutes(d: Duration, workingDayMinutes: number): number {
  switch (d.unit) {
    case 'h':
      return d.value * MINUTES_PER_HOUR
    case 'wd':
      return d.value * workingDayMinutes
    case 'd':
      return d.value * MINUTES_PER_DAY
    default:
      return d.value
  }
}

/**
 * Which rule a target breaks: first response must be shorter than resolve, then deliver
 * (follow-up #1). Null when the order is fine. Mirrors api/internal/sla ValidateProjectSLA.
 */
export function orderError(target: ProjectTarget, workingDayMinutes: number): 'resolve' | 'deliver' | null {
  const first = durationMinutes(target.firstResponse, workingDayMinutes)
  if (first >= durationMinutes(target.resolve, workingDayMinutes)) return 'resolve'
  if (first >= durationMinutes(target.deliver, workingDayMinutes)) return 'deliver'
  return null
}

export function validValue(d: Duration): boolean {
  return Number.isInteger(d.value) && d.value >= MIN_TARGET_VALUE
}
