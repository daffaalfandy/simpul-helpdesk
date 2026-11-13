import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { Priority, Ticket, TicketChannel, TicketType } from '../../api/client'
import { attachmentPolicyQueryOptions, clientsQueryOptions, usePriorityNames } from '../../api/queries'
import { AttachmentPicker } from '../../components/AttachmentPicker'
import { Checkbox } from '../../components/Checkbox'
import { FormDialog } from '../../components/Dialog'
import { Dropdown } from '../../components/Dropdown'
import { ErrorAlert } from '../../components/ErrorAlert'
import { OptionCard, OptionCardGroup } from '../../components/OptionCard'
import { TextArea } from '../../components/TextArea'
import { TextInput } from '../../components/TextInput'
import { megabytesOf } from '../../lib/files'
import { formatTarget } from '../../lib/formatDuration'
import { priorityLong } from '../tickets/labels'
import { clientRequestersQueryOptions, intakeQueryOptions, useCreateTicket } from '../tickets/queries'
import { targetFor } from '../tickets/sla'
import { useAttachmentUploads } from '../tickets/useAttachmentUploads'
import { RequesterPicker } from './RequesterPicker'

/** Agents log calls, emails and walk-ins; tickets from the portal arrive on their own. */
const CHANNELS = ['phone', 'email', 'whatsapp', 'in_person'] as const satisfies readonly TicketChannel[]
const DEFAULT_CHANNEL: TicketChannel = 'phone'
const DEFAULT_PRIORITY: Priority = 'P3'
const TYPES: readonly TicketType[] = ['incident', 'request']
const NO_CLIENT = ''

type LogTicketDialogProps = { onClose: () => void; onCreated: (ticket: Ticket) => void }

/** "Log a ticket for a client" (HANDOFF §4, Main.dc.html). */
export function LogTicketDialog({ onClose, onCreated }: LogTicketDialogProps) {
  const { t } = useTranslation()
  const names = usePriorityNames()
  const { data: clients } = useSuspenseQuery(clientsQueryOptions)
  const { data: policy } = useSuspenseQuery(attachmentPolicyQueryOptions)
  const create = useCreateTicket()
  const uploads = useAttachmentUploads(policy)

  const [clientId, setClientId] = useState<string | null>(clients.length === 1 ? (clients[0]?.id ?? null) : null)
  const [requesterId, setRequesterId] = useState<string | null>(null)
  const [channel, setChannel] = useState<TicketChannel>(DEFAULT_CHANNEL)
  const [priority, setPriority] = useState<Priority>(DEFAULT_PRIORITY)
  const [type, setType] = useState<TicketType>('incident')
  const [projectId, setProjectId] = useState<string | null>(null)
  const [itemId, setItemId] = useState<string | null>(null)
  const [subject, setSubject] = useState('')
  const [details, setDetails] = useState('')
  const [assignToMe, setAssignToMe] = useState(true)
  const [submitted, setSubmitted] = useState(false)

  const intake = useQuery({ ...intakeQueryOptions(clientId ?? NO_CLIENT), enabled: clientId !== null })
  const requesters = useQuery({ ...clientRequestersQueryOptions(clientId ?? NO_CLIENT), enabled: clientId !== null })
  const projects = intake.data?.projects ?? []
  const items = intake.data?.catalogItems ?? []
  const noProject = intake.data?.defaultProjectId === null
  // With one system there is nothing to choose.
  const chosenProject = projectId ?? (projects.length === 1 ? (projects[0]?.id ?? null) : null)
  // Incidents follow the chosen system's SLA; service requests go to the default project.
  const slaProjectId = type === 'incident' ? chosenProject : (intake.data?.defaultProjectId ?? null)
  const targets = projects.find((p) => p.id === slaProjectId)?.targets

  const missing = {
    client: !clientId,
    requester: !requesterId,
    project: noProject,
    system: type === 'incident' && !noProject && !chosenProject,
    item: type === 'request' && items.length > 0 && !itemId,
    subject: subject.trim() === '',
    details: details.trim() === '',
  }

  const changeClient = (id: string) => {
    setClientId(id)
    setRequesterId(null)
    setProjectId(null)
    setItemId(null)
  }

  const priorityOptions = (['P1', 'P2', 'P3', 'P4'] as const).map((p) => {
    const target = targets?.find((x) => x.priority === p)
    const eff = target && targetFor(target, type)
    return {
      id: p,
      label: priorityLong(t, names, p, type),
      description: eff
        ? t(type === 'request' ? 'agent.logTicket.priorityOptionRequest' : 'agent.logTicket.priorityOption', {
            response: formatTarget(t, eff.firstResponse),
            fix: formatTarget(t, eff.resolution),
          })
        : undefined,
    }
  })

  const submit = () => {
    setSubmitted(true)
    if (Object.values(missing).some(Boolean) || !clientId || !requesterId || uploads.isBlocking) return
    create.mutate(
      {
        clientId,
        requesterId,
        channel,
        type,
        priority,
        subject: subject.trim(),
        description: details.trim(),
        projectId: type === 'incident' ? (chosenProject ?? undefined) : undefined,
        catalogItemId: type === 'request' ? (itemId ?? undefined) : undefined,
        attachmentIds: uploads.attachmentIds,
        assignToMe,
      },
      { onSuccess: onCreated },
    )
  }

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => !open && onClose()}
      title={t('agent.logTicket.title')}
      subtitle={t('agent.logTicket.subtitle')}
      submitLabel={t('agent.logTicket.submit')}
      onSubmit={submit}
      isPending={create.isPending}
      error={<ErrorAlert error={create.error ?? intake.error ?? requesters.error} />}
      footerStart={
        <Checkbox isSelected={assignToMe} onChange={setAssignToMe}>
          {t('agent.logTicket.assignToMe')}
        </Checkbox>
      }
    >
      <div className="grid gap-4 sm:grid-cols-2">
        <Dropdown
          label={t('agent.logTicket.client')}
          placeholder={t('common.choose')}
          options={clients.map((c) => ({
            id: c.id,
            label: c.name,
            description: c.projects.length > 0 ? c.projects.map((p) => p.name).join(', ') : t('agent.logTicket.clientNoProjects'),
          }))}
          value={clientId}
          onChange={changeClient}
          isRequired
          isDisabled={clients.length === 0}
          description={clients.length === 0 ? t('agent.logTicket.noClients') : undefined}
          isInvalid={(submitted && missing.client) || noProject}
          errorMessage={noProject ? t('agent.logTicket.errors.noProject') : t('agent.logTicket.errors.client')}
        />
        <RequesterPicker
          clientId={clientId}
          requesters={requesters.data ?? []}
          isLoading={clientId !== null && requesters.isPending}
          value={requesterId}
          onChange={setRequesterId}
          isInvalid={submitted && missing.requester}
        />
        <Dropdown
          label={t('agent.logTicket.channel')}
          options={CHANNELS.map((c) => ({ id: c, label: t(`ticket.channelOption.${c}`) }))}
          value={channel}
          onChange={setChannel}
        />
        <Dropdown label={t('agent.logTicket.priority')} options={priorityOptions} value={priority} onChange={setPriority} />
      </div>
      <OptionCardGroup label={t('agent.logTicket.type')} layout="chips" value={type} onChange={setType}>
        {TYPES.map((k) => (
          <OptionCard key={k} value={k} variant="chip">
            {k === 'incident' ? t('agent.logTicket.typeIncident') : t('agent.logTicket.typeRequest')}
          </OptionCard>
        ))}
      </OptionCardGroup>
      {type === 'incident' && projects.length > 0 && (
        <Dropdown
          label={t('portal.new.system')}
          placeholder={t('common.choose')}
          options={projects.map((p) => ({ id: p.id, label: p.name, description: p.description ?? undefined }))}
          value={chosenProject}
          onChange={setProjectId}
          isRequired
          isInvalid={submitted && missing.system}
          errorMessage={t('agent.logTicket.errors.system')}
        />
      )}
      {type === 'request' && items.length > 0 && (
        <Dropdown
          label={t('portal.new.item')}
          placeholder={t('common.choose')}
          options={items.map((i) => ({ id: i.id, label: i.name, description: i.description ?? undefined }))}
          value={itemId}
          onChange={setItemId}
          isRequired
          isInvalid={submitted && missing.item}
          errorMessage={t('agent.logTicket.errors.item')}
        />
      )}
      <TextInput
        label={t('agent.logTicket.subject')}
        placeholder={t('agent.logTicket.subjectPlaceholder')}
        value={subject}
        onChange={setSubject}
        isRequired
        isInvalid={submitted && missing.subject}
        errorMessage={t('agent.logTicket.errors.subject')}
      />
      <TextArea
        label={t('agent.logTicket.details')}
        placeholder={t('agent.logTicket.detailsPlaceholder')}
        rows={3}
        value={details}
        onChange={setDetails}
        isRequired
        isInvalid={submitted && missing.details}
        errorMessage={t('agent.logTicket.errors.details')}
      />
      <AttachmentPicker
        items={uploads.items}
        onSelect={uploads.add}
        onRemove={uploads.remove}
        onRetry={uploads.retry}
        accept={policy.extensions}
        buttonLabel={t('attach.button')}
        hint={t('attach.hint', { max: megabytesOf(policy.maxBytes) })}
      />
      {submitted && uploads.isBlocking && <p className="text-xs font-semibold text-danger">{t('attach.waitForUploads')}</p>}
    </FormDialog>
  )
}
