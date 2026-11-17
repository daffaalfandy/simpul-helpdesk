import { useSuspenseQuery } from '@tanstack/react-query'
import { Check, RotateCcw, X } from 'lucide-react'
import { useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

import type { Ticket, TicketProgress, UatAnswer } from '../../api/client'
import { Alert } from '../../components/Alert'
import { ConfirmDialog } from '../../components/Dialog'
import { attachmentPolicyQueryOptions, usePriorityNames } from '../../api/queries'
import { AttachmentPicker } from '../../components/AttachmentPicker'
import { Pill, PriorityBadge } from '../../components/Badges'
import { BackLink } from '../../components/BackLink'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { DescriptionList } from '../../components/DescriptionList'
import { ErrorAlert } from '../../components/ErrorAlert'
import { ProgressTracker } from '../../components/ProgressTracker'
import { RichText } from '../../components/RichText'
import { RichTextField } from '../../components/RichTextField'
import { MAX_MESSAGE_LENGTH, paths, SLA_REFRESH_MS } from '../../lib/constants'
import { megabytesOf } from '../../lib/files'
import { runeLength } from '../../lib/text'
import { useSignedInUser } from '../shell/useSignedInUser'
import { priorityLong, requesterStatus } from '../tickets/labels'
import { MessageThread } from '../tickets/MessageThread'
import { ticketMessagesQueryOptions, ticketQueryOptions, useAddMessage } from '../tickets/queries'
import { ServiceLevelCard } from '../tickets/ServiceLevelCard'
import { STATUS_TONE } from '../tickets/status'
import { useTimestamp } from '../tickets/time'
import { useAttachmentUploads } from '../tickets/useAttachmentUploads'
import { PeopleNotifiedCard } from './PeopleNotifiedCard'
import { useReopen } from './queries'
import { UatAnswerDialog } from './UatAnswerDialog'

type PortalTicketPageProps = {
  ticketKey: string
  /** An answer from the test email's buttons (?answer=): its dialog opens once. */
  answer?: UatAnswer
  onAnswerShown: () => void
}

/** What the last client action did, shown above the page until the next one. */
type Notice = 'approved' | 'rejected' | 'reopened' | 'gone'

/** Requester ticket view (HANDOFF §4, PortalTicket.dc.html). */
export function PortalTicketPage({ ticketKey, answer, onAnswerShown }: PortalTicketPageProps) {
  const { t } = useTranslation()
  const names = usePriorityNames()
  const me = useSignedInUser()
  // The SLA clocks move: refresh the ticket now and then.
  const { data: ticket } = useSuspenseQuery({ ...ticketQueryOptions(ticketKey), refetchInterval: SLA_REFRESH_MS })
  const { data: messages } = useSuspenseQuery(ticketMessagesQueryOptions(ticketKey))
  const { stamp, full } = useTimestamp()
  const [answering, setAnswering] = useState<UatAnswer | null>(null)
  const [notice, setNotice] = useState<Notice | null>(null)

  // The test email's buttons land here with ?answer=: open that dialog if the ticket still waits
  // for this user's answer, or say it doesn't, then drop the parameter.
  const [seenAnswer, setSeenAnswer] = useState<UatAnswer | undefined>(undefined)
  if (answer && answer !== seenAnswer) {
    setSeenAnswer(answer)
    if (ticket.uat?.canAnswer) setAnswering(answer)
    else setNotice('gone')
  }
  useEffect(() => {
    if (answer) onAnswerShown()
  }, [answer, onAnswerShown])

  const detail = (p: TicketProgress) => {
    if (p.step === 'in_progress' && p.state === 'current' && ticket.status === 'waiting_on_client') {
      return t('portal.ticket.waitingForYou')
    }
    if (p.step === 'ready_for_test' && p.state === 'current') return t('portal.uat.trackerWaiting')
    if (!p.at) return undefined
    if (p.step === 'assigned' && ticket.assignee) {
      return t('portal.ticket.assignedTo', { name: ticket.assignee.name, when: stamp(p.at) })
    }
    return stamp(p.at)
  }
  const what = ticket.type === 'request' ? ticket.catalogItem : ticket.project

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
      {notice === 'gone' && <Alert tone="error">{t('portal.uat.gone')}</Alert>}
      {(notice === 'approved' || notice === 'rejected' || notice === 'reopened') && (
        <Alert tone="success">
          {notice === 'approved'
            ? t('portal.uat.approved', { key: ticket.key })
            : notice === 'rejected'
              ? t('portal.uat.rejected')
              : t('portal.reopen.reopened', { key: ticket.key })}
        </Alert>
      )}
      {ticket.uat && (
        <UatBanner
          ticket={ticket}
          onAnswer={(a) => {
            setNotice(null)
            setAnswering(a)
          }}
        />
      )}
      {ticket.reopen && <ReopenBanner ticket={ticket} onReopened={() => setNotice('reopened')} />}
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
          {ticket.reopen?.canReopen && <p className="text-xs text-muted">{t('portal.ticket.replyResolvedHint')}</p>}
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
          <ServiceLevelCard ticket={ticket} audience="requester" />
          <PeopleNotifiedCard ticket={ticket} meId={me.id} />
        </aside>
      </div>
      {answering && (
        <UatAnswerDialog
          ticketKey={ticket.key}
          subject={ticket.subject}
          answer={answering}
          onClose={() => setAnswering(null)}
          onAnswered={() => {
            setAnswering(null)
            setNotice(answering === 'approve' ? 'approved' : 'rejected')
          }}
        />
      )}
    </>
  )
}

/** The navy "Your test is needed" banner (PortalTicket mock), while the ticket waits In UAT. */
function UatBanner({ ticket, onAnswer }: { ticket: Ticket; onAnswer: (a: UatAnswer) => void }) {
  const { t } = useTranslation()
  const { day } = useTimestamp()
  const uat = ticket.uat
  if (!uat) return null
  return (
    <Card tone="dark" labelledBy={`uat-${ticket.id}`}>
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="flex max-w-3xl min-w-0 flex-col gap-1.5">
          <h2 id={`uat-${ticket.id}`} className="text-xs font-extrabold tracking-caps text-on-dark-accent uppercase">
            {t('portal.uat.banner.eyebrow')}
          </h2>
          {uat.instructions ? (
            <RichText markdown={uat.instructions} tone="onDark" className="text-md leading-relaxed" />
          ) : (
            <p className="text-md leading-relaxed">{t('portal.uat.noInstructions')}</p>
          )}
          {uat.autoApproveAt && (
            <p className="text-sm text-on-dark-muted">{t('portal.uat.banner.autoApprove', { date: day(uat.autoApproveAt) })}</p>
          )}
        </div>
        {uat.canAnswer && (
          <div className="flex flex-wrap gap-2.5">
            <Button variant="success" icon={<Check aria-hidden="true" className="size-(--icon-md)" />} onPress={() => onAnswer('approve')}>
              {t('portal.uat.approve')}
            </Button>
            <Button variant="danger" icon={<X aria-hidden="true" className="size-(--icon-md)" />} onPress={() => onAnswer('reject')}>
              {t('portal.uat.reject')}
            </Button>
          </div>
        )}
      </div>
    </Card>
  )
}

/** The green "Resolved" banner with the reopen window (PortalTicket mock). */
function ReopenBanner({ ticket, onReopened }: { ticket: Ticket; onReopened: () => void }) {
  const { t } = useTranslation()
  const { day } = useTimestamp()
  const [confirming, setConfirming] = useState(false)
  const reopen = useReopen(ticket.key)
  const info = ticket.reopen
  if (!info) return null
  // "Thank you for testing" when the ticket went through the client's test.
  const tested = ticket.progress.some((p) => p.step === 'ready_for_test' && p.state === 'done')
  return (
    <section className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-success-border bg-success-tint px-4.5 py-3.5 text-success-ink">
      <p className="max-w-3xl text-base font-semibold">
        {t(tested ? 'portal.reopen.bannerAfterTest' : 'portal.reopen.banner', { date: day(info.until) })}
      </p>
      {info.canReopen && (
        <Button variant="outline" icon={<RotateCcw aria-hidden="true" className="size-(--icon-md)" />} onPress={() => setConfirming(true)}>
          {t('portal.reopen.button')}
        </Button>
      )}
      {confirming && (
        <ConfirmDialog
          isOpen
          onOpenChange={(open) => {
            if (!open) {
              setConfirming(false)
              reopen.reset()
            }
          }}
          title={t('portal.reopen.dialog.title', { key: ticket.key })}
          confirmLabel={t('portal.reopen.dialog.confirm')}
          isPending={reopen.isPending}
          error={<ErrorAlert error={reopen.error} />}
          onConfirm={() =>
            reopen.mutate(undefined, {
              onSuccess: () => {
                setConfirming(false)
                onReopened()
              },
            })
          }
        >
          {t('portal.reopen.dialog.body')}
        </ConfirmDialog>
      )}
    </section>
  )
}

/** The mock's reply box is three lines tall. */
const REPLY_ROWS = 3

function ReplyBox({ ticket }: { ticket: Ticket }) {
  const { t } = useTranslation()
  const { data: policy } = useSuspenseQuery(attachmentPolicyQueryOptions)
  const uploads = useAttachmentUploads(policy)
  const send = useAddMessage(ticket.key)
  const [body, setBody] = useState('')
  const [submitted, setSubmitted] = useState(false)
  const empty = body.trim() === ''
  const tooLong = runeLength(body) > MAX_MESSAGE_LENGTH

  const submit = (e: FormEvent) => {
    e.preventDefault()
    setSubmitted(true)
    if (empty || tooLong || uploads.isBlocking) return
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
    <section aria-label={t('portal.ticket.replyLabel')} className="rounded-xl border border-border-strong bg-surface px-4 py-3.5">
      <form onSubmit={submit} noValidate className="flex flex-col gap-2.5">
        <RichTextField
          label={t('portal.ticket.replyLabel')}
          rows={REPLY_ROWS}
          placeholder={t('portal.ticket.replyPlaceholder')}
          value={body}
          onChange={setBody}
          maxLength={MAX_MESSAGE_LENGTH}
          isInvalid={submitted && empty}
          errorMessage={t('ticket.composer.bodyRequired')}
        />
        <div className="flex flex-wrap items-start justify-between gap-3">
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
          <Button type="submit" isPending={send.isPending}>
            {t('portal.ticket.send')}
          </Button>
        </div>
        {submitted && uploads.isBlocking && <p className="text-xs font-semibold text-danger">{t('attach.waitForUploads')}</p>}
        <ErrorAlert error={send.error} />
      </form>
    </section>
  )
}
