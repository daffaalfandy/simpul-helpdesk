import type { TFunction } from 'i18next'
import { Hammer } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { ApiError, type SlaCalendar, type SlaPlan } from '../../api/client'
import { Alert } from '../../components/Alert'
import { EmptyState } from '../../components/EmptyState'
import en from '../../i18n/en.json'
import { toParts } from '../../lib/duration'
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

type ErrorCodeKey = keyof typeof en.errors

function isErrorCode(code: string): code is ErrorCodeKey {
  return Object.hasOwn(en.errors, code)
}

/** A translated message for an API error, or the generic one. */
export function errorMessage(t: TFunction, error: unknown): string {
  if (error instanceof ApiError && isErrorCode(error.code)) return t(`errors.${error.code}`)
  return t('common.errorGeneric')
}

export function ErrorAlert({ error }: { error: unknown }) {
  const { t } = useTranslation()
  if (!error) return null
  return <Alert tone="error">{errorMessage(t, error)}</Alert>
}

/** "2 working days", "15 min", "1 day". */
export function formatDuration(t: TFunction, minutes: number, calendar: SlaCalendar, workingDayMinutes: number): string {
  const { value, unit } = toParts(minutes, calendar, workingDayMinutes)
  return t(`duration.${unit}`, { count: value })
}

/** Dropdown sub-line for a plan: which priorities run 24/7. */
export function planSummary(t: TFunction, plan: SlaPlan): string {
  const always = plan.targets.filter((x) => x.calendar === 'always').map((x) => x.priority)
  return always.length > 0 ? t('settings.sla.summaryAlways', { priorities: always.join(', ') }) : t('settings.sla.summaryBusiness')
}
