import { useTranslation } from 'react-i18next'

import type { Priority, SlaTarget, TicketType } from '../../api/client'
import { usePriorityNames } from '../../api/queries'
import { PriorityBadge } from '../../components/Badges'
import { OptionCard, OptionCardGroup } from '../../components/OptionCard'
import { formatDuration } from '../../lib/formatDuration'
import { priorityName } from '../tickets/labels'
import { targetFor } from '../tickets/sla'

type PriorityCardsProps = {
  targets: ReadonlyArray<SlaTarget>
  type: TicketType
  catalogMinutes?: number
  workingDayMinutes: number
  /** "08:00 – 17:00 WIB" */
  workingHours: string
  value: Priority | null
  onChange: (p: Priority) => void
  isInvalid: boolean
}

/** Priority picker (HANDOFF §4): four cards with the client's response and fix times. */
export function PriorityCards({
  targets,
  type,
  catalogMinutes,
  workingDayMinutes,
  workingHours,
  value,
  onChange,
  isInvalid,
}: PriorityCardsProps) {
  const { t } = useTranslation()
  const names = usePriorityNames()
  return (
    <OptionCardGroup
      label={t('portal.new.priority')}
      description={t('portal.new.priorityHint')}
      value={value}
      onChange={onChange}
      isRequired
      isInvalid={isInvalid}
      errorMessage={t('portal.new.errors.priority')}
    >
      {targets.map((target) => {
        const p = target.priority
        const eff = targetFor(target, type, catalogMinutes)
        const duration = (minutes: number) => formatDuration(t, minutes, eff.calendar, workingDayMinutes)
        return (
          <OptionCard key={p} value={p}>
            <span className="flex items-center gap-2">
              <PriorityBadge priority={p} />
              <span className="font-extrabold">{priorityName(t, names, p, type)}</span>
            </span>
            <span className="text-sm text-muted">
              {type === 'request' ? t(`priority.request.${p}.when`) : names[p].description}
            </span>
            <span aria-hidden="true" className="border-t border-border" />
            <span className="text-sm">{t('portal.new.firstReply', { duration: duration(eff.firstResponseMinutes) })}</span>
            <span className="text-sm">
              {type === 'request'
                ? t('portal.new.deliveredWithinCard', { duration: duration(eff.resolutionMinutes) })
                : t('portal.new.fixedWithin', { duration: duration(eff.resolutionMinutes) })}
            </span>
            <span className="text-xs text-muted">
              {eff.calendar === 'always' ? t('portal.new.handledAlways') : t('portal.new.handledBusiness', { hours: workingHours })}
            </span>
          </OptionCard>
        )
      })}
    </OptionCardGroup>
  )
}
