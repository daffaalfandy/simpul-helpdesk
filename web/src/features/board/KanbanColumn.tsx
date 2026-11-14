import { useId } from 'react'
import { GridList } from 'react-aria-components'
import { useTranslation } from 'react-i18next'

import type { BoardColumn } from '../../api/client'
import { KanbanCard } from './KanbanCard'

type KanbanColumnProps = {
  column: BoardColumn
  /** Filters or a search are on, so an empty column says so. */
  filtered: boolean
}

/** Kanban column per HANDOFF §14.3: title with a count, then the cards. */
export function KanbanColumn({ column, filtered }: KanbanColumnProps) {
  const { t } = useTranslation()
  const headingId = useId()
  const shown = column.cards.length
  const title = t(`board.columns.${column.id}`)
  return (
    <section
      aria-labelledby={headingId}
      className={
        'flex min-h-(--board-column-min-h) w-(--board-column-w) shrink-0 snap-start flex-col gap-2.5 rounded-2xl bg-tint p-3 ' +
        'xl:w-auto xl:min-w-0 xl:shrink'
      }
    >
      <h2 id={headingId} className="flex items-center justify-between gap-2 px-1.5 pt-1 text-base font-extrabold">
        <span>{title}</span>
        <span className="rounded-sm bg-surface px-2.25 py-0.5 text-xs tabular-nums">
          <span aria-hidden="true">{column.total}</span>
          <span className="sr-only">{t('board.columnCount', { count: column.total })}</span>
        </span>
      </h2>
      <GridList
        aria-label={t('board.columnLabel', { column: title, count: column.total })}
        items={column.cards}
        layout="stack"
        renderEmptyState={() => (
          <p className="m-1.5 text-sm text-muted">{filtered ? t('board.emptyFiltered') : t('board.empty')}</p>
        )}
        className="flex flex-1 flex-col gap-2.5 outline-none"
      >
        {(card) => <KanbanCard card={card} />}
      </GridList>
      {shown < column.total && (
        <p className="px-1.5 text-xs text-muted">{t('board.capped', { shown, total: column.total })}</p>
      )}
    </section>
  )
}
