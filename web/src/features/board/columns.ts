import type { Board, BoardCard, BoardColumnId, Priority, TicketStatus } from '../../api/client'

/** The board's columns, left to right (the API returns them in this order too). */
export const BOARD_COLUMNS: readonly BoardColumnId[] = ['new', 'in_progress', 'waiting_on_client', 'in_uat', 'resolved']

const COLUMN_STATUS: Record<Exclude<BoardColumnId, 'new'>, TicketStatus> = {
  in_progress: 'in_progress',
  waiting_on_client: 'waiting_on_client',
  in_uat: 'in_uat',
  resolved: 'resolved',
}

/**
 * The status a card dropped into a column gets, as the API's ticket.DropStatus: New splits on
 * the owner, so an owned ticket waits as "assigned" and an unowned one as "new".
 */
export function dropStatus(column: BoardColumnId, card: Pick<BoardCard, 'assignee'>): TicketStatus {
  if (column === 'new') return card.assignee ? 'assigned' : 'new'
  return COLUMN_STATUS[column]
}

/** Whether the viewer may drop the card into the column; the API lists the allowed moves. */
export function canDrop(card: BoardCard, column: BoardColumnId): boolean {
  return card.allowedStatuses.includes(dropStatus(column, card))
}

const PRIORITY_ORDER: readonly Priority[] = ['P1', 'P2', 'P3', 'P4']

/**
 * The board right after a drop, before the API answers: the card leaves its column and joins
 * the target at the bottom of its priority group, as the API will order it. Its allowed moves
 * stay empty until the board refreshes.
 */
export function moveCard(board: Board, card: BoardCard, column: BoardColumnId, status: TicketStatus): Board {
  const rank = PRIORITY_ORDER.indexOf(card.priority)
  const moved: BoardCard = { ...card, status, allowedStatuses: [] }
  return {
    ...board,
    columns: board.columns.map((c) => {
      const without = c.cards.filter((x) => x.id !== card.id)
      const left = without.length < c.cards.length
      if (c.id !== column) return left ? { ...c, cards: without, total: c.total - 1 } : c
      const at = without.findIndex((x) => PRIORITY_ORDER.indexOf(x.priority) > rank)
      const cards = at === -1 ? [...without, moved] : [...without.slice(0, at), moved, ...without.slice(at)]
      return { ...c, cards, total: c.total + 1 - (left ? 1 : 0) }
    }),
  }
}
