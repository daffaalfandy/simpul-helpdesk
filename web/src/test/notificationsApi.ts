import { http, HttpResponse } from 'msw'

import type {
  EmailFrequency,
  Notification,
  NotificationEvent,
  NotificationEventSetting,
  NotificationFilter,
  NotificationSettings,
} from '../api/client'
import { state } from './server'

const API = '*/api/v1'

const FILTER_EVENTS: Partial<Record<NotificationFilter, NotificationEvent[]>> = {
  comments: ['comment', 'internal_note'],
  status: ['status', 'resolved'],
  assigned: ['assignment'],
  sla: ['sla'],
  replies: ['reply', 'question'],
}

function item(id: string, over: Partial<Notification>): Notification {
  return {
    id,
    event: 'comment',
    kind: null,
    ticket: { key: 'HD-24871', subject: 'e-Office login fails' },
    actorName: null,
    statusFrom: null,
    statusTo: null,
    priorityFrom: null,
    priorityTo: null,
    reason: null,
    assigneeName: null,
    slaClock: null,
    slaDueAt: null,
    slaMinutesLeft: null,
    quote: null,
    also: [],
    channels: { web: true, email: true },
    createdAt: '2026-11-16T07:02:00Z',
    readAt: null,
    ...over,
  }
}

/** The agent's inbox: the mock's items M5 can produce, newest first. */
export const NOTICE_IDS = { reply: 'n-1', assign: 'n-2', priority: 'n-3', question: 'n-4', status: 'n-5' } as const

function teamItems(): Notification[] {
  return [
    item(NOTICE_IDS.reply, {
      actorName: 'Hendra Gunawan',
      quote: 'Two of three staff can sign in now, one still sees the error.',
    }),
    item(NOTICE_IDS.assign, {
      event: 'assignment',
      kind: 'assigned',
      actorName: 'Dewi Wulandari',
      ticket: { key: 'HD-24893', subject: 'Printer offline in Bappeda' },
      createdAt: '2026-11-16T06:05:00Z',
    }),
    item(NOTICE_IDS.priority, {
      event: 'priority',
      actorName: 'Adi Saputra',
      priorityFrom: 'P3',
      priorityTo: 'P2',
      reason: 'The whole team is blocked.',
      ticket: { key: 'HD-24859', subject: 'SIPD export times out' },
      channels: { web: true, email: false },
      createdAt: '2026-11-15T03:00:00Z',
      readAt: '2026-11-15T04:00:00Z',
    }),
  ]
}

function portalItems(): Notification[] {
  return [
    item(NOTICE_IDS.question, {
      event: 'question',
      actorName: 'Yoga Hidayat',
      quote: 'Which time tomorrow suits you for a visit?',
      ticket: { key: 'HD-24812', subject: 'Projector in meeting room 2' },
    }),
    item(NOTICE_IDS.status, {
      event: 'status',
      statusFrom: 'in_progress',
      statusTo: 'in_uat',
      createdAt: '2026-11-16T05:45:00Z',
    }),
  ]
}

function settingsFor(audience: 'team' | 'portal'): NotificationSettings {
  const ev = (event: NotificationEvent, web: boolean, email: boolean, locked = false): NotificationEventSetting => ({
    event,
    web,
    email,
    locked,
  })
  return audience === 'team'
    ? {
        audience,
        emailFrequency: 'instant',
        events: [
          ev('comment', true, true),
          ev('internal_note', true, false),
          ev('status', true, true),
          ev('assignment', true, true, true),
          ev('priority', true, false),
          ev('sla', true, true, true),
        ],
      }
    : {
        audience,
        emailFrequency: 'instant',
        events: [
          ev('reply', true, true),
          ev('status', true, true),
          ev('question', true, true),
          ev('resolved', true, true),
          ev('colleague', true, false),
        ],
      }
}

type Recorded = { method: string; path: string; body?: unknown }

/** Fake notification state per audience, plus every write; reset after each test. */
export const notifications = {
  team: teamItems(),
  portal: portalItems(),
  teamSettings: settingsFor('team'),
  portalSettings: settingsFor('portal'),
  failNextSave: false,
  requests: [] as Recorded[],
}

export function resetNotifications() {
  Object.assign(notifications, {
    team: teamItems(),
    portal: portalItems(),
    teamSettings: settingsFor('team'),
    portalSettings: settingsFor('portal'),
    failNextSave: false,
    requests: [],
  })
}

const portal = () => state.me?.role === 'requester'
const items = () => (portal() ? notifications.portal : notifications.team)
const unread = () => items().filter((n) => n.readAt == null).length

function settingsKey(): 'teamSettings' | 'portalSettings' {
  return portal() ? 'portalSettings' : 'teamSettings'
}

function problem(status: number, code: string) {
  return HttpResponse.json(
    { type: 'about:blank', title: code, status, code },
    { status, headers: { 'Content-Type': 'application/problem+json' } },
  )
}

export const notificationsHandlers = [
  http.get(`${API}/notifications/unread-count`, () => HttpResponse.json({ count: unread() })),
  http.get(`${API}/notifications`, ({ request }) => {
    const url = new URL(request.url)
    const filter = (url.searchParams.get('filter') ?? 'all') as NotificationFilter
    const limit = Number(url.searchParams.get('limit') ?? items().length)
    const events = FILTER_EVENTS[filter]
    const list = items().filter(
      (n) => (filter !== 'unread' || n.readAt == null) && (!events || events.includes(n.event)),
    )
    return HttpResponse.json({ items: list.slice(0, limit), unreadCount: unread(), nextBefore: null })
  }),
  http.put(`${API}/notifications/:id/read`, async ({ request, params }) => {
    const body = (await request.json()) as { read: boolean }
    notifications.requests.push({ method: 'PUT', path: new URL(request.url).pathname, body })
    const n = items().find((x) => x.id === params.id)
    if (!n) return problem(404, 'not_found')
    n.readAt = body.read ? '2026-11-16T08:00:00Z' : null
    return new HttpResponse(null, { status: 204 })
  }),
  http.post(`${API}/notifications/read-all`, ({ request }) => {
    notifications.requests.push({ method: 'POST', path: new URL(request.url).pathname })
    for (const n of items()) n.readAt ??= '2026-11-16T08:00:00Z'
    return new HttpResponse(null, { status: 204 })
  }),
  http.get(`${API}/notification-settings`, () => HttpResponse.json(notifications[settingsKey()])),
  http.put(`${API}/notification-settings/events/:event`, async ({ request, params }) => {
    const body = (await request.json()) as { web: boolean; email: boolean }
    notifications.requests.push({ method: 'PUT', path: new URL(request.url).pathname, body })
    if (notifications.failNextSave) {
      notifications.failNextSave = false
      return problem(500, 'internal')
    }
    const s = notifications[settingsKey()]
    const e = s.events.find((x) => x.event === params.event)
    if (!e) return problem(400, 'notification_event_unknown')
    if (e.locked && !(body.web && body.email)) return problem(400, 'notification_locked')
    Object.assign(e, body)
    return HttpResponse.json(s)
  }),
  http.put(`${API}/notification-settings/email-frequency`, async ({ request }) => {
    const body = (await request.json()) as { emailFrequency: EmailFrequency }
    notifications.requests.push({ method: 'PUT', path: new URL(request.url).pathname, body })
    const s = notifications[settingsKey()]
    s.emailFrequency = body.emailFrequency
    return HttpResponse.json(s)
  }),
]
