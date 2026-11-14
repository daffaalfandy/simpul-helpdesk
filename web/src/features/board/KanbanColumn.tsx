import { useId, type RefObject } from 'react'
import { GridList, useDragAndDrop } from 'react-aria-components'
import { useTranslation } from 'react-i18next'

import type { BoardCard, BoardColumn, BoardColumnId } from '../../api/client'
import { cx } from '../../lib/text'
import { canDrop } from './columns'
import { KanbanCard } from './KanbanCard'

/** Drag data type for a board card; other drags (files, text) are refused. */
export const TICKET_DRAG_TYPE = 'application/x-simdesk-ticket'

type KanbanColumnProps = {
  column: BoardColumn
  /** Filters or a search are on, so an empty column says so. */
  filtered: boolean
  /** The card being dragged anywhere on the board, if any (drives what the columns show). */
  dragging: BoardCard | null
  /**
   * The same card, set the moment a drag begins: React Aria asks which columns take it before
   * the board re-renders with `dragging`.
   */
  draggingRef: RefObject<BoardCard | null>
  onDragStart: (card: BoardCard) => void
  onDragEnd: () => void
  onMove: (card: BoardCard, column: BoardColumnId) => void
}

/**
 * Kanban column per HANDOFF §14.3: title with a count, then the cards. A card can be dropped
 * anywhere in a column the API allows; the others say so while the drag lasts.
 */
export function KanbanColumn({ column, filtered, dragging, draggingRef, onDragStart, onDragEnd, onMove }: KanbanColumnProps) {
  const { t } = useTranslation()
  const headingId = useId()
  const shown = column.cards.length
  const title = t(`board.columns.${column.id}`)
  const takes = (card: BoardCard | null): card is BoardCard =>
    card !== null && !column.cards.some((c) => c.id === card.id) && canDrop(card, column.id)
  const isSource = dragging !== null && column.cards.some((c) => c.id === dragging.id)
  const blocked = dragging !== null && !isSource && !takes(dragging)

  const drop = () => {
    const card = draggingRef.current
    if (takes(card)) onMove(card, column.id)
  }
  const { dragAndDropHooks } = useDragAndDrop<BoardCard>({
    getItems: (keys) => {
      draggingRef.current = column.cards.find((c) => keys.has(c.key)) ?? null
      return [...keys].map((key) => ({ [TICKET_DRAG_TYPE]: String(key), 'text/plain': String(key) }))
    },
    getAllowedDropOperations: () => ['move'],
    acceptedDragTypes: [TICKET_DRAG_TYPE],
    getDropOperation: () => (takes(draggingRef.current) ? 'move' : 'cancel'),
    onDragStart: (e) => {
      const card = column.cards.find((c) => e.keys.has(c.key))
      if (card) onDragStart(card)
    },
    onDragEnd,
    onRootDrop: drop,
    onItemDrop: drop,
  })

  return (
    <section
      aria-labelledby={headingId}
      className={cx(
        'flex min-h-(--board-column-min-h) w-(--board-column-w) shrink-0 snap-start flex-col gap-2.5 rounded-2xl bg-tint p-3',
        'outline-2 -outline-offset-2 outline-transparent transition-colors duration-(--duration-fast) ease-standard',
        'has-[[data-drop-target]]:bg-primary-tint-2 has-[[data-drop-target]]:outline-primary has-[[data-drop-target]]:outline-dashed',
        'xl:w-auto xl:min-w-0 xl:shrink',
      )}
    >
      <h2 id={headingId} className="flex items-center justify-between gap-2 px-1.5 pt-1 text-base font-extrabold">
        <span>{title}</span>
        <span className="rounded-sm bg-surface px-2.25 py-0.5 text-xs tabular-nums">
          <span aria-hidden="true">{column.total}</span>
          <span className="sr-only">{t('board.columnCount', { count: column.total })}</span>
        </span>
      </h2>
      {blocked && <p className="px-1.5 text-xs font-bold text-muted">{t('board.cannotDrop')}</p>}
      <GridList
        aria-label={t('board.columnLabel', { column: title, count: column.total })}
        items={column.cards}
        layout="stack"
        dragAndDropHooks={dragAndDropHooks}
        renderEmptyState={() => (
          <p className="m-1.5 text-sm text-muted">{filtered ? t('board.emptyFiltered') : t('board.empty')}</p>
        )}
        className={cx(
          'flex flex-1 flex-col gap-2.5 rounded-lg outline-none transition-opacity duration-(--duration-fast) ease-standard',
          blocked && 'opacity-50',
        )}
      >
        {(card) => <KanbanCard card={card} />}
      </GridList>
      {shown < column.total && (
        <p className="px-1.5 text-xs text-muted">{t('board.capped', { shown, total: column.total })}</p>
      )}
    </section>
  )
}
