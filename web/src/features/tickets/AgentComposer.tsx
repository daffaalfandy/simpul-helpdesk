import { useSuspenseQuery } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

import type { MessageVisibility, Ticket } from '../../api/client'
import { attachmentPolicyQueryOptions } from '../../api/queries'
import { AttachmentPicker } from '../../components/AttachmentPicker'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { ErrorAlert } from '../../components/ErrorAlert'
import { SegmentedTabs } from '../../components/SegmentedTabs'
import { TextArea } from '../../components/TextArea'
import { megabytesOf } from '../../lib/files'
import { useAddMessage } from './queries'
import { useAttachmentUploads } from './useAttachmentUploads'

/** "Reply to client" vs "Internal note" (HANDOFF §4); the note tab turns the field yellow. */
export function AgentComposer({ ticket }: { ticket: Ticket }) {
  const { t } = useTranslation()
  const { data: policy } = useSuspenseQuery(attachmentPolicyQueryOptions)
  const uploads = useAttachmentUploads(policy)
  const send = useAddMessage(ticket.key)
  const [mode, setMode] = useState<MessageVisibility>('public')
  const [body, setBody] = useState('')
  const [submitted, setSubmitted] = useState(false)
  const empty = body.trim() === ''
  const locked = ticket.status === 'closed'

  const submit = (e: FormEvent) => {
    e.preventDefault()
    setSubmitted(true)
    if (empty || uploads.isBlocking) return
    send.mutate(
      { visibility: mode, body: body.trim(), attachmentIds: uploads.attachmentIds },
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
      <form onSubmit={submit} className="flex flex-col gap-3">
        <SegmentedTabs
          label={t('ticket.composer.tabs')}
          tabs={[
            { id: 'public', label: t('ticket.composer.reply') },
            { id: 'internal', label: t('ticket.composer.note') },
          ]}
          selectedKey={mode}
          onSelectionChange={setMode}
        >
          {(tab) => (
            <TextArea
              label={
                tab === 'internal'
                  ? t('ticket.composer.noteLabel')
                  : t('ticket.composer.replyLabel', { name: ticket.requester.name })
              }
              placeholder={tab === 'internal' ? t('ticket.composer.notePlaceholder') : t('ticket.composer.replyPlaceholder')}
              tone={tab === 'internal' ? 'internal' : 'default'}
              value={body}
              onChange={setBody}
              isDisabled={locked}
              isInvalid={submitted && empty}
              errorMessage={t('ticket.composer.bodyRequired')}
            />
          )}
        </SegmentedTabs>
        <AttachmentPicker
          items={uploads.items}
          onSelect={uploads.add}
          onRemove={uploads.remove}
          onRetry={uploads.retry}
          accept={policy.extensions}
          buttonLabel={t('attach.button')}
          hint={t('attach.hint', { max: megabytesOf(policy.maxBytes) })}
          isDisabled={locked}
        />
        {submitted && uploads.isBlocking && <p className="text-xs font-semibold text-danger">{t('attach.waitForUploads')}</p>}
        <ErrorAlert error={send.error} />
        <div className="flex justify-end">
          <Button type="submit" variant={mode === 'internal' ? 'dark' : 'primary'} isPending={send.isPending} isDisabled={locked}>
            {mode === 'internal' ? t('ticket.composer.saveNote') : t('ticket.composer.sendReply')}
          </Button>
        </div>
      </form>
    </Card>
  )
}
