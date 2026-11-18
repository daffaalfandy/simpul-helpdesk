import { describe, expect, it } from 'vitest'

import type { Board, BoardCard, BoardColumnId } from '../../api/client'
import { BOARD_COLUMNS, canDrop, dropStatus, moveCard } from './columns'

const ref = (id: string) => ({ id, name: id })

function card(key: string, over: Partial<BoardCard> = {}): BoardCard {
  return {
    id: key,
    key,
    type: 'incident',
    status: 'in_progress',
    priority: 'P3',
    subject: key,
    client: ref('client'),
    assignee: null,
    allowedStatuses: [],
    sla: null,
    dodPending: [],
    dodMandatory: true,
    uat: null,
    code: null,
    ...over,
  }
}

function board(cards: Partial<Record<BoardColumnId, BoardCard[]>>): Board {
  return {
    columns: BOARD_COLUMNS.map((id) => ({ id, total: cards[id]?.length ?? 0, cards: cards[id] ?? [] })),
    doneFrom: '2026-11-01',
    doneTo: '2026-11-30',
    stats: {
      openTickets: 0,
      resolvedThisWeek: 0,
      slaBreached: 0,
      slaAtRisk: 0,
      slaOldest: null,
      avgFirstResponseMs: null,
      withinSlaPercent: null,
    },
  }
}

const keys = (b: Board, id: BoardColumnId) => b.columns.find((c) => c.id === id)?.cards.map((c) => c.key)
const total = (b: Board, id: BoardColumnId) => b.columns.find((c) => c.id === id)?.total

describe('board columns', () => {
  it('turns a column into the status a dropped card gets', () => {
    expect(dropStatus('new', card('a'))).toBe('new')
    expect(dropStatus('new', card('a', { assignee: ref('rina') }))).toBe('assigned')
    expect(dropStatus('in_progress', card('a', { assignee: ref('rina') }))).toBe('in_progress')
    expect(dropStatus('waiting_on_client', card('a'))).toBe('waiting_on_client')
    expect(dropStatus('in_uat', card('a'))).toBe('in_uat')
    expect(dropStatus('resolved', card('a'))).toBe('resolved')
  })

  it('allows only the moves the API listed', () => {
    const fresh = card('a', { status: 'new', allowedStatuses: ['in_progress', 'waiting_on_client', 'resolved'] })
    expect(canDrop(fresh, 'in_progress')).toBe(true)
    expect(canDrop(fresh, 'in_uat')).toBe(false)
    expect(canDrop(fresh, 'new')).toBe(false)
    const owned = card('b', { assignee: ref('rina'), allowedStatuses: ['assigned', 'in_uat'] })
    expect(canDrop(owned, 'new')).toBe(true)
  })

  it('moves a card to the bottom of its priority group', () => {
    const moving = card('m', { priority: 'P2', status: 'new', allowedStatuses: ['in_progress'] })
    const before = board({
      new: [card('n1', { status: 'new' }), moving],
      in_progress: [card('p1', { priority: 'P1' }), card('p2', { priority: 'P2' }), card('p3')],
    })
    const after = moveCard(before, moving, 'in_progress', 'in_progress')
    expect(keys(after, 'new')).toEqual(['n1'])
    expect(keys(after, 'in_progress')).toEqual(['p1', 'p2', 'm', 'p3'])
    expect([total(after, 'new'), total(after, 'in_progress')]).toEqual([1, 4])
    const moved = after.columns[1]?.cards[2]
    expect(moved?.status).toBe('in_progress')
    expect(moved?.allowedStatuses).toEqual([])
  })
})
