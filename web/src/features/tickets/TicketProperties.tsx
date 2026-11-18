import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import type { TFunction } from 'i18next'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { Assignee, Me, Priority, Ticket, TicketStatus } from '../../api/client'
import { usePriorityNames } from '../../api/queries'
import { Card } from '../../components/Card'
import { DescriptionList } from '../../components/DescriptionList'
import { ConfirmDialog } from '../../components/Dialog'
import { Dropdown, type DropdownOption } from '../../components/Dropdown'
import { ErrorAlert } from '../../components/ErrorAlert'
import { can } from '../../lib/roles'
import { useSignedInUser } from '../shell/useSignedInUser'
import { agentStatus, priorityLong } from './labels'
import { clientAssigneesQueryOptions, useSetAssignee, useSetPriority, useSetStatus } from './queries'
import { hasHint } from './status'

const PRIORITIES: readonly Priority[] = ['P1', 'P2', 'P3', 'P4']
/** Dropdown id for "nobody"; sent to the API as null. */
const UNASSIGNED = 'unassigned'

/**
 * Who the assignee field offers. Supervisors and admins pick anyone who works the client; an agent
 * only takes an unassigned ticket or releases their own (API: ticket.SelfAssignment).
 */
function assigneeChoices(t: TFunction, ticket: Ticket, me: Me, people: ReadonlyArray<Assignee> | null): DropdownOption<string>[] {
  const nobody = { id: UNASSIGNED, label: t('ticket.unassigned') }
  const current = ticket.assignee
  if (!people) {
    // A colleague's ticket still shows who has it; the field is disabled then.
    if (current && current.id !== me.id) return [nobody, { id: current.id, label: current.name }]
    return [nobody, { id: me.id, label: current ? current.name : t('ticket.assignToMe') }]
  }
  const options = [
    nobody,
    ...people.map((a) => ({
      id: a.id,
      label: a.name,
      description: a.jobTitle
        ? t('ticket.assigneeOption', { title: a.jobTitle, open: t('ticket.openTickets', { count: a.openTicketCount }) })
        : t('ticket.openTickets', { count: a.openTicketCount }),
    })),
  ]
  if (current && !people.some((a) => a.id === current.id)) options.push({ id: current.id, label: current.name })
  return options
}

/** A move to resolved that needs the guard: the Definition of Done isn't met (staff only see it). */
export function needsResolveGuard(ticket: Ticket): boolean {
  return ticket.dod != null && !ticket.dod.met
}

type TicketPropertiesProps = {
  ticket: Ticket
  /** Opens the guard instead of resolving a ticket whose Definition of Done isn't met. */
  onResolveGuard: () => void
}

/** Status, assignee and priority (with a reason), plus the ticket's read-only details. */
export function TicketProperties({ ticket, onResolveGuard }: TicketPropertiesProps) {
  const { t } = useTranslation()
  const me = useSignedInUser()
  const names = usePriorityNames()
  const setStatus = useSetStatus(ticket.key)
  const setAssignee = useSetAssignee(ticket.key)
  const setPriority = useSetPriority(ticket.key)
  const assignAnyone = can.assignAnyone(me.role)
  const { data: assignees, error: assigneesError } = useQuery({
    ...clientAssigneesQueryOptions(ticket.client.id),
    enabled: assignAnyone,
  })
  // An agent can't take over, release or pass on a colleague's ticket.
  const heldByColleague = !assignAnyone && ticket.assignee !== null && ticket.assignee.id !== me.id
  const [pendingPriority, setPendingPriority] = useState<Priority | null>(null)
  const locked = ticket.allowedStatuses.length === 0

  const choices = [ticket.status, ...ticket.allowedStatuses.filter((s) => s !== ticket.status)]
  const statusOptions: DropdownOption<TicketStatus>[] = choices.map((s) => ({
    id: s,
    label: agentStatus(t, s),
    // "Pauses the SLA clock" only where this ticket's project pauses it.
    description:
      hasHint(s) && (s !== 'waiting_on_client' || ticket.sla.pauseOnWaitingClient)
        ? t(`ticket.status.agentHint.${s}`)
        : undefined,
  }))

  const assigneeOptions = assigneeChoices(t, ticket, me, assignAnyone ? (assignees ?? []) : null)

  const details = [
    { term: t('ticket.fields.client'), value: ticket.client.name },
    { term: t('ticket.fields.requester'), value: ticket.requester.name },
    {
      term: t('ticket.fields.project'),
      value: (
        <Link to="/projects/$projectId" params={{ projectId: ticket.project.id }}>
          {ticket.project.name}
        </Link>
      ),
    },
    ...(ticket.catalogItem ? [{ term: t('ticket.fields.catalogItem'), value: ticket.catalogItem.name }] : []),
    { term: t('ticket.fields.channel'), value: t(`ticket.channelOption.${ticket.channel}`) },
  ]

  return (
    <Card as="h2" title={t('ticket.properties')}>
      <Dropdown
        label={t('ticket.fields.status')}
        options={statusOptions}
        value={ticket.status}
        onChange={(s) => {
          if (s === ticket.status) return
          if (s === 'resolved' && needsResolveGuard(ticket)) onResolveGuard()
          else setStatus.mutate({ status: s, from: ticket.status })
        }}
        isDisabled={locked || setStatus.isPending}
      />
      <ErrorAlert error={setStatus.error} />
      <Dropdown
        label={t('ticket.fields.assignee')}
        options={assigneeOptions}
        value={ticket.assignee?.id ?? UNASSIGNED}
        onChange={(id) => setAssignee.mutate(id === UNASSIGNED ? null : id)}
        description={heldByColleague && !locked ? t('ticket.assigneeSupervisorOnly') : undefined}
        isDisabled={locked || heldByColleague || setAssignee.isPending}
      />
      <ErrorAlert error={setAssignee.error ?? assigneesError} />
      <Dropdown
        label={t('ticket.fields.priority')}
        options={PRIORITIES.map((p) => ({ id: p, label: priorityLong(t, names, p, ticket.type) }))}
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
          title={t('ticket.priorityChange.title', { priority: priorityLong(t, names, pendingPriority, ticket.type) })}
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
