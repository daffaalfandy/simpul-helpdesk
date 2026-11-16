import { CircleCheck } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { DodConfig, Ticket, TicketDod, TicketDodItem } from '../../api/client'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { ConfirmDialog, FormDialog } from '../../components/Dialog'
import { ErrorAlert } from '../../components/ErrorAlert'
import { OptionCard, OptionCardGroup } from '../../components/OptionCard'
import { TextArea } from '../../components/TextArea'
import { MAX_REASON_LENGTH } from '../../lib/constants'
import { DodConditions, DodPreview } from '../dod/DodConditions'
import { dodSummary } from '../dod/labels'
import { useSetDod } from './queries'
import { useTimestamp } from './time'

/** The two answers to "Which Definition of Done applies" (Ticket mock). */
type Choice = 'project' | 'ticket'

/** Client items a requester answers by testing; the rest come from code (M7). */
const CLIENT_ITEMS: ReadonlySet<TicketDodItem['item']> = new Set(['uat_approved', 'requester_confirmed'])

/**
 * The ticket's Definition of Done (Ticket mock): which one applies, the checklist with how each
 * item was met, and what happens next. Agents may give the ticket its own when the project
 * allows it; supervisors and admins always may.
 */
export function TicketDodCard({ ticket, dod }: { ticket: Ticket; dod: TicketDod }) {
  const { t } = useTranslation()
  const [editing, setEditing] = useState(false)
  const [resetting, setResetting] = useState(false)
  const done = dod.items.filter((i) => i.doneAt).length
  const titleId = `dod-${ticket.key}`

  const choose = (c: Choice) => {
    if (c === dod.source) return
    if (c === 'ticket') setEditing(true)
    else setResetting(true)
  }

  return (
    <Card
      variant="compact"
      tone="emphasis"
      labelledBy={titleId}
      title={t('ticket.dod.title')}
      actions={
        dod.items.length > 0 && (
          <span className="text-xs font-bold text-muted">{t('ticket.dod.progress', { done, total: dod.items.length })}</span>
        )
      }
    >
      {dod.canOverride ? (
        <OptionCardGroup<Choice> label={t('ticket.dod.which')} value={dod.source} onChange={choose}>
          <OptionCard value="project">
            <span className="text-sm font-bold">{t('ticket.dod.project')}</span>
            <span className="text-xs text-muted">{dodSummary(t, dod.projectConfig)}</span>
          </OptionCard>
          <OptionCard value="ticket">
            <span className="text-sm font-bold">{t('ticket.dod.custom')}</span>
            <span className="text-xs text-muted">
              {dod.source === 'ticket' ? dodSummary(t, dod.config) : t('ticket.dod.customNew')}
            </span>
          </OptionCard>
        </OptionCardGroup>
      ) : (
        <p className="text-sm text-ink-2">
          <b>{t(dod.source === 'ticket' ? 'ticket.dod.custom' : 'ticket.dod.project')}</b>
          {t('common.separator')}
          {dodSummary(t, dod.config)}
        </p>
      )}
      {dod.source === 'ticket' && dod.overrideReason && (
        <p className="text-xs text-muted">{t('ticket.dod.reason', { reason: dod.overrideReason })}</p>
      )}
      {dod.canOverride && dod.source === 'ticket' && (
        <Button variant="ghost" size="sm" className="self-start" onPress={() => setEditing(true)}>
          {t('ticket.dod.change')}
        </Button>
      )}
      <Checklist ticket={ticket} dod={dod} />
      <Footer ticket={ticket} dod={dod} />
      {editing && <CustomDodDialog ticket={ticket} dod={dod} onClose={() => setEditing(false)} />}
      {resetting && <ResetDialog ticket={ticket} onClose={() => setResetting(false)} />}
    </Card>
  )
}

function Checklist({ ticket, dod }: { ticket: Ticket; dod: TicketDod }) {
  const { t } = useTranslation()
  const { stamp, day } = useTimestamp()
  if (dod.items.length === 0) return <p className="text-sm text-ink-2">{t('ticket.dod.nothing')}</p>

  const how = (i: TicketDodItem): string => {
    if (i.doneAt) {
      if (!CLIENT_ITEMS.has(i.item)) return t('ticket.dod.how.doneAt', { time: stamp(i.doneAt) })
      return i.doneBy
        ? t('ticket.dod.how.approvedBy', { name: i.doneBy.name, time: stamp(i.doneAt) })
        : t('ticket.dod.how.approvedAuto', { time: stamp(i.doneAt) })
    }
    if (!CLIENT_ITEMS.has(i.item)) return t('ticket.dod.how.code')
    const name = ticket.requester.name
    if (ticket.status !== 'in_uat') return t('ticket.dod.how.later', { name })
    const auto = ticket.uat?.autoApproveAt
    return auto ? t('ticket.dod.how.askedAuto', { name, date: day(auto) }) : t('ticket.dod.how.asked', { name })
  }

  return (
    <ul aria-label={t('ticket.dod.checklist')} className="flex flex-col gap-2.5">
      {dod.items.map((i) => (
        <li key={i.item} className="flex items-start gap-2.5">
          {i.doneAt ? (
            <CircleCheck aria-label={t('ticket.dod.done')} className="mt-0.5 size-(--icon-lg) shrink-0 fill-success text-on-primary" />
          ) : (
            <span
              role="img"
              aria-label={t('ticket.dod.notDone')}
              className="mt-0.5 inline-block size-(--icon-lg) shrink-0 rounded-full border-2 border-dashed border-border-strong"
            />
          )}
          <span className="flex min-w-0 flex-col">
            <span className="text-sm font-bold">{t(`dod.check.${i.item}`, { branch: dod.config.merged.branch })}</span>
            <span className="text-xs text-muted">{how(i)}</span>
          </span>
        </li>
      ))}
    </ul>
  )
}

/** What happens next, in one line. */
function Footer({ ticket, dod }: { ticket: Ticket; dod: TicketDod }) {
  const { t } = useTranslation()
  const { day } = useTimestamp()
  if (dod.items.length === 0) return null
  let text = t('ticket.dod.foot.askClient')
  if (dod.met) text = t('ticket.dod.foot.met')
  else if (ticket.status === 'in_uat') {
    const auto = ticket.uat?.autoApproveAt
    text = auto ? t('ticket.dod.foot.waitingAuto', { date: day(auto) }) : t('ticket.dod.foot.waiting')
  }
  return <p className="text-xs text-muted">{text}</p>
}

function CustomDodDialog({ ticket, dod, onClose }: { ticket: Ticket; dod: TicketDod; onClose: () => void }) {
  const { t } = useTranslation()
  const setDod = useSetDod(ticket.key)
  const [draft, setDraft] = useState<DodConfig>(() => structuredClone(dod.config))
  const [reason, setReason] = useState(dod.source === 'ticket' ? (dod.overrideReason ?? '') : '')
  const [tried, setTried] = useState(false)
  const missingReason = reason.trim() === ''

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => !open && onClose()}
      title={t('ticket.dod.dialog.title', { key: ticket.key })}
      subtitle={t('ticket.dod.dialog.subtitle')}
      submitLabel={t('ticket.dod.dialog.save')}
      isPending={setDod.isPending}
      error={<ErrorAlert error={setDod.error} />}
      onSubmit={() => {
        setTried(true)
        if (missingReason) return
        setDod.mutate({ dod: draft, reason: reason.trim() }, { onSuccess: onClose })
      }}
    >
      {/* A ticket's own Definition of Done can't ask for code until its project has a repository (M7). */}
      <DodConditions dod={draft} hasRepository={false} legend={t('ticket.dod.dialog.conditions')} onChange={setDraft} />
      <DodPreview dod={draft} />
      <TextArea
        label={t('ticket.dod.dialog.reason')}
        placeholder={t('ticket.dod.dialog.reasonPlaceholder')}
        value={reason}
        onChange={setReason}
        maxLength={MAX_REASON_LENGTH}
        isRequired
        isInvalid={tried && missingReason}
        errorMessage={t('errors.reason_required')}
      />
    </FormDialog>
  )
}

function ResetDialog({ ticket, onClose }: { ticket: Ticket; onClose: () => void }) {
  const { t } = useTranslation()
  const setDod = useSetDod(ticket.key)
  return (
    <ConfirmDialog
      isOpen
      onOpenChange={(open) => !open && onClose()}
      title={t('ticket.dod.reset.title')}
      confirmLabel={t('ticket.dod.reset.confirm')}
      isPending={setDod.isPending}
      error={<ErrorAlert error={setDod.error} />}
      onConfirm={() => setDod.mutate({ dod: null }, { onSuccess: onClose })}
    >
      {t('ticket.dod.reset.body', { key: ticket.key, project: ticket.project.name })}
    </ConfirmDialog>
  )
}
