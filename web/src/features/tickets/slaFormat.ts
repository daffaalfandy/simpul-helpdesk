import type { TFunction } from 'i18next'

import type { SlaClock, SlaClockName, SlaPhase } from '../../api/client'
import type { PillTone } from '../../components/Badges'
import { MINUTES_PER_HOUR, MS_PER_MINUTE } from '../../lib/constants'

const MS_PER_HOUR = MS_PER_MINUTE * MINUTES_PER_HOUR

/**
 * A span of SLA time, compact: "22m", "5h", "3h 20m", "1d 4h". A day is the clock's own day
 * (dayMs): one working day on business hours, 24 h around the clock.
 */
export function compactSpan(t: TFunction, ms: number, dayMs: number): string {
  const total = Math.max(0, Math.abs(ms))
  const parts: string[] = []
  const days = dayMs > 0 ? Math.floor(total / dayMs) : 0
  const rest = total - days * dayMs
  const hours = Math.floor(rest / MS_PER_HOUR)
  const minutes = Math.floor((rest - hours * MS_PER_HOUR) / MS_PER_MINUTE)
  if (days > 0) {
    parts.push(t('durationCompact.d', { count: days }))
    if (hours > 0) parts.push(t('durationCompact.h', { count: hours }))
  } else if (hours > 0) {
    parts.push(t('durationCompact.h', { count: hours }))
    if (minutes > 0) parts.push(t('durationCompact.min', { count: minutes }))
  } else {
    parts.push(t('durationCompact.min', { count: minutes }))
  }
  return parts.join(' ')
}

/** Chip colour per phase (HANDOFF §14.3 chip tokens); the text always says the state. */
export const SLA_TONE: Record<SlaPhase, PillTone> = {
  running: 'success',
  at_risk: 'warning',
  breached: 'danger',
  paused: 'paused',
  met: 'success',
  missed: 'danger',
  stopped: 'neutral',
}

type ClockNow = Pick<SlaClock, 'phase' | 'remainingMs' | 'dayMs'>

/** What a clock says in a chip: "Respond in 22m", "5h left", "Breached 12m", "Paused · 3h left". */
export function slaChipText(t: TFunction, clock: SlaClockName, c: ClockNow): string | null {
  const span = compactSpan(t, c.remainingMs, c.dayMs)
  switch (c.phase) {
    case 'running':
    case 'at_risk':
      return clock === 'first_response' ? t('sla.respondIn', { span }) : t('sla.left', { span })
    case 'breached':
      return t('sla.breached', { span })
    case 'paused':
      return t('sla.paused', { span })
    case 'met':
      return t('sla.met')
    case 'missed':
      return t('sla.missed')
    case 'stopped':
      return null
  }
}
