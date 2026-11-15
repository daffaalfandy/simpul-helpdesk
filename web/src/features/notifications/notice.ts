import type { TFunction } from 'i18next'
import { ArrowRight, Clock, Flag, MessageSquare, TicketIcon, User, UserPlus, type LucideIcon } from 'lucide-react'

import type { Audience, Notification, NotificationFilter, TicketStatus } from '../../api/client'

/** Inbox filters each audience offers (HANDOFF §4). SLA arrives with the SLA engine; code activity in M7. */
export const TEAM_FILTERS: readonly NotificationFilter[] = ['all', 'unread', 'comments', 'status', 'assigned', 'sla']
export const PORTAL_FILTERS: readonly NotificationFilter[] = ['all', 'unread', 'replies', 'status']
export const BELL_FILTERS: readonly NotificationFilter[] = ['all', 'unread']
export const DEFAULT_FILTER: NotificationFilter = 'all'

export function filtersFor(audience: Audience) {
  return audience === 'team' ? TEAM_FILTERS : PORTAL_FILTERS
}

/** Icon tint per kind of change (the Notifications artboard). */
const tones = {
  message: 'bg-primary-tint text-primary',
  status: 'bg-success-tint text-success',
  person: 'bg-chip-paused-bg text-chip-paused-fg',
  priority: 'bg-chip-warning-bg text-chip-warning-fg',
  sla: 'bg-danger-tint text-danger',
} as const

/** The type icon of a notification: what kind of change it is, never decoration. */
export function noticeIcon(n: Notification): { Icon: LucideIcon; tone: string } {
  switch (n.event) {
    case 'status':
    case 'resolved':
      return { Icon: ArrowRight, tone: tones.status }
    case 'assignment':
      return { Icon: User, tone: tones.person }
    case 'priority':
      return { Icon: Flag, tone: tones.priority }
    case 'sla':
      return { Icon: Clock, tone: tones.sla }
    case 'ticket_logged':
      return { Icon: TicketIcon, tone: tones.message }
    case 'colleague':
      if (n.kind === 'added') return { Icon: UserPlus, tone: tones.person }
      if (n.kind === 'status') return { Icon: ArrowRight, tone: tones.status }
      return { Icon: MessageSquare, tone: tones.message }
    default:
      return { Icon: MessageSquare, tone: tones.message }
  }
}

/** What changed, in the recipient's wording ("Status changed: In progress → Ready for your test"). */
export function noticeText(t: TFunction, n: Notification, audience: Audience, clock: (iso: string) => string): string {
  const actor = n.actorName ?? t('notifications.text.system')
  const status = (s?: TicketStatus | null) =>
    s ? t(audience === 'team' ? `ticket.status.agent.${s}` : `ticket.status.requester.${s}`) : ''
  const from = status(n.statusFrom)
  const to = status(n.statusTo)
  switch (n.event) {
    case 'comment':
    case 'internal_note':
    case 'reply':
    case 'question':
      return t(`notifications.text.${n.event}`, { actor })
    case 'status':
    case 'resolved':
      return t('notifications.text.status', { from, to })
    case 'assignment':
      return t(n.kind === 'unassigned' ? 'notifications.text.unassigned' : 'notifications.text.assigned', { actor })
    case 'priority':
      return t('notifications.text.priority', { actor, from: n.priorityFrom ?? '', to: n.priorityTo ?? '' })
    case 'colleague':
      if (n.kind === 'added') return t('notifications.text.colleagueAdded', { actor })
      if (n.kind === 'status') return t('notifications.text.colleagueStatus', { actor, from, to })
      return t('notifications.text.colleagueReply', { actor })
    case 'ticket_logged':
      return t('notifications.text.ticketLogged', { actor })
    case 'sla': {
      const slaClock = t(`notifications.text.slaClock.${n.slaClock ?? 'resolution'}`)
      return n.kind === 'breached'
        ? t('notifications.text.slaBreached', { clock: slaClock, time: n.slaDueAt ? clock(n.slaDueAt) : '' })
        : t('notifications.text.slaAtRisk', { clock: slaClock, minutes: n.slaMinutesLeft ?? 0 })
    }
  }
}

/** The quoted line under the text: the message, or a priority change's reason. */
export function noticeQuote(n: Notification): string | null {
  return n.quote ?? (n.event === 'priority' ? (n.reason ?? null) : null)
}
