import { useTranslation } from 'react-i18next'

import type { Ticket } from '../../api/client'
import { Card } from '../../components/Card'
import { DescriptionList } from '../../components/DescriptionList'
import { formatDuration } from '../../lib/formatDuration'
import type { Audience } from './MessageThread'
import { useTimestamp } from './time'

type ServiceLevelCardProps = { ticket: Ticket; audience: Audience; workingDayMinutes: number }

/**
 * The ticket's targets from its SLA snapshot. Live timers, pausing and breach arrive with the
 * SLA engine (M5); until then this shows the promise, not the countdown.
 */
export function ServiceLevelCard({ ticket, audience, workingDayMinutes }: ServiceLevelCardProps) {
  const { t } = useTranslation()
  const { stamp } = useTimestamp()
  const { sla } = ticket
  const duration = (minutes: number) => formatDuration(t, minutes, sla.calendar, workingDayMinutes)
  const isRequest = ticket.type === 'request'

  if (audience === 'requester') {
    return (
      <Card as="h2" title={t('ticket.sla.requesterTitle', { priority: sla.priority })}>
        <ul className="flex flex-col gap-1.5 text-sm">
          <li>{t('ticket.sla.firstReplyWithin', { duration: duration(sla.firstResponseMinutes) })}</li>
          <li>
            {isRequest
              ? t('ticket.sla.deliveredWithin', { duration: duration(sla.resolutionMinutes) })
              : t('ticket.sla.fixWithin', { duration: duration(sla.resolutionMinutes) })}
          </li>
          <li className="text-muted">{sla.calendar === 'always' ? t('ticket.sla.always') : t('ticket.sla.business')}</li>
        </ul>
        <p className="text-xs text-muted">{t('ticket.sla.pausedNote')}</p>
      </Card>
    )
  }
  return (
    <Card as="h2" title={t('ticket.sla.agentTitle', { priority: sla.priority })}>
      <DescriptionList
        items={[
          { term: t('ticket.sla.firstResponse'), value: duration(sla.firstResponseMinutes) },
          {
            term: isRequest ? t('ticket.sla.deliver') : t('ticket.sla.resolve'),
            value: duration(sla.resolutionMinutes),
          },
        ]}
      />
      <ul className="flex flex-col gap-1 text-xs text-muted">
        <li>{sla.calendar === 'always' ? t('ticket.sla.always') : t('ticket.sla.business')}</li>
        {sla.fromCatalog && <li>{t('ticket.sla.fromCatalog')}</li>}
        {ticket.firstResponseAt && <li>{t('ticket.sla.firstResponseMet', { when: stamp(ticket.firstResponseAt) })}</li>}
      </ul>
    </Card>
  )
}
