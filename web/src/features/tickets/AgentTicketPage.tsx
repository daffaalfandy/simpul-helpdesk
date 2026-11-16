import { useSuspenseQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { usePriorityNames } from '../../api/queries'
import { BackLink } from '../../components/BackLink'
import { PriorityBadge } from '../../components/Badges'
import { Button } from '../../components/Button'
import { ErrorAlert } from '../../components/ErrorAlert'
import { paths, SLA_REFRESH_MS } from '../../lib/constants'
import { ResolveGuardDialog } from '../dod/ResolveGuardDialog'
import { useSignedInUser } from '../shell/useSignedInUser'
import { ActivityLog } from './ActivityLog'
import { AgentComposer } from './AgentComposer'
import { CodeActivityCard } from './CodeActivityCard'
import { priorityLong } from './labels'
import { MessageThread } from './MessageThread'
import { ticketMessagesQueryOptions, ticketQueryOptions, useSetStatus } from './queries'
import { ServiceLevelCard } from './ServiceLevelCard'
import { TicketDodCard } from './TicketDodCard'
import { needsResolveGuard, TicketProperties } from './TicketProperties'
import { useTimestamp } from './time'

/** Agent ticket detail (HANDOFF §4, Ticket.dc.html). */
export function AgentTicketPage({ ticketKey }: { ticketKey: string }) {
  const { t } = useTranslation()
  const names = usePriorityNames()
  const me = useSignedInUser()
  // The SLA clocks move: refresh the ticket now and then.
  const { data: ticket } = useSuspenseQuery({ ...ticketQueryOptions(ticketKey), refetchInterval: SLA_REFRESH_MS })
  const { data: messages } = useSuspenseQuery(ticketMessagesQueryOptions(ticketKey))
  const { stamp } = useTimestamp()
  const [guarding, setGuarding] = useState(false)
  const resolve = useSetStatus(ticketKey)
  const canSkipDod = needsResolveGuard(ticket) && ticket.allowedStatuses.includes('resolved')

  return (
    <>
      <BackLink to={paths.board}>{t('agent.ticket.back')}</BackLink>
      <header className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-2 text-sm font-semibold text-muted">
          <span>
            {[
              t('ticket.keyType', { key: ticket.key, type: t(`ticket.type.${ticket.type}`) }),
              t('ticket.openedVia', { when: stamp(ticket.createdAt), channel: t(`ticket.channel.${ticket.channel}`) }),
              ticket.loggedByStaff && t('ticket.loggedBy', { name: ticket.createdBy.name }),
            ]
              .filter(Boolean)
              .join(t('common.separator'))}
          </span>
          <PriorityBadge priority={ticket.priority}>{priorityLong(t, names, ticket.priority, ticket.type)}</PriorityBadge>
        </div>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <h1 className="min-w-0 text-2xl leading-tight font-extrabold tracking-title wrap-break-word">{ticket.subject}</h1>
          {canSkipDod && (
            <Button variant="outline" onPress={() => setGuarding(true)}>
              {t('ticket.resolveGuard.button')}
            </Button>
          )}
        </div>
      </header>
      <div className="flex flex-col gap-5 lg:grid lg:grid-cols-[minmax(0,1fr)_var(--ticket-aside-w)] lg:items-start">
        <section aria-label={t('ticket.message.conversation')} className="flex min-w-0 flex-col gap-4">
          <MessageThread ticket={ticket} messages={messages} audience="agent" meId={me.id} />
          <AgentComposer ticket={ticket} />
        </section>
        <aside className="flex min-w-0 flex-col gap-4">
          <ServiceLevelCard ticket={ticket} audience="agent" />
          {ticket.dod && <TicketDodCard ticket={ticket} dod={ticket.dod} />}
          <CodeActivityCard ticket={ticket} />
          <TicketProperties ticket={ticket} onResolveGuard={() => setGuarding(true)} />
          <ActivityLog ticketKey={ticket.key} type={ticket.type} />
        </aside>
      </div>
      {guarding && ticket.dod && (
        <ResolveGuardDialog
          ticketKey={ticket.key}
          pending={ticket.dod.pending}
          custom={ticket.dod.source === 'ticket'}
          isPending={resolve.isPending}
          error={<ErrorAlert error={resolve.error} />}
          onCancel={() => {
            setGuarding(false)
            resolve.reset()
          }}
          onConfirm={(reason) =>
            resolve.mutate({ status: 'resolved', from: ticket.status, reason }, { onSuccess: () => setGuarding(false) })
          }
        />
      )}
    </>
  )
}
