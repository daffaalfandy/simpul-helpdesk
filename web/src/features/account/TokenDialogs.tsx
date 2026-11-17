import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { CreatedPersonalToken, PersonalToken, PersonalTokenInput, TokenAccess } from '../../api/client'
import { ConfirmDialog, FormDialog } from '../../components/Dialog'
import { CopyField } from '../../components/CopyField'
import { Dropdown } from '../../components/Dropdown'
import { ErrorAlert } from '../../components/ErrorAlert'
import { TextInput } from '../../components/TextInput'
import { DEFAULT_TOKEN_LIFETIME_DAYS, MAX_TOKEN_NAME_LENGTH, TOKEN_LIFETIME_DAYS } from '../../lib/constants'
import { useCreateToken, useRevokeToken } from './queries'
import { snippetsFor } from './snippets'

const ACCESS: readonly TokenAccess[] = ['read', 'write']
/** A new token starts read only: the least it needs to read tickets. */
const DEFAULT_ACCESS: TokenAccess = 'read'

type Lifetime = PersonalTokenInput['expiresInDays']
const lifetimeKey = (days: Lifetime) => String(days)

export function CreateTokenDialog({
  orgName,
  onClose,
  onCreated,
}: {
  orgName: string
  onClose: () => void
  onCreated: (result: CreatedPersonalToken) => void
}) {
  const { t } = useTranslation()
  const create = useCreateToken()
  const [name, setName] = useState('')
  const [access, setAccess] = useState<TokenAccess>(DEFAULT_ACCESS)
  const [days, setDays] = useState<Lifetime>(DEFAULT_TOKEN_LIFETIME_DAYS)
  const [tried, setTried] = useState(false)
  const missingName = name.trim() === ''

  const submit = () => {
    setTried(true)
    if (missingName) return
    create.mutate({ name: name.trim(), access, expiresInDays: days }, { onSuccess: onCreated })
  }

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => !open && onClose()}
      title={t('account.mcp.createTitle')}
      subtitle={t('account.mcp.createSubtitle', { org: orgName })}
      submitLabel={t('account.mcp.createSubmit')}
      onSubmit={submit}
      isPending={create.isPending}
      error={<ErrorAlert error={create.error} />}
    >
      <TextInput
        label={t('account.mcp.nameLabel')}
        description={t('account.mcp.nameHint')}
        value={name}
        onChange={setName}
        maxLength={MAX_TOKEN_NAME_LENGTH}
        isRequired
        autoFocus
        isInvalid={tried && missingName}
        errorMessage={t('account.mcp.nameRequired')}
      />
      <Dropdown<TokenAccess>
        label={t('account.mcp.accessLabel')}
        options={ACCESS.map((a) => ({ id: a, label: t(`account.mcp.access.${a}.short`), description: t(`account.mcp.access.${a}.long`) }))}
        value={access}
        onChange={setAccess}
      />
      <Dropdown<string>
        label={t('account.mcp.expiresLabel')}
        options={TOKEN_LIFETIME_DAYS.map((d) => ({ id: lifetimeKey(d), label: t(`account.mcp.lifetime.${d}`) }))}
        value={lifetimeKey(days)}
        onChange={(key) => setDays(Number(key) as Lifetime)}
      />
    </FormDialog>
  )
}

/** The secret, shown once, with the client settings filled in. */
export function TokenCreatedDialog({ mcpUrl, result, onClose }: { mcpUrl: string; result: CreatedPersonalToken; onClose: () => void }) {
  const { t } = useTranslation()
  const snippets = snippetsFor(mcpUrl, result.secret)
  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => !open && onClose()}
      title={t('account.mcp.createdTitle', { name: result.token.name })}
      subtitle={t(`account.mcp.access.${result.token.access}.long`)}
      submitLabel={t('account.mcp.createdDone')}
      onSubmit={onClose}
      hideCancel
    >
      <dl className="flex flex-col gap-4">
        <CopyField label={t('account.mcp.token')} value={result.secret} hint={t('account.mcp.secretOnce')} />
        <CopyField label={t('account.mcp.claudeCode')} value={snippets.claudeCode} />
        <CopyField label={t('account.mcp.otherClients')} value={snippets.json} block />
      </dl>
    </FormDialog>
  )
}

export function RevokeTokenDialog({ token, onClose, onRevoked }: { token: PersonalToken; onClose: () => void; onRevoked: () => void }) {
  const { t } = useTranslation()
  const revoke = useRevokeToken()
  return (
    <ConfirmDialog
      isOpen
      onOpenChange={(open) => !open && onClose()}
      tone="danger"
      title={t('account.mcp.revokeTitle', { name: token.name })}
      confirmLabel={t('account.mcp.revokeConfirm')}
      onConfirm={() => revoke.mutate(token.id, { onSuccess: onRevoked })}
      isPending={revoke.isPending}
      error={<ErrorAlert error={revoke.error} />}
    >
      {t('account.mcp.revokeBody')}
    </ConfirmDialog>
  )
}
