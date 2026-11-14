import { queryOptions } from '@tanstack/react-query'

import { api, type TicketAssigneeFilter } from '../../api/client'
import { queryKeys } from '../../api/queries'
import { unwrap } from '../../api/unwrap'
import { MAX_SEARCH_LENGTH } from '../../lib/constants'

/** Board filters, kept in the URL. Defaults stay out of it. */
export type BoardView = 'all' | 'mine' | 'unassigned'
export type BoardSearch = { view?: BoardView; q?: string }

export const BOARD_VIEWS: readonly BoardView[] = ['all', 'mine', 'unassigned']
export const DEFAULT_VIEW: BoardView = 'all'

const VIEW_ASSIGNEE: Record<BoardView, TicketAssigneeFilter | undefined> = { all: undefined, mine: 'me', unassigned: 'unassigned' }

export function validateBoardSearch(search: Record<string, unknown>): BoardSearch {
  const out: BoardSearch = {}
  if (BOARD_VIEWS.includes(search.view as BoardView) && search.view !== DEFAULT_VIEW) out.view = search.view as BoardView
  const q = cleanQuery(search.q)
  if (q) out.q = q
  return out
}

/** The search box as the API takes it: trimmed, at most MAX_SEARCH_LENGTH characters. */
export function cleanQuery(value: unknown): string {
  return typeof value === 'string' ? value.trim().slice(0, MAX_SEARCH_LENGTH) : ''
}

export function isFiltered(search: BoardSearch): boolean {
  return Boolean(search.view || search.q)
}

export const boardQueryOptions = (search: BoardSearch) =>
  queryOptions({
    queryKey: queryKeys.board(search),
    queryFn: () =>
      unwrap(
        api.GET('/board', {
          params: { query: { assignee: VIEW_ASSIGNEE[search.view ?? DEFAULT_VIEW], q: search.q } },
        }),
      ),
  })
