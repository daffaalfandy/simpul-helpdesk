import type { TFunction } from 'i18next'

import type { Priority, TicketActivity, TicketStatus, TicketType } from '../../api/client'
import type { PriorityNames } from '../../api/queries'

/**
 * "Critical" (Settings › Priorities), or the report form's request wording, e.g. "Urgent" for a
 * P1 service request (NewTicket mock).
 */
export function priorityName(t: TFunction, names: PriorityNames, priority: Priority, type: TicketType): string {
  return type === 'request' ? t(`priority.request.${priority}.name`) : names[priority].name
}

/** "P1 · Critical" */
export function priorityLong(t: TFunction, names: PriorityNames, priority: Priority, type: TicketType): string {
  return t('priority.long', { priority, name: priorityName(t, names, priority, type) })
}

export function agentStatus(t: TFunction, status: TicketStatus): string {
  return t(`ticket.status.agent.${status}`)
}

export function requesterStatus(t: TFunction, status: TicketStatus): string {
  return t(`ticket.status.requester.${status}`)
}

/** One activity-log line in plain words. */
export function activityText(t: TFunction, names: PriorityNames, entry: TicketActivity, type: TicketType): string {
  const actor = entry.actorName ?? t('ticket.activity.system')
  const status = (s: string | null) => (s ? agentStatus(t, s as TicketStatus) : '')
  const priority = (p: string | null) => (p ? priorityLong(t, names, p as Priority, type) : '')
  switch (entry.action) {
    case 'created':
      return t('ticket.activity.created', { actor, channel: t(`ticket.channel.${entry.channel ?? 'portal'}`) })
    case 'status_changed': {
      const values = { actor, from: status(entry.from), to: status(entry.to) }
      if (entry.cause === 'assignment') return t('ticket.activity.statusByAssignment', values)
      if (entry.cause === 'requester_reply') return t('ticket.activity.statusByReply', values)
      return t('ticket.activity.statusChanged', values)
    }
    case 'assignee_changed':
      if (!entry.from) return t('ticket.activity.assigned', { actor, to: entry.to })
      if (!entry.to) return t('ticket.activity.unassigned', { actor, from: entry.from })
      return t('ticket.activity.reassigned', { actor, from: entry.from, to: entry.to })
    case 'priority_changed':
      return t('ticket.activity.priorityChanged', {
        actor,
        from: priority(entry.from),
        to: priority(entry.to),
        reason: entry.reason ?? '',
      })
    case 'reply_added':
      return t('ticket.activity.replyAdded', { actor })
    case 'note_added':
      return t('ticket.activity.noteAdded', { actor })
    case 'first_responded':
      return t('ticket.activity.firstResponded')
  }
}
