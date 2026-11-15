import { useTranslation } from 'react-i18next'

import type { BoardStats } from '../../api/client'
import { Button } from '../../components/Button'

/**
 * The board's SLA alert (Main mock): how many open tickets are breached or at risk, the one
 * that fell due first, and a way to show only them. Hidden when there are none.
 */
export function SlaBanner({ stats, onShow }: { stats: BoardStats; onShow?: () => void }) {
  const { t } = useTranslation()
  const count = stats.slaBreached + stats.slaAtRisk
  if (count === 0) return null
  return (
    <section
      aria-label={t('sla.banner.label')}
      className="flex flex-wrap items-center gap-x-5 gap-y-3 rounded-2xl bg-ink px-5 py-4 text-on-primary"
    >
      <span className="text-3xl font-extrabold text-on-dark-accent tabular-nums">{count}</span>
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <p className="text-md font-bold">{t('sla.banner.title', { count })}</p>
        <p className="text-sm text-on-dark-muted">
          {t('sla.banner.detail', { breached: stats.slaBreached, atRisk: stats.slaAtRisk })}
          {stats.slaOldest && ` ${t('sla.banner.oldest', { key: stats.slaOldest.key, client: stats.slaOldest.clientName })}`}
        </p>
      </div>
      {onShow && (
        <Button variant="outline" onPress={onShow}>
          {t('sla.banner.show')}
        </Button>
      )}
    </section>
  )
}
