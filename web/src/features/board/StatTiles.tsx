import { useTranslation } from 'react-i18next'

import type { BoardStats } from '../../api/client'

/**
 * The board's stat tiles (Main mock). Only the numbers the app can count today: the SLA tiles
 * arrive with the SLA engine (M5b) and reports (M8).
 */
export function StatTiles({ stats }: { stats: BoardStats }) {
  const { t } = useTranslation()
  const tiles = [
    { id: 'open', label: t('board.stats.openTickets'), value: stats.openTickets },
    { id: 'resolved', label: t('board.stats.resolvedThisWeek'), value: stats.resolvedThisWeek },
  ]
  return (
    <dl className="grid grid-cols-2 gap-3 sm:flex sm:flex-wrap">
      {tiles.map((tile) => (
        <div
          key={tile.id}
          className="flex flex-col gap-1 rounded-xl border border-border bg-surface px-4 py-3.5 sm:min-w-(--stat-tile-min-w)"
        >
          <dt className="text-xs font-semibold text-muted">{tile.label}</dt>
          <dd className="text-2xl font-extrabold tabular-nums">{tile.value}</dd>
        </div>
      ))}
    </dl>
  )
}
