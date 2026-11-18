import type { BoardColumnId, Ticket, TicketStatus } from '../api/client'

/**
 * The API's manual moves (api/internal/ticket/lifecycle.go), for the fake server only: the app
 * itself reads the allowed moves from the API and never decides them.
 */
const MOVES: Record<TicketStatus, TicketStatus[]> = {
  new: ['in_progress', 'waiting_on_client', 'resolved'],
  assigned: ['in_progress', 'waiting_on_client', 'resolved'],
  in_progress: ['new', 'assigned', 'waiting_on_client', 'in_uat', 'resolved'],
  waiting_on_client: ['new', 'assigned', 'in_progress', 'in_uat', 'resolved'],
  in_uat: ['in_progress', 'waiting_on_client', 'resolved'],
  resolved: ['in_progress'],
  closed: [],
}

export function allowedMoves(t: Pick<Ticket, 'status' | 'assignee'>): TicketStatus[] {
  return MOVES[t.status].filter((to) => !(to === 'new' && t.assignee) && !(to === 'assigned' && !t.assignee))
}

/** The board column a status shows in (api/internal/ticket/board.go): closed ones sit with the resolved. */
export function columnOf(status: TicketStatus): BoardColumnId {
  switch (status) {
    case 'new':
    case 'assigned':
      return 'new'
    case 'closed':
      return 'resolved'
    default:
      return status
  }
}
