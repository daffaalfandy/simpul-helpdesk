import { useTranslation } from 'react-i18next'

import type { Priority, ProjectTarget, TicketType } from '../../api/client'
import { usePriorityNames } from '../../api/queries'
import { PriorityBadge } from '../../components/Badges'
import { OptionCard, OptionCardGroup } from '../../components/OptionCard'
import { formatTarget } from '../../lib/formatDuration'
import { priorityName } from '../tickets/labels'
import { targetFor } from '../tickets/sla'

const PRIORITIES: readonly Priority[] = ['P1', 'P2', 'P3', 'P4']

type PriorityCardsProps = {
  /** The project's targets; null until an incident's system is chosen. */
  targets: ReadonlyArray<ProjectTarget> | null
  type: TicketType
  /** "08:00 – 17:00 WIB" */
  workingHours: string
  value: Priority | null
  onChange: (p: Priority) => void
  isInvalid: boolean
}

/** Priority picker (HANDOFF §4): four cards with the project's response and fix times. */
export function PriorityCards({ targets, type, workingHours, value, onChange, isInvalid }: PriorityCardsProps) {
  const { t } = useTranslation()
  const names = usePriorityNames()
  return (
    <OptionCardGroup
      label={t('portal.new.priority')}
      description={targets ? t('portal.new.priorityHint') : t('portal.new.priorityPickSystem')}
      value={value}
      onChange={onChange}
      isRequired
      isInvalid={isInvalid}
      errorMessage={t('portal.new.errors.priority')}
    >
      {PRIORITIES.map((p) => {
        const target = targets?.find((x) => x.priority === p)
        const eff = target ? targetFor(target, type) : null
        return (
          <OptionCard key={p} value={p}>
            <span className="flex items-center gap-2">
              <PriorityBadge priority={p} />
              <span className="font-extrabold">{priorityName(t, names, p, type)}</span>
            </span>
            <span className="text-sm text-muted">
              {type === 'request' ? t(`priority.request.${p}.when`) : names[p].description}
            </span>
            {eff && (
              <>
                <span aria-hidden="true" className="border-t border-border" />
                <span className="text-sm">{t('portal.new.firstReply', { duration: formatTarget(t, eff.firstResponse) })}</span>
                <span className="text-sm">
                  {type === 'request'
                    ? t('portal.new.deliveredWithinCard', { duration: formatTarget(t, eff.resolution) })
                    : t('portal.new.fixedWithin', { duration: formatTarget(t, eff.resolution) })}
                </span>
                <span className="text-xs text-muted">
                  {eff.calendar === 'always'
                    ? t('portal.new.handledAlways')
                    : t('portal.new.handledBusiness', { hours: workingHours })}
                </span>
              </>
            )}
          </OptionCard>
        )
      })}
    </OptionCardGroup>
  )
}
