import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'

import type { Ticket, TicketMessage } from '../../api/client'
import { AttachmentList } from '../../components/AttachmentList'
import { Message, type MessageVariant } from '../../components/Message'
import { Timestamp } from './Timestamp'

export type Audience = 'agent' | 'requester'

type Author = { id: string; name: string; jobTitle: string | null | undefined; fromSupport: boolean }

function header(t: TFunction, audience: Audience, a: Author, client: string, meId: string): string {
  if (audience === 'agent') {
    if (!a.fromSupport) {
      return a.jobTitle
        ? t('ticket.message.fromClientTitled', { name: a.name, title: a.jobTitle, client })
        : t('ticket.message.fromClient', { name: a.name, client })
    }
    return t('ticket.message.replied', { name: a.name })
  }
  if (a.id === meId) return t('ticket.message.you', { name: a.name })
  if (a.fromSupport) return t('ticket.message.support', { name: a.name })
  return a.jobTitle ? t('ticket.message.colleague', { name: a.name, title: a.jobTitle }) : a.name
}

function variantFor(m: Pick<TicketMessage, 'fromSupport' | 'visibility'>): MessageVariant {
  if (m.visibility === 'internal') return 'internal'
  return m.fromSupport ? 'support' : 'public'
}

type MessageThreadProps = { ticket: Ticket; messages: ReadonlyArray<TicketMessage>; audience: Audience; meId: string }

/**
 * The conversation, oldest first. The ticket's description opens it as the requester's
 * report. Requesters only ever receive public messages from the API.
 */
export function MessageThread({ ticket, messages, audience, meId }: MessageThreadProps) {
  const { t } = useTranslation()
  const client = ticket.client.name
  const requester: Author = {
    id: ticket.requester.id,
    name: ticket.requester.name,
    jobTitle: ticket.requesterJobTitle,
    fromSupport: false,
  }
  return (
    <ol className="flex flex-col gap-3">
      <li>
        <Message
          variant="public"
          header={header(t, audience, requester, client, meId)}
          time={<Timestamp iso={ticket.createdAt} />}
          footer={<AttachmentList items={ticket.attachments} />}
        >
          {ticket.description}
        </Message>
      </li>
      {messages.map((m) => {
        const author: Author = { id: m.author.id, name: m.author.name, jobTitle: m.authorJobTitle, fromSupport: m.fromSupport }
        return (
          <li key={m.id}>
            <Message
              variant={variantFor(m)}
              header={
                m.visibility === 'internal'
                  ? t('ticket.message.internal', { name: m.author.name })
                  : header(t, audience, author, client, meId)
              }
              time={<Timestamp iso={m.createdAt} />}
              footer={<AttachmentList items={m.attachments} />}
            >
              {m.body}
            </Message>
          </li>
        )
      })}
    </ol>
  )
}
