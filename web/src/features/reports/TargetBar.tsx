import { TriangleAlert } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { cx } from '../../lib/text'
import { formatPercent } from './format'

const FULL = 100

/**
 * Share of tickets within SLA against the contract target (Reports mock): green at or above the
 * target, blue below, with the target marked on the bar. Below target also says so in words and
 * with an icon, never by colour alone. The bar slides when the month changes.
 */
export function TargetBar({ percent, target }: { percent: number; target: number }) {
  const { t } = useTranslation()
  const onTarget = percent >= target
  const value = formatPercent(percent)
  return (
    <div className="flex items-center gap-2.5">
      <div
        role="meter"
        aria-label={t('reports.table.within')}
        aria-valuemin={0}
        aria-valuemax={FULL}
        aria-valuenow={percent}
        aria-valuetext={t('reports.withinValue', { value, target: formatPercent(target) })}
        className="relative h-(--target-bar-h) min-w-0 flex-1 rounded-full bg-tint"
      >
        <div
          className={cx(
            'h-full rounded-full transition-[width] duration-(--duration-base) ease-standard motion-reduce:transition-none',
            onTarget ? 'bg-success' : 'bg-primary',
          )}
          style={{ width: `${Math.min(percent, FULL)}%` }}
        />
        <div
          aria-hidden="true"
          className="absolute top-1/2 h-(--target-tick-h) w-(--target-tick-w) -translate-x-1/2 -translate-y-1/2 bg-ink"
          style={{ left: `${target}%` }}
        />
      </div>
      <span
        className={cx(
          'inline-flex min-w-16 items-center justify-end gap-1 font-extrabold tabular-nums',
          onTarget ? 'text-chip-success-fg' : 'text-chip-danger-fg',
        )}
      >
        {!onTarget && <TriangleAlert aria-hidden="true" className="size-(--icon-sm)" />}
        {t('reports.percent', { value })}
        {!onTarget && <span className="sr-only">{t('reports.belowTarget')}</span>}
      </span>
    </div>
  )
}
