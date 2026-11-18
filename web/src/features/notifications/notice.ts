import type { TFunction } from 'i18next'
import { ArrowRight, Clock, Flag, GitBranch, ListChecks, MessageSquare, TicketIcon, User, UserPlus, type LucideIcon } from 'lucide-react'

import type { Audience, Notification, NotificationFilter, TicketStatus } from '../../api/client'
import { codeEventText } from '../code/labels'

/** Inbox filters each audience offers (HANDOFF §4). */
export const TEAM_FILTERS: readonly NotificationFilter[] = ['all', 'unread', 'comments', 'status', 'assigned', 'sla', 'code']
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
  code: 'bg-chip-code-bg text-chip-code-fg',
} as const

/** The type icon of a notification: what kind of change it is, never decoration. */
export function noticeIcon(n: Notification): { Icon: LucideIcon; tone: string } {
  switch (n.event) {
    case 'status':
    case 'resolved':
    case 'uat_requested':
    case 'uat_result':
      return { Icon: ArrowRight, tone: tones.status }
    case 'dod':
      return { Icon: ListChecks, tone: tones.priority }
    case 'assignment':
      return { Icon: User, tone: tones.person }
    case 'priority':
      return { Icon: Flag, tone: tones.priority }
    case 'sla':
      return { Icon: Clock, tone: tones.sla }
    case 'ticket_logged':
    case 'new_ticket':
      return { Icon: TicketIcon, tone: tones.message }
    case 'code':
      return { Icon: GitBranch, tone: tones.code }
    case 'colleague':
      if (n.kind === 'added') return { Icon: UserPlus, tone: tones.person }
      if (n.kind === 'status') return { Icon: ArrowRight, tone: tones.status }
      return { Icon: MessageSquare, tone: tones.message }
    default:
      return { Icon: MessageSquare, tone: tones.message }
  }
}

/** Formats the times a notice mentions, in the organisation's time zone. */
type NoticeTimes = { clock: (iso: string) => string; day: (iso: string) => string }

/** What changed, in the recipient's wording ("Status changed: In progress → Ready for your test"). */
export function noticeText(t: TFunction, n: Notification, audience: Audience, time: NoticeTimes): string {
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
    case 'new_ticket':
      return t(`notifications.text.${n.event}`, { actor })
    case 'status': {
      const code = n.also.find((a) => a.code)?.code
      return code
        ? t('notifications.text.statusByCode', { from, to, event: codeEventText(t, code) })
        : t('notifications.text.status', { from, to })
    }
    case 'resolved':
      return n.reopenUntil && n.statusTo === 'resolved'
        ? t('notifications.text.resolvedReopen', { from, to, date: time.day(n.reopenUntil) })
        : t('notifications.text.status', { from, to })
    case 'uat_requested':
      return n.autoApproveAt
        ? t('notifications.text.uatRequested', { from, to, date: time.day(n.autoApproveAt) })
        : t('notifications.text.status', { from, to })
    case 'uat_result':
      if (n.kind === 'rejected') return t('notifications.text.uatRejected', { actor, from, to })
      if (n.kind === 'auto_approved') return t('notifications.text.uatAutoApproved', { from, to })
      return t('notifications.text.uatApproved', { actor, from, to })
    case 'dod':
      return t(n.kind === 'override' ? 'notifications.text.dodOverride' : 'notifications.text.dodSkipped', { actor })
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
    case 'code':
      return n.code
        ? t('notifications.text.code', { provider: t(`code.providers.${n.code.provider}`), event: codeEventText(t, n.code) })
        : t('notifications.text.codeUnknown')
    case 'sla': {
      const slaClock = t(`notifications.text.slaClock.${n.slaClock ?? 'resolution'}`)
      return n.kind === 'breached'
        ? t('notifications.text.slaBreached', { clock: slaClock, time: n.slaDueAt ? time.clock(n.slaDueAt) : '' })
        : t('notifications.text.slaAtRisk', { clock: slaClock, minutes: n.slaMinutesLeft ?? 0 })
    }
  }
}

/** The quoted line under the text: the message, or a staff reason (a priority change, a skipped DoD). */
export function noticeQuote(n: Notification): string | null {
  return n.quote ?? (n.event === 'priority' || n.event === 'dod' ? (n.reason ?? null) : null)
}
