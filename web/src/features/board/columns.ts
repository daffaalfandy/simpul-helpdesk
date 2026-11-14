import type { BoardCard, BoardColumnId, TicketStatus } from '../../api/client'

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
