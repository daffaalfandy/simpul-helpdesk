import { useTranslation } from 'react-i18next'

import type { Board } from '../../api/client'
import { KanbanColumn } from './KanbanColumn'

type KanbanBoardProps = {
  board: Board
  filtered: boolean
}

/**
 * The five columns. From xl they sit side by side as in the mock; below that they scroll
 * sideways in their own box, one column at a time, so the page never does.
 */
export function KanbanBoard({ board, filtered }: KanbanBoardProps) {
  const { t } = useTranslation()
  return (
    <div
      role="region"
      aria-label={t('board.label')}
      className="-mx-4 flex snap-x snap-mandatory items-start gap-3 overflow-x-auto px-4 pb-2 sm:-mx-7 sm:px-7 xl:mx-0 xl:grid xl:grid-cols-5 xl:overflow-visible xl:px-0 xl:pb-0"
    >
      {board.columns.map((column) => (
        <KanbanColumn key={column.id} column={column} filtered={filtered} />
      ))}
    </div>
  )
}
