import type { TFunction } from 'i18next'
import { useRef } from 'react'
import { useTranslation } from 'react-i18next'

import type { Board, BoardCard, BoardColumnId } from '../../api/client'
import { formatDateRange } from '../../lib/dates'
import { KanbanColumn } from './KanbanColumn'

type KanbanBoardProps = {
  board: Board
  /** Whether filters or a search narrow the column, so an empty one says so. */
  isFiltered: (column: BoardColumnId) => boolean
  dragging: BoardCard | null
  onDraggingChange: (card: BoardCard | null) => void
  onMove: (card: BoardCard, column: BoardColumnId) => void
}

/**
 * The five columns. From xl they sit side by side as in the mock; below that they scroll
 * sideways in their own box, one column at a time, so the page never does.
 */
export function KanbanBoard({ board, isFiltered, dragging, onDraggingChange, onMove }: KanbanBoardProps) {
  const { t } = useTranslation()
  const draggingRef = useRef<BoardCard | null>(null)
  return (
    <div
      role="region"
      aria-label={t('board.label')}
      className="-mx-4 flex snap-x snap-mandatory items-start gap-3 overflow-x-auto px-4 pb-2 sm:-mx-7 sm:px-7 xl:mx-0 xl:grid xl:grid-cols-5 xl:overflow-visible xl:px-0 xl:pb-0"
    >
      {board.columns.map((column) => (
        <KanbanColumn
          key={column.id}
          column={column}
          note={columnNote(t, board, column.id)}
          filtered={isFiltered(column.id)}
          dragging={dragging}
          draggingRef={draggingRef}
          onDragStart={onDraggingChange}
          onDragEnd={() => {
            draggingRef.current = null
            onDraggingChange(null)
          }}
          onMove={onMove}
        />
      ))}
    </div>
  )
}

/** The line under a column's title (Main mock), where there is something to say. */
function columnNote(t: TFunction, board: Board, column: BoardColumnId): string | undefined {
  if (column === 'in_uat') return t('agent.board.columnNote.in_uat')
  if (column === 'resolved') return t('agent.board.columnNote.resolved', { range: formatDateRange(board.doneFrom, board.doneTo) })
  return undefined
}
