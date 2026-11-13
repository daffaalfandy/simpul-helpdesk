import { useSuspenseQuery } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

import type { Ticket, TicketProgress } from '../../api/client'
import { attachmentPolicyQueryOptions, usePriorityNames } from '../../api/queries'
import { AttachmentPicker } from '../../components/AttachmentPicker'
import { Pill, PriorityBadge } from '../../components/Badges'
import { BackLink } from '../../components/BackLink'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { DescriptionList } from '../../components/DescriptionList'
import { ErrorAlert } from '../../components/ErrorAlert'
import { ProgressTracker } from '../../components/ProgressTracker'
import { TextArea } from '../../components/TextArea'
import { paths } from '../../lib/constants'
import { megabytesOf } from '../../lib/files'
import { businessHoursQueryOptions } from '../settings/queries'
import { useSignedInUser } from '../shell/useSignedInUser'
import { priorityLong, requesterStatus } from '../tickets/labels'
import { MessageThread } from '../tickets/MessageThread'
import { ticketMessagesQueryOptions, ticketQueryOptions, useAddMessage } from '../tickets/queries'
import { ServiceLevelCard } from '../tickets/ServiceLevelCard'
import { STATUS_TONE } from '../tickets/status'
import { useTimestamp } from '../tickets/time'
import { useAttachmentUploads } from '../tickets/useAttachmentUploads'

/** Requester ticket view (HANDOFF §4, PortalTicket.dc.html), M3 parts. */
export function PortalTicketPage({ ticketKey }: { ticketKey: string }) {
  const { t } = useTranslation()
  const names = usePriorityNames()
  const me = useSignedInUser()
  const { data: ticket } = useSuspenseQuery(ticketQueryOptions(ticketKey))
  const { data: messages } = useSuspenseQuery(ticketMessagesQueryOptions(ticketKey))
  const { data: hours } = useSuspenseQuery(businessHoursQueryOptions)
  const { stamp, full } = useTimestamp()

  const detail = (p: TicketProgress) => {
    if (p.step === 'in_progress' && p.state === 'current' && ticket.status === 'waiting_on_client') {
      return t('portal.ticket.waitingForYou')
    }
    if (!p.at) return undefined
    if (p.step === 'assigned' && ticket.assignee) {
      return t('portal.ticket.assignedTo', { name: ticket.assignee.name, when: stamp(p.at) })
    }
    return stamp(p.at)
  }
  const what = ticket.type === 'request' ? ticket.catalogItem : ticket.system

  return (
    <>
      <BackLink to={paths.portal}>{t('portal.ticket.back')}</BackLink>
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex min-w-0 flex-col gap-2">
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 text-sm text-muted">
            <span className="font-bold text-ink">
              {t('ticket.keyType', { key: ticket.key, type: t(`ticket.type.${ticket.type}`) })}
            </span>
            <span>{t('ticket.reported', { when: full(ticket.createdAt) })}</span>
            <PriorityBadge priority={ticket.priority}>{priorityLong(t, names, ticket.priority, ticket.type)}</PriorityBadge>
          </div>
          <h1 className="text-2xl leading-tight font-extrabold tracking-title wrap-break-word">{ticket.subject}</h1>
        </div>
        <Pill tone={STATUS_TONE[ticket.status]} size="md">
          {requesterStatus(t, ticket.status)}
        </Pill>
      </header>
      <Card>
        <ProgressTracker
          label={t('portal.ticket.progress')}
          doneLabel={t('common.completed')}
          steps={ticket.progress.map((p) => ({
            id: p.step,
            label: t(`portal.ticket.steps.${p.step}`),
            state: p.state,
            detail: detail(p),
          }))}
        />
      </Card>
      <div className="flex flex-col gap-5 lg:grid lg:grid-cols-[minmax(0,1fr)_var(--ticket-aside-w)] lg:items-start">
        <section aria-label={t('ticket.message.conversation')} className="flex min-w-0 flex-col gap-4">
          <MessageThread ticket={ticket} messages={messages} audience="requester" meId={me.id} />
          {ticket.status !== 'closed' && <ReplyBox ticket={ticket} />}
        </section>
        <aside className="flex min-w-0 flex-col gap-4">
          <Card as="h2" title={t('portal.ticket.details')}>
            <DescriptionList
              items={[
                ...(what
                  ? [
                      {
                        term: ticket.type === 'request' ? t('ticket.fields.catalogItem') : t('ticket.fields.system'),
                        value: what.name,
                      },
                    ]
                  : []),
                { term: t('ticket.fields.type'), value: t(`ticket.type.${ticket.type}`) },
                { term: t('ticket.fields.reportedBy'), value: ticket.requester.name },
                { term: t('ticket.fields.handledBy'), value: ticket.assignee?.name ?? t('portal.ticket.notAssigned') },
              ]}
            />
          </Card>
          <ServiceLevelCard ticket={ticket} audience="requester" workingDayMinutes={hours.workingDayMinutes} />
        </aside>
      </div>
    </>
  )
}

function ReplyBox({ ticket }: { ticket: Ticket }) {
  const { t } = useTranslation()
  const { data: policy } = useSuspenseQuery(attachmentPolicyQueryOptions)
  const uploads = useAttachmentUploads(policy)
  const send = useAddMessage(ticket.key)
  const [body, setBody] = useState('')
  const [submitted, setSubmitted] = useState(false)
  const empty = body.trim() === ''

  const submit = (e: FormEvent) => {
    e.preventDefault()
    setSubmitted(true)
    if (empty || uploads.isBlocking) return
    send.mutate(
      { visibility: 'public', body: body.trim(), attachmentIds: uploads.attachmentIds },
      {
        onSuccess: () => {
          setBody('')
          setSubmitted(false)
          uploads.reset()
        },
      },
    )
  }

  return (
    <Card>
      <form onSubmit={submit} noValidate className="flex flex-col gap-3">
        <TextArea
          label={t('portal.ticket.replyLabel')}
          placeholder={t('portal.ticket.replyPlaceholder')}
          value={body}
          onChange={setBody}
          isInvalid={submitted && empty}
          errorMessage={t('ticket.composer.bodyRequired')}
        />
        <AttachmentPicker
          items={uploads.items}
          onSelect={uploads.add}
          onRemove={uploads.remove}
          onRetry={uploads.retry}
          accept={policy.extensions}
          buttonLabel={t('attach.button')}
          hint={t('attach.hint', { max: megabytesOf(policy.maxBytes) })}
          variant="dashed"
        />
        {submitted && uploads.isBlocking && <p className="text-xs font-semibold text-danger">{t('attach.waitForUploads')}</p>}
        <ErrorAlert error={send.error} />
        <div className="flex justify-end">
          <Button type="submit" isPending={send.isPending}>
            {t('portal.ticket.send')}
          </Button>
        </div>
      </form>
    </Card>
  )
}
