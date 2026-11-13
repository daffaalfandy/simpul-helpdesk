import type { TFunction } from 'i18next'

import type { Duration, DurationUnit } from '../api/client'

const LONG_KEY = { min: 'minutes', h: 'hours', wd: 'workingDays', d: 'days' } as const satisfies Record<DurationUnit, string>

/** A project target in words: "15 min", "4 hours", "1 working day", "2 days". */
export function formatTarget(t: TFunction, d: Duration): string {
  return t(`duration.${LONG_KEY[d.unit]}`, { count: d.value })
}

/** A project target in the agent's SLA box: "15m", "4h", "1 wd", "2d". */
export function formatTargetCompact(t: TFunction, d: Duration): string {
  return t(`durationCompact.${d.unit}`, { count: d.value })
}

/** A project target for chips: "15 min", "2 h", "1 wd", "2 d". */
export function formatTargetShort(t: TFunction, d: Duration): string {
  return t(`durationShort.${d.unit}`, { count: d.value })
}
