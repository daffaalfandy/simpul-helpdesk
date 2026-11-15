import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'

import type { SlaClock, SlaClockName, SlaPhase, Ticket } from '../../api/client'
import { Card } from '../../components/Card'
import { formatTarget, formatTargetCompact } from '../../lib/formatDuration'
import { cx } from '../../lib/text'
import type { Audience } from './MessageThread'
import { compactSpan, shownPhase } from './slaFormat'
import { useTimestamp } from './time'

type ServiceLevelCardProps = { ticket: Ticket; audience: Audience }

const FULL = 100

/** Bar and text colour on the dark SLA box; the text always names the state too. */
const DARK_TONE: Record<SlaPhase, { bar: string; text: string }> = {
  running: { bar: 'bg-on-dark-accent', text: 'text-on-dark-accent' },
  at_risk: { bar: 'bg-chip-warning-bg', text: 'text-chip-warning-bg' },
  breached: { bar: 'bg-chip-danger-bg', text: 'text-chip-danger-bg' },
  missed: { bar: 'bg-chip-danger-bg', text: 'text-chip-danger-bg' },
  paused: { bar: 'bg-chip-paused-bg', text: 'text-chip-paused-bg' },
  met: { bar: 'bg-chip-success-bg', text: 'text-chip-success-bg' },
  stopped: { bar: 'bg-on-dark-muted', text: 'text-on-dark-muted' },
}

/** What a clock says on the agent's SLA box: "Met in 8m", "41m left", "Breached 12m". */
function agentClockText(t: TFunction, clock: SlaClockName, c: SlaClock): string {
  const left = compactSpan(t, c.remainingMs, c.dayMs)
  switch (shownPhase(c)) {
    case 'met':
      return t('ticket.sla.metIn', { span: compactSpan(t, c.usedMs, c.dayMs) })
    case 'missed':
      return t(clock === 'first_response' ? 'ticket.sla.repliedLate' : 'ticket.sla.resolvedLate')
    case 'running':
      return clock === 'first_response' ? t('sla.respondIn', { span: left }) : t('sla.left', { span: left })
    case 'at_risk':
      return t('ticket.sla.atRisk', { span: left })
    case 'breached':
      return t('sla.breached', { span: left })
    case 'paused':
      return t('sla.paused', { span: left })
    case 'stopped':
      return t('ticket.sla.stopped')
  }
}

/** The requester's view of the fix clock: no deadlines, just where it stands (Portal mock). */
function requesterFixText(t: TFunction, c: SlaClock): string | null {
  switch (c.phase) {
    case 'met':
    case 'missed':
      return t('ticket.sla.resolved')
    case 'paused':
      return t('ticket.sla.pausedState')
    case 'stopped':
      return null
    default:
      return t('ticket.sla.inProgress')
  }
}

/**
 * The ticket's SLA: its targets (from the project's SLA when it was opened) and the live clocks.
 * Agents get the dark box with bars (Ticket mock); requesters the simpler service-level card.
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
    const fr = sla.firstResponseClock
    const fix = sla.resolutionClock && requesterFixText(t, sla.resolutionClock)
    const firstReply =
      fr?.phase === 'met'
        ? t('ticket.sla.metAfter', { span: compactSpan(t, fr.usedMs, fr.dayMs) })
        : replied && t('ticket.sla.replied')
    return (
      <Card as="h2" variant="compact" title={t('ticket.sla.requesterTitle', { priority: sla.priority })}>
        <dl className="flex flex-col gap-2 text-sm">
          <div className="flex flex-wrap justify-between gap-x-3">
            <dt>{t('ticket.sla.firstReplyWithin', { duration: formatTarget(t, sla.firstResponse) })}</dt>
            {firstReply && <dd className="font-bold text-success">{firstReply}</dd>}
          </div>
          <div className="flex flex-wrap justify-between gap-x-3">
            <dt>
              {isRequest
                ? t('ticket.sla.deliveredWithin', { duration: formatTarget(t, sla.resolution) })
                : t('ticket.sla.fixWithin', { duration: formatTarget(t, sla.resolution) })}
            </dt>
            {fix && <dd className="font-bold">{fix}</dd>}
          </div>
        </dl>
        <p className="flex flex-col gap-0.5 text-xs text-muted">
          <span>{sla.calendar === 'always' ? t('ticket.sla.always') : t('ticket.sla.business')}</span>
          {pausedNote && <span>{pausedNote}</span>}
        </p>
      </Card>
    )
  }

  const rows = [
    {
      id: 'first_response' as const,
      label: t('ticket.sla.firstResponse'),
      target: sla.firstResponse,
      clock: sla.firstResponseClock,
      fallback: replied && t('ticket.sla.sentAt', { when: replied }),
    },
    {
      id: 'resolution' as const,
      label: isRequest ? t('ticket.sla.deliver') : t('ticket.sla.resolve'),
      target: sla.resolution,
      clock: sla.resolutionClock,
      fallback: null,
    },
  ]
  return (
    <section
      aria-labelledby={`sla-${ticket.id}`}
      className="flex flex-col gap-3 rounded-2xl bg-ink px-4.5 py-4 text-on-primary shadow-card"
    >
      <h2 id={`sla-${ticket.id}`} className="text-base font-extrabold">
        {t('ticket.sla.agentTitle', { priority: sla.priority, project: ticket.project.name })}
      </h2>
      <dl className="flex flex-col gap-3 text-sm">
        {rows.map((r) => {
          const text = r.clock ? agentClockText(t, r.id, r.clock) : r.fallback
          const tone = r.clock ? DARK_TONE[shownPhase(r.clock)] : null
          const used = r.clock ? Math.min(FULL, Math.round((r.clock.usedMs / r.clock.budgetMs) * FULL)) : 0
          return (
            <div key={r.id} className="flex flex-col gap-1.5">
              <div className="flex flex-wrap items-baseline justify-between gap-x-3">
                <dt>{t('ticket.sla.row', { label: r.label, target: formatTargetCompact(t, r.target) })}</dt>
                {text && <dd className={cx('font-bold', tone?.text ?? 'text-success-border')}>{text}</dd>}
              </div>
              {r.clock && tone && (
                <div
                  role="meter"
                  aria-label={r.label}
                  aria-valuemin={0}
                  aria-valuemax={FULL}
                  aria-valuenow={used}
                  aria-valuetext={text ?? undefined}
                  className="h-(--tracker-bar-h) overflow-hidden rounded-full bg-ink-2"
                >
                  <div className={cx('h-full rounded-full', tone.bar)} style={{ width: `${used}%` }} />
                </div>
              )}
            </div>
          )
        })}
      </dl>
      <p className="flex flex-col gap-0.5 text-xs text-on-dark-muted">
        <span>{sla.calendar === 'always' ? t('ticket.sla.always') : t('ticket.sla.business')}</span>
        {pausedNote && <span>{pausedNote}</span>}
      </p>
    </section>
  )
}
