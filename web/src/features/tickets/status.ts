import type { TicketStatus } from '../../api/client'
import type { PillTone } from '../../components/Badges'

/** Every status in lifecycle order (matches the API's TicketStatus). */
export const TICKET_STATUSES: readonly TicketStatus[] = [
  'new',
  'assigned',
  'in_progress',
  'waiting_on_client',
  'in_uat',
  'resolved',
  'closed',
]

/** Work is finished in these; everything else is open. */
export const DONE_STATUSES: readonly TicketStatus[] = ['resolved', 'closed']
export const OPEN_STATUSES: readonly TicketStatus[] = TICKET_STATUSES.filter((s) => !DONE_STATUSES.includes(s))

/** Pill colour per status (HANDOFF §14.3 chip tokens); the text always names the status. */
export const STATUS_TONE: Record<TicketStatus, PillTone> = {
  new: 'neutral',
  assigned: 'neutral',
  in_progress: 'warning',
  waiting_on_client: 'paused',
  in_uat: 'info',
  resolved: 'success',
  closed: 'neutral',
}

/** Statuses whose dropdown option explains what it does. */
export const STATUS_HINTS = ['waiting_on_client', 'in_uat', 'resolved'] as const satisfies readonly TicketStatus[]
export type HintedStatus = (typeof STATUS_HINTS)[number]

export function hasHint(status: TicketStatus): status is HintedStatus {
  return (STATUS_HINTS as readonly TicketStatus[]).includes(status)
}

export type PortalFilter = 'open' | 'resolved' | 'all'
export const PORTAL_FILTERS: readonly PortalFilter[] = ['open', 'resolved', 'all']
export const DEFAULT_PORTAL_FILTER: PortalFilter = 'open'

export function isPortalFilter(value: unknown): value is PortalFilter {
  return PORTAL_FILTERS.includes(value as PortalFilter)
}

export function statusesForPortalFilter(filter: PortalFilter): readonly TicketStatus[] {
  if (filter === 'open') return OPEN_STATUSES
  if (filter === 'resolved') return DONE_STATUSES
  return []
}
