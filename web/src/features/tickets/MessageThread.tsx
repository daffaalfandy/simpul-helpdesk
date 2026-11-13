import { Trans } from 'react-i18next'

import type { Ticket, TicketMessage } from '../../api/client'
import { AttachmentList } from '../../components/AttachmentList'
import { Message, type MessageVariant } from '../../components/Message'
import { Timestamp } from './Timestamp'

export type Audience = 'agent' | 'requester'

type Author = { id: string; name: string; jobTitle: string | null | undefined; fromSupport: boolean }

type HeaderKey =
  | 'ticket.message.fromClient'
  | 'ticket.message.fromClientTitled'
  | 'ticket.message.replied'
  | 'ticket.message.you'
  | 'ticket.message.support'
  | 'ticket.message.colleague'
  | 'ticket.message.nameOnly'
  | 'ticket.message.internal'

type Header = { key: HeaderKey; values: Record<string, string> }

function header(audience: Audience, a: Author, client: string, meId: string): Header {
  const name = a.name
  if (audience === 'agent') {
    if (!a.fromSupport) {
      return a.jobTitle
        ? { key: 'ticket.message.fromClientTitled', values: { name, title: a.jobTitle, client } }
        : { key: 'ticket.message.fromClient', values: { name, client } }
    }
    return { key: 'ticket.message.replied', values: { name } }
  }
  if (a.id === meId) return { key: 'ticket.message.you', values: { name } }
  if (a.fromSupport) return { key: 'ticket.message.support', values: { name } }
  return a.jobTitle
    ? { key: 'ticket.message.colleague', values: { name, title: a.jobTitle } }
    : { key: 'ticket.message.nameOnly', values: { name } }
}

/** The author line with the name in bold (`<b>` in the translation). */
function HeaderText({ header: h }: { header: Header }) {
  return <Trans i18nKey={h.key} values={h.values} components={{ b: <strong /> }} />
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
          header={<HeaderText header={header(audience, requester, client, meId)} />}
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
                <HeaderText
                  header={
                    m.visibility === 'internal'
                      ? { key: 'ticket.message.internal', values: { name: m.author.name } }
                      : header(audience, author, client, meId)
                  }
                />
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
