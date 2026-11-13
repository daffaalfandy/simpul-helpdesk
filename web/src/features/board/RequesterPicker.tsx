import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ApiError, ErrorCode, type Requester } from '../../api/client'
import { Button } from '../../components/Button'
import { ErrorAlert } from '../../components/ErrorAlert'
import { SearchSelect } from '../../components/SearchSelect'
import { TextInput } from '../../components/TextInput'
import { useCreateRequester } from '../tickets/queries'

const EMAIL_AT = '@'

type RequesterPickerProps = {
  clientId: string | null
  requesters: ReadonlyArray<Requester>
  isLoading: boolean
  value: string | null
  onChange: (id: string | null) => void
  isInvalid: boolean
}

/**
 * Pick one of the client's requesters, or add someone new (they become an invited
 * requester; HANDOFF §4 "Log a ticket").
 */
export function RequesterPicker({ clientId, requesters, isLoading, value, onChange, isInvalid }: RequesterPickerProps) {
  const { t } = useTranslation()
  const [draft, setDraft] = useState<{ name: string; email: string } | null>(null)

  if (draft && clientId) {
    return (
      <NewRequester
        key={clientId}
        clientId={clientId}
        initial={draft}
        onAdded={(r) => {
          setDraft(null)
          onChange(r.id)
        }}
        onCancel={() => setDraft(null)}
      />
    )
  }
  return (
    <SearchSelect
      key={clientId ?? ''}
      label={t('agent.logTicket.requester')}
      placeholder={t('agent.logTicket.requesterPlaceholder')}
      description={clientId ? undefined : t('agent.logTicket.chooseClientFirst')}
      emptyText={isLoading ? t('agent.logTicket.loadingRequesters') : t('agent.logTicket.noRequesters')}
      options={requesters.map((r) => ({ id: r.id, label: r.name, description: r.email, searchText: `${r.name} ${r.email}` }))}
      value={value}
      onChange={onChange}
      isDisabled={!clientId}
      isRequired
      isInvalid={isInvalid}
      errorMessage={t('agent.logTicket.errors.requester')}
      action={{
        label: (query) => t('agent.logTicket.addRequester', { query }),
        onAction: (query) => setDraft(query.includes(EMAIL_AT) ? { name: '', email: query } : { name: query, email: '' }),
      }}
    />
  )
}

type NewRequesterProps = {
  clientId: string
  initial: { name: string; email: string }
  onAdded: (r: Requester) => void
  onCancel: () => void
}

function NewRequester({ clientId, initial, onAdded, onCancel }: NewRequesterProps) {
  const { t } = useTranslation()
  const create = useCreateRequester(clientId)
  const [name, setName] = useState(initial.name)
  const [email, setEmail] = useState(initial.email)
  const [tried, setTried] = useState(false)
  const emailTaken = create.error instanceof ApiError && create.error.code === ErrorCode.EmailTaken
  const nameMissing = name.trim() === ''
  const emailInvalid = !email.trim().includes(EMAIL_AT)

  const add = () => {
    setTried(true)
    if (nameMissing || emailInvalid) return
    create.mutate({ name: name.trim(), email: email.trim() }, { onSuccess: onAdded })
  }

  return (
    <fieldset className="flex flex-col gap-3 rounded-xl border border-border p-3">
      <legend className="px-1 text-sm font-bold text-ink">{t('agent.logTicket.newRequester')}</legend>
      <div className="grid gap-3 sm:grid-cols-2">
        <TextInput
          label={t('agent.logTicket.newName')}
          value={name}
          onChange={setName}
          isInvalid={tried && nameMissing}
          errorMessage={t('agent.logTicket.errors.name')}
        />
        <TextInput
          label={t('agent.logTicket.newEmail')}
          type="email"
          value={email}
          onChange={(v) => {
            setEmail(v)
            create.reset()
          }}
          isInvalid={(tried && emailInvalid) || emailTaken}
          errorMessage={emailTaken ? t('errors.email_taken') : t('agent.logTicket.errors.email')}
        />
      </div>
      <p className="text-xs text-muted">{t('agent.logTicket.newRequesterHelp')}</p>
      {!emailTaken && <ErrorAlert error={create.error} />}
      <div className="flex flex-wrap gap-2">
        <Button size="sm" onPress={add} isPending={create.isPending}>
          {t('agent.logTicket.addRequesterButton')}
        </Button>
        <Button size="sm" variant="ghost" onPress={onCancel}>
          {t('agent.logTicket.pickExisting')}
        </Button>
      </div>
    </fieldset>
  )
}
