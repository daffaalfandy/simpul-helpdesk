import type { SlaTarget, TicketType } from '../../api/client'

export type EffectiveTarget = { firstResponseMinutes: number; resolutionMinutes: number; calendar: SlaTarget['calendar'] }

/**
 * The target a new ticket would get (resolve time for incidents, delivery time for requests),
 * previewing the server's rule in api/internal/ticket/snapshot.go.
 */
export function targetFor(target: SlaTarget, type: TicketType, catalogMinutes?: number): EffectiveTarget {
  let resolutionMinutes = target.resolveMinutes
  if (type === 'request') {
    resolutionMinutes = target.deliverFromCatalog && catalogMinutes !== undefined ? catalogMinutes : target.deliverMinutes
  }
  return { firstResponseMinutes: target.firstResponseMinutes, resolutionMinutes, calendar: target.calendar }
}
