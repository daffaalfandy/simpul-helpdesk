import { CircleCheck } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { ApiError, ErrorCode, type DodConfig, type Ticket, type TicketDod, type TicketDodItem } from '../../api/client'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { ConfirmDialog, FormDialog } from '../../components/Dialog'
import { ErrorAlert } from '../../components/ErrorAlert'
import { ExternalLink } from '../../components/ExternalLink'
import { OptionCard, OptionCardGroup } from '../../components/OptionCard'
import { TextArea } from '../../components/TextArea'
import { MAX_REASON_LENGTH } from '../../lib/constants'
import { DodConditions, DodPreview } from '../dod/DodConditions'
import { codePaused, dodSummary, forRepository } from '../dod/labels'
import { useSetDod } from './queries'
import { useTimestamp } from './time'

/** The two answers to "Which Definition of Done applies" (Ticket mock). */
type Choice = 'project' | 'ticket'

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
  const summary = (c: DodConfig) => dodSummary(t, forRepository(c, dod.hasRepository))

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
            <span className="text-xs text-muted">{summary(dod.projectConfig)}</span>
          </OptionCard>
          <OptionCard value="ticket">
            <span className="text-sm font-bold">{t('ticket.dod.custom')}</span>
            <span className="text-xs text-muted">
              {dod.source === 'ticket' ? summary(dod.config) : t('ticket.dod.customNew')}
            </span>
          </OptionCard>
        </OptionCardGroup>
      ) : (
        <p className="text-sm text-ink-2">
          <b>{t(dod.source === 'ticket' ? 'ticket.dod.custom' : 'ticket.dod.project')}</b>
          {t('common.separator')}
          {summary(dod.config)}
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
  const branch = dod.config.codeOnBranch.branch

  // Git ticks the code item with a commit; the client's answer to the test ticks the others.
  const how = (i: TicketDodItem): ReactNode => {
    if (i.item === 'code_on_branch') {
      if (i.doneAt && i.commit) {
        const sha = <span className="font-mono">{i.commit.sha}</span>
        return (
          <>
            {i.commit.url ? <ExternalLink href={i.commit.url}>{sha}</ExternalLink> : sha}{' '}
            {t('ticket.dod.how.reached', { branch, time: stamp(i.doneAt) })}
          </>
        )
      }
      if (i.doneAt) return t('ticket.dod.how.reached', { branch, time: stamp(i.doneAt) })
      if (ticket.status === 'resolved' || ticket.status === 'closed') return t('ticket.dod.how.codeMissed', { branch })
      return t('ticket.dod.how.waitingCode', { key: ticket.key, branch })
    }
    if (i.doneAt) {
      return i.doneBy
        ? t('ticket.dod.how.approvedBy', { name: i.doneBy.name, time: stamp(i.doneAt) })
        : t('ticket.dod.how.approvedAuto', { time: stamp(i.doneAt) })
    }
    const name = ticket.requester.name
    if (ticket.status !== 'in_uat') return t('ticket.dod.how.later', { name })
    const auto = ticket.uat?.autoApproveAt
    return auto ? t('ticket.dod.how.askedAuto', { name, date: day(auto) }) : t('ticket.dod.how.asked', { name })
  }

  return (
    <>
      {dod.items.length === 0 ? (
        <p className="text-sm text-ink-2">{t('ticket.dod.nothing')}</p>
      ) : (
        <ul aria-label={t('ticket.dod.checklist')} className="flex flex-col gap-2.5">
          {dod.items.map((i) => (
            <li key={i.item} className="flex items-start gap-2.5">
              {i.doneAt ? (
                <CircleCheck
                  role="img"
                  aria-label={t('ticket.dod.done')}
                  className="mt-0.5 size-(--icon-lg) shrink-0 fill-success text-on-primary"
                />
              ) : (
                <span
                  role="img"
                  aria-label={t('ticket.dod.notDone')}
                  className="mt-0.5 inline-block size-(--icon-lg) shrink-0 rounded-full border-2 border-dashed border-border-strong"
                />
              )}
              <span className="flex min-w-0 flex-col">
                <span className="text-sm font-bold">{t(`dod.check.${i.item}`, { branch })}</span>
                <span className="text-xs text-muted">{how(i)}</span>
              </span>
            </li>
          ))}
        </ul>
      )}
      {codePaused(dod.config, dod.hasRepository) && (
        <p className="text-xs text-muted">{t('ticket.dod.codePaused', { branch })}</p>
      )}
    </>
  )
}

/** What happens next, in one line, and whether the Definition of Done is only guidance. */
function Footer({ ticket, dod }: { ticket: Ticket; dod: TicketDod }) {
  const { t } = useTranslation()
  const { day } = useTimestamp()
  if (dod.items.length === 0) return null
  const clientCheck = dod.items.some((i) => i.item !== 'code_on_branch')
  let text = t('ticket.dod.foot.askClient')
  if (dod.met) text = t('ticket.dod.foot.met')
  else if (!clientCheck) text = t('ticket.dod.foot.code', { branch: dod.config.codeOnBranch.branch })
  else if (ticket.status === 'in_uat') {
    const auto = ticket.uat?.autoApproveAt
    text = auto ? t('ticket.dod.foot.waitingAuto', { date: day(auto) }) : t('ticket.dod.foot.waiting')
  }
  return (
    <div className="flex flex-col gap-1">
      <p className="text-xs text-muted">{text}</p>
      {!dod.mandatory && !dod.met && <p className="text-xs font-bold text-ink-2">{t('ticket.dod.foot.guidance')}</p>}
    </div>
  )
}

function CustomDodDialog({ ticket, dod, onClose }: { ticket: Ticket; dod: TicketDod; onClose: () => void }) {
  const { t } = useTranslation()
  const setDod = useSetDod(ticket.key)
  // Without a repository the code condition can't be saved on, so the form starts with it off.
  const [draft, setDraft] = useState<DodConfig>(() => forRepository(structuredClone(dod.config), dod.hasRepository))
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
      <DodConditions
        dod={draft}
        legend={t('ticket.dod.dialog.conditions')}
        hasRepository={dod.hasRepository}
        branchInvalid={setDod.error instanceof ApiError && setDod.error.code === ErrorCode.InvalidBranch}
        onChange={setDraft}
      />
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
