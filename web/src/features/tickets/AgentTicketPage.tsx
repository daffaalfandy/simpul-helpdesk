import { useSuspenseQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { usePriorityNames } from '../../api/queries'
import { BackLink } from '../../components/BackLink'
import { PriorityBadge } from '../../components/Badges'
import { paths } from '../../lib/constants'
import { useSignedInUser } from '../shell/useSignedInUser'
import { ActivityLog } from './ActivityLog'
import { AgentComposer } from './AgentComposer'
import { priorityLong } from './labels'
import { MessageThread } from './MessageThread'
import { ticketMessagesQueryOptions, ticketQueryOptions } from './queries'
import { ServiceLevelCard } from './ServiceLevelCard'
import { TicketProperties } from './TicketProperties'
import { useTimestamp } from './time'

/** Agent ticket detail (HANDOFF §4, Ticket.dc.html), M3 parts. */
export function AgentTicketPage({ ticketKey }: { ticketKey: string }) {
  const { t } = useTranslation()
  const names = usePriorityNames()
  const me = useSignedInUser()
  const { data: ticket } = useSuspenseQuery(ticketQueryOptions(ticketKey))
  const { data: messages } = useSuspenseQuery(ticketMessagesQueryOptions(ticketKey))
  const { stamp } = useTimestamp()

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
        <h1 className="text-2xl leading-tight font-extrabold tracking-title wrap-break-word">{ticket.subject}</h1>
      </header>
      <div className="flex flex-col gap-5 lg:grid lg:grid-cols-[minmax(0,1fr)_var(--ticket-aside-w)] lg:items-start">
        <section aria-label={t('ticket.message.conversation')} className="flex min-w-0 flex-col gap-4">
          <MessageThread ticket={ticket} messages={messages} audience="agent" meId={me.id} />
          <AgentComposer ticket={ticket} />
        </section>
        <aside className="flex min-w-0 flex-col gap-4">
          <ServiceLevelCard ticket={ticket} audience="agent" />
          <TicketProperties ticket={ticket} />
          <ActivityLog ticketKey={ticket.key} type={ticket.type} />
        </aside>
      </div>
    </>
  )
}
