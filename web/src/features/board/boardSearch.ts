import type { TicketStatus } from '../../api/client'
import { ticketListQueryOptions, type TicketListFilters } from '../tickets/queries'
import { OPEN_STATUSES, TICKET_STATUSES } from '../tickets/status'

/** Board filters, kept in the URL. Defaults stay out of it. */
export type BoardView = 'all' | 'mine' | 'unassigned'
export type BoardStatus = 'open' | 'all' | TicketStatus
export type BoardSearch = { view?: BoardView; status?: BoardStatus; client?: string }

export const BOARD_VIEWS: readonly BoardView[] = ['all', 'mine', 'unassigned']
export const DEFAULT_VIEW: BoardView = 'all'
export const DEFAULT_STATUS: BoardStatus = 'open'
const STATUS_GROUPS = ['open', 'all'] as const

export function validateBoardSearch(search: Record<string, unknown>): BoardSearch {
  const out: BoardSearch = {}
  if (BOARD_VIEWS.includes(search.view as BoardView) && search.view !== DEFAULT_VIEW) out.view = search.view as BoardView
  const status = search.status as BoardStatus
  if ((STATUS_GROUPS.includes(status as (typeof STATUS_GROUPS)[number]) || TICKET_STATUSES.includes(status as TicketStatus)) && status !== DEFAULT_STATUS) {
    out.status = status
  }
  if (typeof search.client === 'string' && search.client !== '') out.client = search.client
  return out
}

/** The ticket list a board search shows. */
export function boardListOptions(search: BoardSearch) {
  const filters: TicketListFilters = {}
  const status = search.status ?? DEFAULT_STATUS
  if (status === 'open') filters.status = [...OPEN_STATUSES]
  else if (status !== 'all') filters.status = [status]
  if (search.view === 'mine') filters.assignee = 'me'
  if (search.view === 'unassigned') filters.assignee = 'unassigned'
  if (search.client) filters.clientId = search.client
  return ticketListQueryOptions(filters)
}
