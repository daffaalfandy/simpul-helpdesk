import { useTranslation } from 'react-i18next'

import type { BoardStats } from '../../api/client'
import { DATE_LOCALE } from '../../i18n/locale'
import { HOURS_PER_DAY, MINUTES_PER_HOUR, MS_PER_MINUTE } from '../../lib/constants'
import { cx } from '../../lib/text'
import { compactSpan } from '../tickets/slaFormat'

const MS_PER_DAY = HOURS_PER_DAY * MINUTES_PER_HOUR * MS_PER_MINUTE
const percent = new Intl.NumberFormat(DATE_LOCALE, { maximumFractionDigits: 1 })

/**
 * The board's stat tiles (Main mock). The mock's "target 30m" under the average is left out:
 * targets differ by project and priority.
 */
export function StatTiles({ stats }: { stats: BoardStats }) {
  const { t } = useTranslation()
  const tiles = [
    { id: 'open', label: t('board.stats.openTickets'), value: String(stats.openTickets), empty: false },
    {
      id: 'first-response',
      label: t('board.stats.avgFirstResponse'),
      value:
        stats.avgFirstResponseMs == null
          ? t('board.stats.noFirstReplies')
          : compactSpan(t, stats.avgFirstResponseMs, MS_PER_DAY),
      empty: stats.avgFirstResponseMs == null,
    },
    { id: 'resolved', label: t('board.stats.resolvedThisWeek'), value: String(stats.resolvedThisWeek), empty: false },
    {
      id: 'within',
      label: t('board.stats.withinSla'),
      value:
        stats.withinSlaPercent == null
          ? t('board.stats.noResolved')
          : t('board.stats.percent', { value: percent.format(stats.withinSlaPercent) }),
      empty: stats.withinSlaPercent == null,
    },
  ]
  return (
    <dl className="grid grid-cols-2 gap-3 sm:flex sm:flex-wrap">
      {tiles.map((tile) => (
        <div
          key={tile.id}
          className="flex flex-col gap-1 rounded-xl border border-border bg-surface px-4 py-3.5 sm:min-w-(--stat-tile-min-w)"
        >
          <dt className="text-xs font-semibold text-muted">{tile.label}</dt>
          <dd className={cx('font-extrabold tabular-nums', tile.empty ? 'text-base text-muted' : 'text-2xl')}>
            {tile.value}
          </dd>
        </div>
      ))}
    </dl>
  )
}
