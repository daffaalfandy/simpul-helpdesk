import { useTranslation } from 'react-i18next'

import type { SlaReport } from '../../api/client'
import { cx } from '../../lib/text'
import { formatPercent } from './format'

/** The report's four figures (Reports mock): a label, the number, and what it is measured against. */
export function KpiTiles({ report }: { report: SlaReport }) {
  const { t } = useTranslation()
  const { totals, targetPercent, projectsBelowTarget } = report
  const target = formatPercent(targetPercent)
  const empty = totals.tickets === 0
  const tiles = [
    {
      id: 'tickets',
      label: t('reports.kpi.tickets'),
      value: String(totals.tickets),
      sub: empty ? t('reports.kpi.noTicketsSub') : t('reports.kpi.ticketsSub', { count: report.projectsWithTickets }),
    },
    {
      id: 'within',
      label: t('reports.kpi.within'),
      value:
        totals.withinSlaPercent == null
          ? t('reports.noTickets')
          : t('reports.percent', { value: formatPercent(totals.withinSlaPercent) }),
      sub: t('reports.kpi.withinSub', { target }),
      muted: totals.withinSlaPercent == null,
    },
    { id: 'breaches', label: t('reports.kpi.breaches'), value: String(totals.breaches), sub: t('reports.kpi.breachesSub') },
    {
      id: 'below',
      label: t('reports.kpi.belowTarget'),
      value: String(projectsBelowTarget),
      sub: empty
        ? t('reports.kpi.noTicketsSub')
        : projectsBelowTarget > 0
          ? t('reports.kpi.belowTargetSub')
          : t('reports.kpi.allOnTarget'),
    },
  ]
  return (
    <dl className="grid grid-cols-[repeat(auto-fit,minmax(var(--kpi-tile-min-w),1fr))] gap-3">
      {tiles.map((tile) => (
        <div key={tile.id} className="flex flex-col gap-1 rounded-2xl border border-border bg-surface px-4.5 py-4">
          <dt className="text-xs font-semibold text-muted">{tile.label}</dt>
          <dd className="flex flex-col gap-0.5">
            <span className={cx('font-extrabold tabular-nums', tile.muted ? 'text-lg text-muted' : 'text-3xl')}>
              {tile.value}
            </span>
            <span className="text-xs text-muted">{tile.sub}</span>
          </dd>
        </div>
      ))}
    </dl>
  )
}
