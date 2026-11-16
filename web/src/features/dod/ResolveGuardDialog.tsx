import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import type { DodItem } from '../../api/client'
import { ConfirmDialog } from '../../components/Dialog'
import { MAX_REASON_LENGTH } from '../../lib/constants'
import { itemsPhrase } from './labels'

type ResolveGuardDialogProps = {
  ticketKey: string
  /** What the Definition of Done still needs; empty when the caller only knows it isn't met. */
  pending: readonly DodItem[]
  /** The ticket has a Definition of Done of its own. */
  custom?: boolean
  onCancel: () => void
  onConfirm: (reason: string) => void
  isPending?: boolean
  error?: ReactNode
}

/**
 * The guard before resolving a ticket whose Definition of Done isn't met (Main mock): it names
 * what is missing and needs a reason, which goes to the audit log and the supervisors. The board
 * drop, the status dropdown and "Resolve without DoD…" all open it.
 */
export function ResolveGuardDialog({ ticketKey, pending, custom, onCancel, onConfirm, isPending, error }: ResolveGuardDialogProps) {
  const { t, i18n } = useTranslation()
  const items = itemsPhrase(t, pending, i18n.language)
  const body =
    pending.length === 0
      ? t('ticket.resolveGuard.bodyUnknown')
      : t(custom ? 'ticket.resolveGuard.bodyCustom' : 'ticket.resolveGuard.body', { items })
  return (
    <ConfirmDialog
      isOpen
      onOpenChange={(open) => !open && onCancel()}
      tone="danger"
      title={t('ticket.resolveGuard.title', { key: ticketKey })}
      reasonLabel={t('ticket.resolveGuard.reason')}
      reasonPlaceholder={t('ticket.resolveGuard.placeholder')}
      reasonMaxLength={MAX_REASON_LENGTH}
      cancelLabel={t('ticket.resolveGuard.keep')}
      confirmLabel={t('ticket.resolveGuard.confirm')}
      isPending={isPending}
      error={error}
      onConfirm={onConfirm}
    >
      {body}
    </ConfirmDialog>
  )
}
