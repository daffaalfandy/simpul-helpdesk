import type { TFunction } from 'i18next'

import type { Priority, TicketActivity, TicketStatus, TicketType } from '../../api/client'
import type { PriorityNames } from '../../api/queries'
import { codeEventText } from '../code/labels'

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
      switch (entry.cause) {
        case 'assignment':
          return t('ticket.activity.statusByAssignment', values)
        case 'requester_reply':
          return t('ticket.activity.statusByReply', values)
        case 'uat_approved':
          return t('ticket.activity.statusByUatApproved', values)
        case 'uat_rejected':
          return t('ticket.activity.statusByUatRejected', values)
        case 'uat_auto_approved':
          return t('ticket.activity.statusByUatAutoApproved', values)
        case 'reopened':
          return t('ticket.activity.statusByReopen', values)
        case 'auto_closed':
          return t('ticket.activity.statusByAutoClose', values)
        case 'dod_skipped':
          return t('ticket.activity.statusByDoDSkipped', { actor, reason: entry.reason ?? '' })
        case 'code_activity':
          return t('ticket.activity.statusByCodeActivity', values)
        case 'code_done':
          return t('ticket.activity.statusByCodeDone', values)
        case 'code_reopened':
          return t('ticket.activity.statusByCodeReopened', values)
      }
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
    case 'dod_changed':
      return entry.to === 'ticket'
        ? t('ticket.activity.dodChangedTicket', { actor, reason: entry.reason ?? '' })
        : t('ticket.activity.dodChangedProject', { actor })
    case 'code_linked': {
      const code = entry.code
      if (!code) return t('ticket.activity.codeLinked', { provider: '', event: '' })
      return t('ticket.activity.codeLinked', {
        provider: t(`code.providers.${code.provider}`),
        event: codeEventText(t, code),
      })
    }
    case 'sla_at_risk':
    case 'sla_breached': {
      const clock = t(entry.to === 'first_response' ? 'ticket.activity.slaClock.first_response' : 'ticket.activity.slaClock.resolution')
      return t(entry.action === 'sla_at_risk' ? 'ticket.activity.slaAtRisk' : 'ticket.activity.slaBreached', { clock })
    }
  }
}
