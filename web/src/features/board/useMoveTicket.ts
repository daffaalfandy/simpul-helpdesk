import { useMutation, useQueryClient } from '@tanstack/react-query'
import { flushSync } from 'react-dom'

import { api, type Board, type BoardCard, type BoardColumnId, type TicketStatus } from '../../api/client'
import { queryKeys, useInvalidate } from '../../api/queries'
import { unwrap } from '../../api/unwrap'
import type { BoardSearch } from './boardSearch'
import { moveCard } from './columns'

export type Move = { card: BoardCard; column: BoardColumnId; status: TicketStatus }

const REDUCED_MOTION = '(prefers-reduced-motion: reduce)'

/**
 * Runs a DOM update inside a view transition, so a card glides to its new column (HANDOFF §14:
 * motion that shows what changed). Without browser support, or with reduced motion, it just
 * updates.
 */
function withTransition(update: () => void) {
  const reduced = typeof window.matchMedia === 'function' && window.matchMedia(REDUCED_MOTION).matches
  if (typeof document.startViewTransition !== 'function' || reduced) {
    update()
    return
  }
  document.startViewTransition(() => flushSync(update))
}

/**
 * Moves a ticket from the board. The card moves at once; the API checks the move with the
 * status the card showed (`from`), and a refused move puts the card back.
 */
export function useMoveTicket(search: BoardSearch) {
  const queryClient = useQueryClient()
  const invalidate = useInvalidate()
  const boardKey = queryKeys.board(search)
  return useMutation({
    mutationFn: ({ card, status }: Move) =>
      unwrap(
        api.PUT('/tickets/{ticketKey}/status', {
          params: { path: { ticketKey: card.key } },
          body: { status, from: card.status },
        }),
      ),
    onMutate: async ({ card, column, status }: Move) => {
      await queryClient.cancelQueries({ queryKey: queryKeys.boards })
      const previous = queryClient.getQueryData<Board>(boardKey)
      if (previous) withTransition(() => queryClient.setQueryData(boardKey, moveCard(previous, card, column, status)))
      return { previous }
    },
    onError: (_error, _move, context) => {
      const previous = context?.previous
      if (previous) withTransition(() => queryClient.setQueryData(boardKey, previous))
    },
    onSuccess: (ticket) => queryClient.setQueryData(queryKeys.ticket(ticket.key), ticket),
    onSettled: () => invalidate(queryKeys.tickets),
  })
}
