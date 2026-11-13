import { useSuspenseQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { CircleCheck, FolderX } from 'lucide-react'
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

import type { Intake, Priority, ProjectTarget, Ticket, TicketType } from '../../api/client'
import { attachmentPolicyQueryOptions, usePriorityNames } from '../../api/queries'
import { AttachmentPicker } from '../../components/AttachmentPicker'
import { PriorityBadge } from '../../components/Badges'
import { BackLink } from '../../components/BackLink'
import { Button, buttonClasses } from '../../components/Button'
import { Card } from '../../components/Card'
import { Dropdown } from '../../components/Dropdown'
import { EmptyState } from '../../components/EmptyState'
import { ErrorAlert } from '../../components/ErrorAlert'
import { OptionCard, OptionCardGroup } from '../../components/OptionCard'
import { TextArea } from '../../components/TextArea'
import { TextInput } from '../../components/TextInput'
import { DATE_LOCALE } from '../../i18n/locale'
import { paths } from '../../lib/constants'
import { zoneAbbreviation } from '../../lib/dates'
import { megabytesOf } from '../../lib/files'
import { formatTarget } from '../../lib/formatDuration'
import { weekdayRuns } from '../../lib/weekdays'
import { priorityLong } from '../tickets/labels'
import { intakeQueryOptions, useCreateTicket } from '../tickets/queries'
import { targetFor } from '../tickets/sla'
import { useAttachmentUploads } from '../tickets/useAttachmentUploads'
import { PriorityCards } from './PriorityCards'

const TYPES: readonly TicketType[] = ['incident', 'request']
const list = new Intl.ListFormat(DATE_LOCALE, { type: 'conjunction' })

/** "Report an issue" (HANDOFF §4, NewTicket.dc.html). */
export function ReportIssuePage() {
  const [created, setCreated] = useState<Ticket | null>(null)
  const [formKey, setFormKey] = useState(0)
  return (
    <div className="mx-auto flex w-full max-w-form flex-col gap-4">
      {created ? (
        <ReportSuccess
          ticket={created}
          onAnother={() => {
            setCreated(null)
            setFormKey((k) => k + 1)
          }}
        />
      ) : (
        <ReportForm key={formKey} onCreated={setCreated} />
      )}
    </div>
  )
}

/**
 * "08:00 – 17:00 WIB" and "Monday – Friday" from the organisation's business hours, and which
 * priorities the project handles around the clock.
 */
function useWorkingHours(intake: Intake, targets: ReadonlyArray<ProjectTarget> | null) {
  const { t } = useTranslation()
  const h = intake.businessHours
  const hours = t('portal.new.workingHours', {
    range: t('common.range', { from: h.opens, to: h.closes }),
    zone: zoneAbbreviation(h.timezone),
  })
  const days = list.format(
    weekdayRuns(h.weekdays).map((r) =>
      r.first === r.last
        ? t(`weekday.long.${r.first}`)
        : t('common.range', { from: t(`weekday.long.${r.first}`), to: t(`weekday.long.${r.last}`) }),
    ),
  )
  const always = (targets ?? []).filter((x) => x.calendar === 'always').map((x) => x.priority)
  return { hours, days, always }
}

function ReportForm({ onCreated }: { onCreated: (t: Ticket) => void }) {
  const { data: intake } = useSuspenseQuery(intakeQueryOptions(null))
  // Every ticket belongs to a project; without one the client can't report yet.
  if (intake.defaultProjectId === null) return <NoProject />
  return <ReportFormFields intake={intake} onCreated={onCreated} />
}

function NoProject() {
  const { t } = useTranslation()
  return (
    <>
      <BackLink to={paths.portal}>{t('portal.new.back')}</BackLink>
      <EmptyState icon={FolderX} title={t('portal.new.noProject.title')}>
        {t('portal.new.noProject.body')}
      </EmptyState>
    </>
  )
}

function ReportFormFields({ intake, onCreated }: { intake: Intake; onCreated: (t: Ticket) => void }) {
  const { t } = useTranslation()
  const { data: policy } = useSuspenseQuery(attachmentPolicyQueryOptions)
  const create = useCreateTicket()
  const uploads = useAttachmentUploads(policy)

  const [type, setType] = useState<TicketType>('incident')
  const [subject, setSubject] = useState('')
  // With one system there is nothing to choose.
  const [projectId, setProjectId] = useState<string | null>(intake.projects.length === 1 ? (intake.projects[0]?.id ?? null) : null)
  const [itemId, setItemId] = useState<string | null>(null)
  const [priority, setPriority] = useState<Priority | null>(null)
  const [details, setDetails] = useState('')
  const [submitted, setSubmitted] = useState(false)

  // Incidents follow the chosen system's SLA; service requests go to the default project.
  const slaProjectId = type === 'incident' ? projectId : intake.defaultProjectId
  const targets = intake.projects.find((p) => p.id === slaProjectId)?.targets ?? null
  const working = useWorkingHours(intake, targets)
  const missing = {
    subject: subject.trim() === '',
    system: type === 'incident' && !projectId,
    item: type === 'request' && intake.catalogItems.length > 0 && !itemId,
    priority: priority === null,
    details: details.trim() === '',
  }
  const target = targets?.find((x) => x.priority === priority)
  const eff = target ? targetFor(target, type) : null

  const submit = (e: FormEvent) => {
    e.preventDefault()
    setSubmitted(true)
    if (Object.values(missing).some(Boolean) || !priority || uploads.isBlocking) return
    create.mutate(
      {
        type,
        priority,
        subject: subject.trim(),
        description: details.trim(),
        projectId: type === 'incident' ? (projectId ?? undefined) : undefined,
        catalogItemId: type === 'request' ? (itemId ?? undefined) : undefined,
        attachmentIds: uploads.attachmentIds,
      },
      { onSuccess: onCreated },
    )
  }

  return (
    <>
      <BackLink to={paths.portal}>{t('portal.new.back')}</BackLink>
      <Card>
        <form onSubmit={submit} noValidate className="flex flex-col gap-6">
          <div className="flex flex-col gap-1">
            <h1 className="text-2xl font-extrabold tracking-title">{t('portal.new.title')}</h1>
            <p className="text-md text-muted">
              {t('portal.new.hours', { hours: working.hours, days: working.days })}{' '}
              {working.always.length > 0 && t('portal.new.hoursAlways', { priorities: list.format(working.always) })}
            </p>
          </div>

          <OptionCardGroup label={t('portal.new.typeLegend')} value={type} onChange={setType}>
            {TYPES.map((k) => (
              <OptionCard key={k} value={k}>
                <span className="font-extrabold">{t(`portal.new.${k}.title`)}</span>
                <span className="text-sm text-muted">{t(`portal.new.${k}.description`)}</span>
              </OptionCard>
            ))}
          </OptionCardGroup>

          <TextInput
            label={t('portal.new.subject')}
            placeholder={t('portal.new.subjectPlaceholder')}
            size="lg"
            value={subject}
            onChange={setSubject}
            isRequired
            maxLength={MAX_SUBJECT_LENGTH}
            isInvalid={submitted && missing.subject}
            errorMessage={t('portal.new.errors.subject')}
          />

          {type === 'incident' && (
            <Dropdown
              label={t('portal.new.system')}
              size="lg"
              placeholder={t('common.choose')}
              options={intake.projects.map((p) => ({ id: p.id, label: p.name, description: p.description ?? undefined }))}
              value={projectId}
              onChange={setProjectId}
              isRequired
              isInvalid={submitted && missing.system}
              errorMessage={t('portal.new.errors.system')}
            />
          )}
          {type === 'request' && intake.catalogItems.length > 0 && (
            <Dropdown
              label={t('portal.new.item')}
              size="lg"
              placeholder={t('common.choose')}
              options={intake.catalogItems.map((i) => ({ id: i.id, label: i.name, description: i.description ?? undefined }))}
              value={itemId}
              onChange={setItemId}
              isRequired
              isInvalid={submitted && missing.item}
              errorMessage={t('portal.new.errors.item')}
            />
          )}

          <PriorityCards
            targets={targets}
            type={type}
            workingHours={working.hours}
            value={priority}
            onChange={setPriority}
            isInvalid={submitted && missing.priority}
          />

          <TextArea
            label={t('portal.new.details')}
            placeholder={t('portal.new.detailsPlaceholder')}
            size="lg"
            value={details}
            onChange={setDetails}
            isRequired
            isInvalid={submitted && missing.details}
            errorMessage={t('portal.new.errors.details')}
          />

          <AttachmentPicker
            items={uploads.items}
            onSelect={uploads.add}
            onRemove={uploads.remove}
            onRetry={uploads.retry}
            accept={policy.extensions}
            buttonLabel={t('attach.buttonLong')}
            hint={t('attach.hint', { max: megabytesOf(policy.maxBytes) })}
            variant="dashed"
          />
          {submitted && uploads.isBlocking && <p className="text-xs font-semibold text-danger">{t('attach.waitForUploads')}</p>}

          <ErrorAlert error={create.error} />
          <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl bg-tint px-4 py-3">
            <p className="flex items-center gap-2 text-sm font-semibold" aria-live="polite">
              {priority && eff ? (
                <>
                  <PriorityBadge priority={priority} />
                  {t(type === 'request' ? 'portal.new.summaryRequest' : 'portal.new.summaryIncident', {
                    response: formatTarget(t, eff.firstResponse),
                    fix: formatTarget(t, eff.resolution),
                  })}
                </>
              ) : (
                <span className="text-muted">{t('portal.new.summaryChoose')}</span>
              )}
            </p>
            <Button type="submit" size="lg" isPending={create.isPending}>
              {t('portal.new.submit')}
            </Button>
          </div>
        </form>
      </Card>
    </>
  )
}

/** Must match the API (tickets.MaxSubjectLen). */
const MAX_SUBJECT_LENGTH = 200

function ReportSuccess({ ticket, onAnother }: { ticket: Ticket; onAnother: () => void }) {
  const { t } = useTranslation()
  const names = usePriorityNames()
  const heading = useRef<HTMLHeadingElement>(null)
  useEffect(() => heading.current?.focus(), [])
  const response = formatTarget(t, ticket.sla.firstResponse)
  return (
    <Card>
      <div className="flex flex-col items-center gap-4 py-6 text-center">
        <CircleCheck aria-hidden="true" className="size-12 text-success" />
        <h1 ref={heading} tabIndex={-1} className="text-2xl font-extrabold tracking-title outline-none">
          {t('portal.new.success.title', { key: ticket.key })}
        </h1>
        <p className="max-w-xl text-md text-muted">
          {t('portal.new.success.body', { priority: priorityLong(t, names, ticket.priority, ticket.type), response })}
        </p>
        <div className="flex flex-wrap justify-center gap-3">
          <Button variant="outline" onPress={onAnother}>
            {t('portal.new.success.another')}
          </Button>
          <Link to={paths.portalTicket} params={{ key: ticket.key }} className={buttonClasses('primary')}>
            {t('portal.new.success.view')}
          </Link>
        </div>
      </div>
    </Card>
  )
}
