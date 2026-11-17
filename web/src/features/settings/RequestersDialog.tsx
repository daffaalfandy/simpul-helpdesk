import { useQuery } from '@tanstack/react-query'
import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { Client, Requester } from '../../api/client'
import { Alert } from '../../components/Alert'
import { ConfirmDialog, FormDialog } from '../../components/Dialog'
import { TextInput } from '../../components/TextInput'
import { MemberActions, MemberStatusPill } from './MemberControls'
import { clientRequestersAdminQueryOptions, useInviteRequester, useMemberAction, type MemberAction } from './queries'
import { ErrorAlert } from './shared'

/**
 * A client's requesters (Settings › Clients): only administrators invite them (owner, 17 Nov).
 * The list sits above the invite form; resend, suspend and reactivate work as for the team.
 */
export function RequestersDialog({ client, onClose }: { client: Client; onClose: () => void }) {
  const { t } = useTranslation()
  const requesters = useQuery(clientRequestersAdminQueryOptions(client.id))
  const invite = useInviteRequester(client.id)
  const action = useMemberAction()
  const [flash, setFlash] = useState<string | null>(null)
  const [suspending, setSuspending] = useState<Requester | null>(null)
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [jobTitle, setJobTitle] = useState('')
  const [tried, setTried] = useState(false)
  const inviteId = useId()

  const act = (r: Requester, a: MemberAction) => {
    setFlash(null)
    action.mutate(
      { userId: r.id, action: a },
      {
        onSuccess: () => {
          setSuspending(null)
          setFlash(t(`settings.members.done.${a}`, { name: r.name }))
        },
      },
    )
  }
  const submit = () => {
    setTried(true)
    setFlash(null)
    if (name.trim() === '' || email.trim() === '') return
    const body = {
      name: name.trim(),
      email: email.trim(),
      jobTitle: jobTitle.trim() || null,
    }
    invite.mutate(body, {
      onSuccess: () => {
        setName('')
        setEmail('')
        setJobTitle('')
        setTried(false)
        setFlash(t('settings.members.invited', { email: body.email }))
      },
    })
  }

  return (
    <>
      <FormDialog
        isOpen
        onOpenChange={(open) => !open && onClose()}
        title={t('settings.clients.requesters.title', { name: client.name })}
        subtitle={t('settings.clients.requesters.subtitle')}
        submitLabel={t('settings.members.inviteSubmit')}
        isPending={invite.isPending}
        onSubmit={submit}
        error={<ErrorAlert error={invite.error} />}
      >
        {flash && <Alert tone="success">{flash}</Alert>}
        <ErrorAlert error={requesters.error ?? (suspending ? null : action.error)} />
        {requesters.isPending ? (
          <p className="text-sm text-muted">{t('common.loading')}</p>
        ) : requesters.data?.length === 0 ? (
          <p className="text-sm text-muted">{t('settings.clients.requesters.empty')}</p>
        ) : (
          <ul
            aria-label={t('settings.clients.requesters.listLabel', {
              name: client.name,
            })}
            className="flex flex-col divide-y divide-border-subtle"
          >
            {requesters.data?.map((r) => (
              <li key={r.id} className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 py-2.5">
                <span className="flex min-w-0 flex-col">
                  <span className="font-bold">{r.name}</span>
                  <span className="truncate text-sm text-muted">{[r.email, r.jobTitle].filter(Boolean).join(t('common.separator'))}</span>
                </span>
                <span className="flex items-center gap-2">
                  <MemberStatusPill status={r.status} />
                  <MemberActions name={r.name} status={r.status} onAction={(a) => (a === 'suspend' ? setSuspending(r) : act(r, a))} />
                </span>
              </li>
            ))}
          </ul>
        )}
        <div role="group" aria-labelledby={inviteId} className="flex flex-col gap-3 border-t border-border-subtle pt-4">
          <div className="flex flex-col gap-1">
            <h3 id={inviteId} className="text-md font-extrabold">
              {t('settings.clients.requesters.inviteTitle')}
            </h3>
            <p className="text-sm text-muted">{t('settings.clients.requesters.inviteHint')}</p>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <TextInput
              label={t('settings.members.name')}
              value={name}
              onChange={setName}
              isInvalid={tried && name.trim() === ''}
              errorMessage={t('settings.members.nameRequired')}
            />
            <TextInput
              label={t('settings.members.email')}
              type="email"
              value={email}
              onChange={setEmail}
              isInvalid={tried && email.trim() === ''}
              errorMessage={t('settings.members.emailRequired')}
            />
          </div>
          <TextInput
            label={t('settings.members.jobTitle')}
            value={jobTitle}
            onChange={setJobTitle}
            description={t('settings.clients.requesters.jobTitleHint')}
          />
        </div>
      </FormDialog>
      {/* Outside the form: a portal's submit would bubble to the invite form through React. */}
      {suspending && (
        <ConfirmDialog
          isOpen
          onOpenChange={(open) => {
            if (open) return
            setSuspending(null)
            action.reset()
          }}
          tone="danger"
          title={t('settings.members.confirm.suspend.title', { name: suspending.name })}
          confirmLabel={t('settings.members.actions.suspend')}
          isPending={action.isPending}
          error={<ErrorAlert error={action.error} />}
          onConfirm={() => act(suspending, 'suspend')}
        >
          {t('settings.clients.requesters.suspendBody')}
        </ConfirmDialog>
      )}
    </>
  )
}
