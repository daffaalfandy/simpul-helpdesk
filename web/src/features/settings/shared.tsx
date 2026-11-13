import type { TFunction } from 'i18next'
import { Hammer } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import type { SlaPlan } from '../../api/client'
import { EmptyState } from '../../components/EmptyState'
import type { SectionId } from './sections'

/** Section title and description at the top of a settings page. */
export function SectionHeader({ section, actions }: { section: SectionId; actions?: ReactNode }) {
  const { t } = useTranslation()
  return (
    <div className="flex flex-wrap items-end justify-between gap-4">
      <div className="flex flex-col gap-1">
        <h2 className="text-2xl font-extrabold tracking-title">{t(`settings.sections.${section}.title`)}</h2>
        <p className="max-w-3xl text-base text-muted">{t(`settings.sections.${section}.description`)}</p>
      </div>
      {actions}
    </div>
  )
}

export function ComingSoonSection({ section }: { section: SectionId }) {
  const { t } = useTranslation()
  return (
    <>
      <SectionHeader section={section} />
      <EmptyState icon={Hammer} title={t('common.comingSoonTitle')}>
        {t('common.comingSoon')}
      </EmptyState>
    </>
  )
}

export { errorMessage } from '../../api/errors'
export { ErrorAlert } from '../../components/ErrorAlert'
export { formatDuration } from '../../lib/formatDuration'

/** Dropdown sub-line for a plan: which priorities run 24/7. */
export function planSummary(t: TFunction, plan: SlaPlan): string {
  const always = plan.targets.filter((x) => x.calendar === 'always').map((x) => x.priority)
  return always.length > 0 ? t('settings.sla.summaryAlways', { priorities: always.join(', ') }) : t('settings.sla.summaryBusiness')
}
