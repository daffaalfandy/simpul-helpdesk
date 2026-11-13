import type { Duration, ProjectTarget, SlaCalendar, TicketType } from '../../api/client'

export type EffectiveTarget = { firstResponse: Duration; resolution: Duration; calendar: SlaCalendar }

/**
 * The targets a new ticket would get: resolve for incidents, deliver for service requests.
 * Previews the server's rule in api/internal/ticket/snapshot.go.
 */
export function targetFor(target: ProjectTarget, type: TicketType): EffectiveTarget {
  return {
    firstResponse: target.firstResponse,
    resolution: type === 'request' ? target.deliver : target.resolve,
    calendar: target.calendar,
  }
}
