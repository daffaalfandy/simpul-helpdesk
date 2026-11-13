import { useTranslation } from 'react-i18next'

import type { Ticket } from '../../api/client'
import { Card } from '../../components/Card'
import { formatTarget, formatTargetCompact } from '../../lib/formatDuration'
import type { Audience } from './MessageThread'
import { useTimestamp } from './time'

type ServiceLevelCardProps = { ticket: Ticket; audience: Audience }

/**
 * The ticket's targets from its SLA snapshot (the project's SLA when the ticket was opened).
 * Live timers, bars and breach states arrive with the SLA engine (M5b); until then this shows
 * the promise and when the first reply went out.
 */
export function ServiceLevelCard({ ticket, audience }: ServiceLevelCardProps) {
  const { t } = useTranslation()
  const { stamp } = useTimestamp()
  const { sla } = ticket
  const isRequest = ticket.type === 'request'
  const pausedNote =
    sla.pauseOnWaitingClient && sla.pauseOnUat
      ? t('ticket.sla.pausedBoth')
      : sla.pauseOnWaitingClient
        ? t('ticket.sla.pausedWaiting')
        : sla.pauseOnUat
          ? t('ticket.sla.pausedUat')
          : null
  const replied = ticket.firstResponseAt ? stamp(ticket.firstResponseAt) : null

  if (audience === 'requester') {
    return (
      <Card as="h2" variant="compact" title={t('ticket.sla.requesterTitle', { priority: sla.priority })}>
        <dl className="flex flex-col gap-2 text-sm">
          <div className="flex flex-wrap justify-between gap-x-3">
            <dt>{t('ticket.sla.firstReplyWithin', { duration: formatTarget(t, sla.firstResponse) })}</dt>
            {replied && <dd className="font-bold text-success">{t('ticket.sla.replied')}</dd>}
          </div>
          <div className="flex flex-wrap justify-between gap-x-3">
            <dt>
              {isRequest
                ? t('ticket.sla.deliveredWithin', { duration: formatTarget(t, sla.resolution) })
                : t('ticket.sla.fixWithin', { duration: formatTarget(t, sla.resolution) })}
            </dt>
          </div>
        </dl>
        <p className="text-xs text-muted">
          {sla.calendar === 'always' ? t('ticket.sla.always') : t('ticket.sla.business')}
          {pausedNote && <> {pausedNote}</>}
        </p>
      </Card>
    )
  }

  const rows = [
    { label: t('ticket.sla.firstResponse'), target: sla.firstResponse, state: replied && t('ticket.sla.sentAt', { when: replied }) },
    { label: isRequest ? t('ticket.sla.deliver') : t('ticket.sla.resolve'), target: sla.resolution, state: null },
  ]
  return (
    <section
      aria-labelledby={`sla-${ticket.id}`}
      className="flex flex-col gap-3 rounded-2xl bg-ink px-4.5 py-4 text-on-primary shadow-card"
    >
      <h2 id={`sla-${ticket.id}`} className="text-base font-extrabold">
        {t('ticket.sla.agentTitle', { priority: sla.priority, project: ticket.project.name })}
      </h2>
      <dl className="flex flex-col gap-2 text-sm">
        {rows.map((r) => (
          <div key={r.label} className="flex flex-wrap items-baseline justify-between gap-x-3">
            <dt>{t('ticket.sla.row', { label: r.label, target: formatTargetCompact(t, r.target) })}</dt>
            {r.state && <dd className="font-bold text-success-border">{r.state}</dd>}
          </div>
        ))}
      </dl>
      <p className="text-xs text-on-dark-muted">
        {sla.calendar === 'always' ? t('ticket.sla.always') : t('ticket.sla.business')}
        {pausedNote && <> {pausedNote}</>}
      </p>
    </section>
  )
}
