import { queryOptions } from '@tanstack/react-query'

import { api, type BoardColumnId, type BoardSlaFilter, type TicketAssigneeFilter } from '../../api/client'
import { queryKeys } from '../../api/queries'
import { unwrap } from '../../api/unwrap'
import { MAX_SEARCH_LENGTH } from '../../lib/constants'
import { validateDateRangeSearch, type DateRange, type DateRangeSearch } from '../../lib/dateRange'

/**
 * Board filters, kept in the URL. Defaults stay out of it, the current month included: `from`
 * and `to` are there only for other days.
 */
export type BoardView = 'all' | 'mine' | 'risk' | 'unassigned'
export type BoardSearch = { view?: BoardView; q?: string } & DateRangeSearch
/** What the board asks the API for: the URL's filters with the range worked out. */
export type BoardFilters = BoardSearch & DateRange

/** The column the date range limits; open tickets show whenever they were reported (owner, 18 Nov). */
export const DONE_COLUMN: BoardColumnId = 'resolved'

export const BOARD_VIEWS: readonly BoardView[] = ['all', 'mine', 'risk', 'unassigned']
export const DEFAULT_VIEW: BoardView = 'all'
/** The view the SLA banner's "Show me" opens. */
export const RISK_VIEW: BoardView = 'risk'

const VIEW_ASSIGNEE: Record<BoardView, TicketAssigneeFilter | undefined> = {
  all: undefined,
  mine: 'me',
  risk: undefined,
  unassigned: 'unassigned',
}
const VIEW_SLA: Record<BoardView, BoardSlaFilter | undefined> = {
  all: undefined,
  mine: undefined,
  risk: 'at_risk',
  unassigned: undefined,
}

export function validateBoardSearch(search: Record<string, unknown>): BoardSearch {
  const out: BoardSearch = {}
  if (BOARD_VIEWS.includes(search.view as BoardView) && search.view !== DEFAULT_VIEW) out.view = search.view as BoardView
  const q = cleanQuery(search.q)
  if (q) out.q = q
  return { ...out, ...validateDateRangeSearch(search) }
}

/** The search box as the API takes it: trimmed, at most MAX_SEARCH_LENGTH characters. */
export function cleanQuery(value: unknown): string {
  return typeof value === 'string' ? value.trim().slice(0, MAX_SEARCH_LENGTH) : ''
}

/** Whether a filter narrows the column, so an empty one says so. Other days narrow only the done column. */
export function isFiltered(search: BoardSearch, column: BoardColumnId): boolean {
  return Boolean(search.view || search.q || (column === DONE_COLUMN && search.from))
}

export const boardQueryOptions = (filters: BoardFilters) =>
  queryOptions({
    queryKey: queryKeys.board(filters),
    queryFn: () =>
      unwrap(
        api.GET('/board', {
          params: {
            query: {
              assignee: VIEW_ASSIGNEE[filters.view ?? DEFAULT_VIEW],
              sla: VIEW_SLA[filters.view ?? DEFAULT_VIEW],
              q: filters.q,
              from: filters.from,
              to: filters.to,
            },
          },
        }),
      ),
  })
