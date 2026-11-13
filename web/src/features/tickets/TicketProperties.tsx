import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { Priority, Ticket, TicketStatus } from '../../api/client'
import { Card } from '../../components/Card'
import { DescriptionList } from '../../components/DescriptionList'
import { ConfirmDialog } from '../../components/Dialog'
import { Dropdown, type DropdownOption } from '../../components/Dropdown'
import { ErrorAlert } from '../../components/ErrorAlert'
import { agentStatus, priorityLong } from './labels'
import { clientAssigneesQueryOptions, useSetAssignee, useSetPriority, useSetStatus } from './queries'
import { hasHint } from './status'

const PRIORITIES: readonly Priority[] = ['P1', 'P2', 'P3', 'P4']
/** Dropdown id for "nobody"; sent to the API as null. */
const UNASSIGNED = 'unassigned'

/** Status, assignee and priority (with a reason), plus the ticket's read-only details. */
export function TicketProperties({ ticket }: { ticket: Ticket }) {
  const { t } = useTranslation()
  const setStatus = useSetStatus(ticket.key)
  const setAssignee = useSetAssignee(ticket.key)
  const setPriority = useSetPriority(ticket.key)
  const { data: assignees } = useQuery(clientAssigneesQueryOptions(ticket.client.id))
  const [pendingPriority, setPendingPriority] = useState<Priority | null>(null)
  const locked = ticket.allowedStatuses.length === 0

  const choices = [ticket.status, ...ticket.allowedStatuses.filter((s) => s !== ticket.status)]
  const statusOptions: DropdownOption<TicketStatus>[] = choices.map((s) => ({
    id: s,
    label: agentStatus(t, s),
    description: hasHint(s) ? t(`ticket.status.agentHint.${s}`) : undefined,
  }))

  const people = assignees ?? []
  const assigneeOptions: DropdownOption<string>[] = [
    { id: UNASSIGNED, label: t('ticket.unassigned') },
    ...people.map((a) => ({
      id: a.id,
      label: a.name,
      description: a.jobTitle
        ? t('ticket.assigneeOption', { title: a.jobTitle, open: t('ticket.openTickets', { count: a.openTicketCount }) })
        : t('ticket.openTickets', { count: a.openTicketCount }),
    })),
  ]
  if (ticket.assignee && !people.some((a) => a.id === ticket.assignee?.id)) {
    assigneeOptions.push({ id: ticket.assignee.id, label: ticket.assignee.name })
  }

  const detail = ticket.type === 'request' ? ticket.catalogItem : ticket.system
  const details = [
    { term: t('ticket.fields.client'), value: ticket.client.name },
    { term: t('ticket.fields.requester'), value: ticket.requester.name },
    ...(detail
      ? [{ term: ticket.type === 'request' ? t('ticket.fields.catalogItem') : t('ticket.fields.system'), value: detail.name }]
      : []),
    { term: t('ticket.fields.channel'), value: t(`ticket.channelOption.${ticket.channel}`) },
  ]

  return (
    <Card as="h2" title={t('ticket.properties')}>
      <Dropdown
        label={t('ticket.fields.status')}
        options={statusOptions}
        value={ticket.status}
        onChange={(s) => s !== ticket.status && setStatus.mutate(s)}
        isDisabled={locked || setStatus.isPending}
      />
      <ErrorAlert error={setStatus.error} />
      <Dropdown
        label={t('ticket.fields.assignee')}
        options={assigneeOptions}
        value={ticket.assignee?.id ?? UNASSIGNED}
        onChange={(id) => setAssignee.mutate(id === UNASSIGNED ? null : id)}
        isDisabled={locked || setAssignee.isPending}
      />
      <ErrorAlert error={setAssignee.error} />
      <Dropdown
        label={t('ticket.fields.priority')}
        options={PRIORITIES.map((p) => ({ id: p, label: priorityLong(t, p, ticket.type) }))}
        value={ticket.priority}
        onChange={(p) => p !== ticket.priority && setPendingPriority(p)}
        isDisabled={locked || setPriority.isPending}
      />
      <DescriptionList items={details} />
      {pendingPriority && (
        <ConfirmDialog
          isOpen
          onOpenChange={(open) => {
            if (!open) {
              setPendingPriority(null)
              setPriority.reset()
            }
          }}
          title={t('ticket.priorityChange.title', { priority: priorityLong(t, pendingPriority, ticket.type) })}
          reasonLabel={t('ticket.priorityChange.reason')}
          confirmLabel={t('ticket.priorityChange.confirm')}
          isPending={setPriority.isPending}
          error={<ErrorAlert error={setPriority.error} />}
          onConfirm={(reason) =>
            setPriority.mutate({ priority: pendingPriority, reason }, { onSuccess: () => setPendingPriority(null) })
          }
        >
          {t('ticket.priorityChange.body')}
        </ConfirmDialog>
      )}
    </Card>
  )
}
