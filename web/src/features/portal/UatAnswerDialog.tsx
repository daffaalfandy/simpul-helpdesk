import { useTranslation } from 'react-i18next'

import type { Ticket, UatAnswer } from '../../api/client'
import { ConfirmDialog } from '../../components/Dialog'
import { ErrorAlert } from '../../components/ErrorAlert'
import { MAX_MESSAGE_LENGTH } from '../../lib/constants'
import { useAnswerUat } from './queries'

type UatAnswerDialogProps = {
  ticketKey: string
  subject: string
  answer: UatAnswer
  onClose: () => void
  onAnswered: (ticket: Ticket) => void
}

/**
 * The client's answer to a test (PortalHome and PortalTicket mocks): approving resolves the
 * ticket; "Still a problem" needs what went wrong, which reaches the team as the client's reply.
 */
export function UatAnswerDialog({ ticketKey, subject, answer, onClose, onAnswered }: UatAnswerDialogProps) {
  const { t } = useTranslation()
  const send = useAnswerUat(ticketKey)
  const approve = answer === 'approve'
  return (
    <ConfirmDialog
      isOpen
      onOpenChange={(open) => !open && onClose()}
      tone={approve ? 'success' : 'danger'}
      title={t(approve ? 'portal.uat.approveDialog.title' : 'portal.uat.rejectDialog.title', { key: ticketKey })}
      confirmLabel={t(approve ? 'portal.uat.approveDialog.confirm' : 'portal.uat.rejectDialog.confirm')}
      reasonLabel={approve ? undefined : t('portal.uat.rejectDialog.reason')}
      reasonPlaceholder={approve ? undefined : t('portal.uat.rejectDialog.placeholder')}
      reasonMaxLength={MAX_MESSAGE_LENGTH}
      isPending={send.isPending}
      error={<ErrorAlert error={send.error} />}
      onConfirm={(message) =>
        send.mutate(approve ? { answer } : { answer, message }, { onSuccess: (ticket) => onAnswered(ticket) })
      }
    >
      {t(approve ? 'portal.uat.approveDialog.body' : 'portal.uat.rejectDialog.body', { title: subject })}
    </ConfirmDialog>
  )
}
