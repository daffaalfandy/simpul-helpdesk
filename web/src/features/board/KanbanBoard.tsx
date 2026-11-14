import { useRef } from 'react'
import { useTranslation } from 'react-i18next'

import type { Board, BoardCard, BoardColumnId } from '../../api/client'
import { KanbanColumn } from './KanbanColumn'

type KanbanBoardProps = {
  board: Board
  filtered: boolean
  dragging: BoardCard | null
  onDraggingChange: (card: BoardCard | null) => void
  onMove: (card: BoardCard, column: BoardColumnId) => void
}

/**
 * The five columns. From xl they sit side by side as in the mock; below that they scroll
 * sideways in their own box, one column at a time, so the page never does.
 */
export function KanbanBoard({ board, filtered, dragging, onDraggingChange, onMove }: KanbanBoardProps) {
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
          filtered={filtered}
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
