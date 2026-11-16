import { delay, http, HttpResponse } from 'msw'

import type {
  Assignee,
  BoardCardSla,
  Colleague,
  Attachment,
  AttachmentPolicy,
  Board,
  BoardCard,
  CatalogItem,
  Intake,
  Requester,
  Ticket,
  TicketActivity,
  TicketMessage,
  TicketStatus,
  TicketSummary,
} from '../api/client'
import { BOARD_COLUMNS } from '../features/board/columns'
import { allowedMoves, columnOf } from './lifecycle'
import { state } from './server'
import { defaultTargets, PROJECT_IDS, projectsApi } from './projectsApi'
import { IDS, settings } from './settingsApi'

const API = '*/api/v1'
const MAX_BYTES = 10 * 1024 * 1024
/** The API's BoardColumnLimit. */
export const BOARD_COLUMN_LIMIT = 100
const PRIORITY_ORDER = ['P1', 'P2', 'P3', 'P4']
const DAYS_PER_WEEK = 7
const MINUTE_MS = 60_000
const HOUR_MS = 60 * MINUTE_MS
const WORKING_DAY_MS = 9 * HOUR_MS
const FLAGGED: readonly string[] = ['at_risk', 'breached']

export const TICKET_IDS = {
  hendra: '01a1cedf-0000-7000-8000-000000000003',
  dian: '01a1cedf-0000-7000-8000-000000000005',
  sri: '01a1cedf-0000-7000-8000-000000000006',
  vpn: '01a1cedf-0000-7000-8000-0000000f0001',
  account: '01a1cedf-0000-7000-8000-0000000f0002',
} as const

export const policy: AttachmentPolicy = { maxBytes: MAX_BYTES, maxPerMessage: 2, extensions: ['.pdf', '.png', '.txt'] }

const ref = (id: string, name: string) => ({ id, name })
const rina = ref(IDS.rina, 'Rina Pratiwi')
const adi = ref(IDS.adi, 'Adi Saputra')
const hendra = ref(TICKET_IDS.hendra, 'Hendra Gunawan')
const jambi = ref(IDS.jambi, 'Dinas Kominfo Jambi')
const batam = ref(IDS.batam, 'BPKAD Kota Batam')

function file(id: string, filename: string): Attachment {
  return { id, filename, contentType: 'image/png', sizeBytes: 84_000, createdAt: '2026-11-12T01:14:00Z' }
}

function ticket(over: Partial<Ticket> & Pick<Ticket, 'id' | 'key' | 'subject'>): Ticket {
  return {
    type: 'incident',
    status: 'new',
    priority: 'P3',
    client: jambi,
    requester: hendra,
    assignee: null,
    createdAt: '2026-11-12T01:14:00Z',
    updatedAt: '2026-11-12T01:14:00Z',
    resolvedAt: null,
    channel: 'portal',
    description: 'It broke.',
    requesterJobTitle: 'IT Officer',
    project: ref(PROJECT_IDS.eOffice, 'e-Office'),
    catalogItem: null,
    createdBy: hendra,
    loggedByStaff: false,
    attachments: [],
    sla: {
      priority: 'P3',
      firstResponse: { value: 2, unit: 'h' },
      resolution: { value: 2, unit: 'wd' },
      calendar: 'business_hours',
      pauseOnWaitingClient: true,
      pauseOnUat: true, firstResponseClock: null, resolutionClock: null,
    },
    slaTargets: defaultTargets,
    firstResponseAt: null,
    closedAt: null,
    progress: [
      { step: 'received', state: 'current', at: '2026-11-12T01:14:00Z' },
      { step: 'assigned', state: 'upcoming', at: null },
      { step: 'in_progress', state: 'upcoming', at: null },
      { step: 'ready_for_test', state: 'upcoming', at: null },
      { step: 'resolved', state: 'upcoming', at: null },
    ],
    allowedStatuses: ['in_progress', 'waiting_on_client', 'resolved'],
    dod: null,
    ...over,
  }
}

function fixtures() {
  const tickets: Ticket[] = [
    ticket({
      id: 't-24871',
      key: 'HD-24871',
      subject: 'e-Office login fails for all staff',
      description: 'Since 08:00 nobody can sign in.',
      priority: 'P1',
      status: 'in_progress',
      assignee: rina,
      attachments: [file('a-shot', 'screenshot-login-error.png')],
      firstResponseAt: '2026-11-12T01:22:00Z',
      sla: {
        priority: 'P1',
        firstResponse: { value: 15, unit: 'min' },
        resolution: { value: 4, unit: 'h' },
        calendar: 'always',
        pauseOnWaitingClient: true,
        pauseOnUat: true, firstResponseClock: null, resolutionClock: null,
      },
      allowedStatuses: ['assigned', 'waiting_on_client', 'in_uat', 'resolved'],
      progress: [
        { step: 'received', state: 'done', at: '2026-11-12T01:14:00Z' },
        { step: 'assigned', state: 'done', at: '2026-11-12T01:16:00Z' },
        { step: 'in_progress', state: 'current', at: '2026-11-12T01:52:00Z' },
        { step: 'ready_for_test', state: 'upcoming', at: null },
        { step: 'resolved', state: 'upcoming', at: null },
      ],
    }),
    ticket({ id: 't-24859', key: 'HD-24859', subject: 'VPN access for the new officer', type: 'request', status: 'waiting_on_client', assignee: rina }),
    ticket({ id: 't-24790', key: 'HD-24790', subject: 'Install a PDF reader', type: 'request', status: 'closed', resolvedAt: '2026-11-09T03:00:00Z', closedAt: '2026-11-11T03:00:00Z', allowedStatuses: [] }),
    ticket({ id: 't-24900', key: 'HD-24900', subject: 'SIPD export times out', client: batam, requester: ref(TICKET_IDS.sri, 'Sri Lestari'), createdBy: ref(TICKET_IDS.sri, 'Sri Lestari') }),
    // Board fixtures at another client, so the portal's lists stay as they are.
    ticket({ id: 't-24910', key: 'HD-24910', subject: 'Router config backup', priority: 'P2', status: 'assigned', assignee: adi, client: batam }),
    ticket({ id: 't-24915', key: 'HD-24915', subject: 'Finance report totals are off', priority: 'P2', status: 'in_uat', assignee: adi, client: batam }),
    ticket({ id: 't-24920', key: 'HD-24920', subject: 'Shared drive permissions reset', status: 'resolved', assignee: adi, client: batam, resolvedAt: new Date().toISOString() }),
  ]
  const messages: Record<string, TicketMessage[]> = {
    'HD-24871': [
      {
        id: 'm-1',
        author: rina,
        authorJobTitle: 'L2',
        fromSupport: true,
        visibility: 'public',
        body: 'Thanks Pak Hendra, we are on it.',
        attachments: [],
        createdAt: '2026-11-12T01:22:00Z',
      },
      {
        id: 'm-2',
        author: rina,
        authorJobTitle: 'L2',
        fromSupport: true,
        visibility: 'internal',
        body: "Matches last night's SSO patch.",
        attachments: [file('a-plan', 'rollback-plan.txt')],
        createdAt: '2026-11-12T01:31:00Z',
      },
    ],
  }
  const activity: Record<string, TicketActivity[]> = {
    'HD-24871': [
      { id: 'e-1', at: '2026-11-12T01:14:00Z', actorName: 'Hendra Gunawan', action: 'created', from: null, to: null, reason: null, cause: null, channel: 'portal' },
      { id: 'e-2', at: '2026-11-12T01:16:00Z', actorName: 'Rina Pratiwi', action: 'assignee_changed', from: null, to: 'Rina Pratiwi', reason: null, cause: null, channel: null },
    ],
  }
  const catalogItems: CatalogItem[] = [
    { id: TICKET_IDS.vpn, name: 'VPN / remote access', description: null, sortOrder: 10, active: true, ticketCount: 1 },
    { id: TICKET_IDS.account, name: 'New email account', description: null, sortOrder: 20, active: true, ticketCount: 0 },
  ]
  const requesters: Record<string, Requester[]> = {
    [IDS.jambi]: [
      { id: TICKET_IDS.hendra, name: 'Hendra Gunawan', email: 'hendra@kominfo-jambi.test', jobTitle: 'IT Officer', status: 'active' },
      { id: TICKET_IDS.dian, name: 'Dian Puspita', email: 'dian@kominfo-jambi.test', jobTitle: null, status: 'invited' },
    ],
  }
  const assignees: Assignee[] = [
    { id: IDS.rina, name: 'Rina Pratiwi', role: 'agent', jobTitle: 'L2', openTicketCount: 6 },
    { id: IDS.adi, name: 'Adi Saputra', role: 'agent', jobTitle: 'L2', openTicketCount: 4 },
  ]
  /** Colleagues following each ticket ("People notified"). */
  const watchers: Record<string, Colleague[]> = {}
  /** The SLA clock each board card shows. */
  const cardSla: Record<string, BoardCardSla> = {
    'HD-24910': { clock: 'first_response', phase: 'at_risk', remainingMs: 22 * MINUTE_MS, dayMs: WORKING_DAY_MS },
    'HD-24915': { clock: 'resolution', phase: 'breached', remainingMs: -12 * MINUTE_MS, dayMs: WORKING_DAY_MS },
    'HD-24920': { clock: 'resolution', phase: 'met', remainingMs: 2 * HOUR_MS, dayMs: WORKING_DAY_MS },
  }
  return { tickets, messages, activity, catalogItems, requesters, assignees, watchers, cardSla }
}

type Recorded = { method: string; path: string; search: string; body: unknown }

/** Fake ticket data, plus every request made; reset after each test. */
export const tickets = { ...fixtures(), requests: [] as Recorded[], nextNumber: 24892, uploads: [] as { size: number; type: string }[] }

export function resetTickets() {
  Object.assign(tickets, fixtures(), { requests: [], nextNumber: 24892, uploads: [] })
}

function problem(status: number, code: string) {
  return HttpResponse.json(
    { type: 'about:blank', title: code, status, code },
    { status, headers: { 'Content-Type': 'application/problem+json' } },
  )
}

async function record(request: Request, body?: unknown) {
  const url = new URL(request.url)
  let parsed = body
  if (parsed === undefined && request.method !== 'GET') {
    const text = await request.text()
    parsed = text ? JSON.parse(text) : undefined
  }
  tickets.requests.push({ method: request.method, path: url.pathname, search: url.search, body: parsed })
  return parsed
}

const isRequester = () => state.me?.role === 'requester'

/** What the signed-in user may see, like the API's scope (requesters: their client only). */
function visible(t: Ticket) {
  return !isRequester() || t.client.id === state.me?.client?.id
}

function find(key: unknown) {
  const t = tickets.tickets.find((x) => x.key === key)
  return t && visible(t) ? t : undefined
}

function summary(t: Ticket): TicketSummary {
  const { id, key, type, status, priority, subject, client, requester, assignee, createdAt, updatedAt, resolvedAt } = t
  return { id, key, type, status, priority, subject, client, requester, assignee, createdAt, updatedAt, resolvedAt }
}

let nextId = 0
const newId = (prefix: string) => `${prefix}-${++nextId}`

/** The project a new ticket lands in: the incident's system, or the client's default. */
function projectOf(body: Record<string, unknown>) {
  const clientId = (body.clientId as string | undefined) ?? IDS.jambi
  const p =
    projectsApi.projects.find((x) => x.id === body.projectId) ??
    projectsApi.projects.find((x) => x.client.id === clientId && x.isDefault)
  return ref(p?.id ?? '', p?.name ?? '')
}

function boardCard(t: Ticket): BoardCard {
  const { id, key, type, status, priority, subject, client, assignee } = t
  return {
    id,
    key,
    type,
    status,
    priority,
    subject,
    client,
    assignee,
    allowedStatuses: allowedMoves(t),
    sla: tickets.cardSla[key] ?? null,
    dodPending: t.dod?.pending ?? [],
  }
}

function startOfToday() {
  const d = new Date()
  d.setHours(0, 0, 0, 0)
  return d
}

/** The board like GET /board: columns in order, P1 first, done tickets from midnight only. */
function board(assignee: string | null, q: string, slaFilter: string | null): Board {
  const midnight = startOfToday()
  const weekStart = new Date(midnight)
  weekStart.setDate(weekStart.getDate() - ((weekStart.getDay() + DAYS_PER_WEEK - 1) % DAYS_PER_WEEK))
  const since = (t: Ticket, from: Date) => t.resolvedAt !== null && new Date(t.resolvedAt) >= from
  const search = q.toLowerCase()
  const cards = tickets.tickets
    .filter((t) => columnOf(t.status) !== null && (t.resolvedAt === null || since(t, midnight)))
    .filter((t) => assignee !== 'me' || t.assignee?.id === state.me?.id)
    .filter((t) => assignee !== 'unassigned' || t.assignee === null)
    .filter((t) => !search || [t.key, t.subject, t.client.name].some((s) => s.toLowerCase().includes(search)))
    .filter((t) => slaFilter !== 'at_risk' || FLAGGED.includes(tickets.cardSla[t.key]?.phase ?? ''))
  const flagged = (phase: string) => Object.values(tickets.cardSla).filter((c) => c.phase === phase).length
  return {
    columns: BOARD_COLUMNS.map((id) => {
      const inColumn = cards
        .filter((t) => columnOf(t.status) === id)
        .sort((a, b) => PRIORITY_ORDER.indexOf(a.priority) - PRIORITY_ORDER.indexOf(b.priority))
      return { id, total: inColumn.length, cards: inColumn.slice(0, BOARD_COLUMN_LIMIT).map(boardCard) }
    }),
    resolvedSince: midnight.toISOString(),
    stats: {
      openTickets: tickets.tickets.filter((t) => t.resolvedAt === null && t.status !== 'closed').length,
      resolvedThisWeek: tickets.tickets.filter((t) => since(t, weekStart)).length,
      slaBreached: flagged('breached'),
      slaAtRisk: flagged('at_risk'),
      slaOldest: flagged('breached') ? { key: 'HD-24915', clientName: 'BPKAD Kota Batam' } : null,
      avgFirstResponseMs: 18 * MINUTE_MS,
      withinSlaPercent: 94.6,
    },
  }
}

/** The ticket's client's other requesters, as colleagues who could follow it. */
function colleaguesFor(t: Ticket): Colleague[] {
  const following = tickets.watchers[t.key] ?? []
  return (tickets.requesters[t.client.id] ?? [])
    .filter((r) => r.id !== t.requester.id && !following.some((w) => w.id === r.id))
    .map((r) => ({ id: r.id, name: r.name, jobTitle: r.jobTitle }))
}

export const ticketsHandlers = [
  http.get(`${API}/tickets/:key/watchers`, ({ params }) => HttpResponse.json(tickets.watchers[String(params.key)] ?? [])),
  http.get(`${API}/tickets/:key/watcher-candidates`, async ({ request, params }) => {
    await record(request)
    const t = tickets.tickets.find((x) => x.key === params.key)
    if (!t || !visible(t)) return problem(404, 'not_found')
    const q = (new URL(request.url).searchParams.get('q') ?? '').toLowerCase()
    return HttpResponse.json(colleaguesFor(t).filter((c) => c.name.toLowerCase().includes(q)))
  }),
  http.post(`${API}/tickets/:key/watchers`, async ({ request, params }) => {
    const body = (await record(request)) as { userId: string }
    const t = tickets.tickets.find((x) => x.key === params.key)
    if (!t || !visible(t)) return problem(404, 'not_found')
    const c = colleaguesFor(t).find((x) => x.id === body.userId)
    if (!c) return problem(400, 'invalid_watcher')
    const list = [...(tickets.watchers[t.key] ?? []), c]
    tickets.watchers[t.key] = list
    return HttpResponse.json(list)
  }),
  http.delete(`${API}/tickets/:key/watchers/:userId`, async ({ request, params }) => {
    await record(request)
    const list = tickets.watchers[String(params.key)] ?? []
    if (!list.some((w) => w.id === params.userId)) return problem(404, 'not_found')
    tickets.watchers[String(params.key)] = list.filter((w) => w.id !== params.userId)
    return new HttpResponse(null, { status: 204 })
  }),
  http.get(`${API}/board`, async ({ request }) => {
    await record(request)
    if (isRequester()) return problem(403, 'forbidden')
    const params = new URL(request.url).searchParams
    return HttpResponse.json(board(params.get('assignee'), params.get('q') ?? '', params.get('sla')))
  }),
  http.get(`${API}/attachment-policy`, () => HttpResponse.json(policy)),
  http.get(`${API}/intake`, async ({ request }) => {
    await record(request)
    const clientId = new URL(request.url).searchParams.get('clientId') ?? IDS.jambi
    const projects = projectsApi.projects.filter((p) => p.client.id === clientId)
    const intake: Intake = {
      clientId,
      businessHours: settings.hours,
      projects: projects
        .filter((p) => p.active)
        .map(({ id, name, description, sla }) => ({ id, name, description, targets: sla.targets })),
      defaultProjectId: projects.find((p) => p.isDefault)?.id ?? null,
      catalogItems: tickets.catalogItems.filter((i) => i.active).map(({ id, name, description }) => ({ id, name, description })),
    }
    return HttpResponse.json(intake)
  }),
  http.get(`${API}/tickets`, async ({ request }) => {
    await record(request)
    const params = new URL(request.url).searchParams
    const statuses = params.getAll('status') as TicketStatus[]
    const assignee = params.get('assignee')
    const client = params.get('clientId')
    const list = tickets.tickets
      .filter(visible)
      .filter((t) => statuses.length === 0 || statuses.includes(t.status))
      .filter((t) => assignee !== 'me' || t.assignee?.id === state.me?.id)
      .filter((t) => assignee !== 'unassigned' || t.assignee === null)
      .filter((t) => !client || t.client.id === client)
    return HttpResponse.json(list.map(summary))
  }),
  http.post(`${API}/tickets`, async ({ request }) => {
    const body = (await record(request)) as Record<string, unknown>
    const me = state.me
    const staff = !isRequester()
    const t = ticket({
      id: newId('t'),
      key: `HD-${tickets.nextNumber++}`,
      subject: String(body.subject),
      description: String(body.description),
      type: body.type as Ticket['type'],
      priority: body.priority as Ticket['priority'],
      channel: staff ? (body.channel as Ticket['channel']) : 'portal',
      loggedByStaff: staff,
      createdBy: ref(me?.id ?? '', me?.name ?? ''),
      status: body.assignToMe ? 'assigned' : 'new',
      assignee: body.assignToMe && me ? ref(me.id, me.name) : null,
      project: projectOf(body),
      sla: {
        priority: body.priority as Ticket['priority'],
        firstResponse: { value: 30, unit: 'min' },
        resolution: { value: 8, unit: 'h' },
        calendar: 'business_hours',
        pauseOnWaitingClient: true,
        pauseOnUat: true, firstResponseClock: null, resolutionClock: null,
      },
    })
    tickets.tickets.unshift(t)
    return HttpResponse.json(t, { status: 201 })
  }),
  http.get(`${API}/tickets/:key`, ({ params }) => {
    const t = find(params.key)
    return t ? HttpResponse.json(t) : problem(404, 'not_found')
  }),
  http.get(`${API}/tickets/:key/messages`, ({ params }) => {
    const t = find(params.key)
    if (!t) return problem(404, 'not_found')
    const all = tickets.messages[t.key] ?? []
    return HttpResponse.json(isRequester() ? all.filter((m) => m.visibility === 'public') : all)
  }),
  http.post(`${API}/tickets/:key/messages`, async ({ request, params }) => {
    const body = (await record(request)) as { visibility: 'public' | 'internal'; body: string }
    const t = find(params.key)
    if (!t) return problem(404, 'not_found')
    const me = state.me
    const m: TicketMessage = {
      id: newId('m'),
      author: ref(me?.id ?? '', me?.name ?? ''),
      authorJobTitle: me?.jobTitle ?? null,
      fromSupport: !isRequester(),
      visibility: body.visibility,
      body: body.body,
      attachments: [],
      createdAt: new Date().toISOString(),
    }
    tickets.messages[t.key] = [...(tickets.messages[t.key] ?? []), m]
    return HttpResponse.json(m, { status: 201 })
  }),
  http.put(`${API}/tickets/:key/status`, async ({ request, params }) => {
    const body = (await record(request)) as { status: TicketStatus; from?: TicketStatus }
    const t = find(params.key)
    if (!t) return problem(404, 'not_found')
    if (body.from !== undefined && body.from !== t.status) return problem(409, 'status_changed')
    if (body.status !== t.status && !allowedMoves(t).includes(body.status)) return problem(409, 'invalid_status_change')
    const { status } = body
    const from = t.status
    t.status = status
    t.resolvedAt = status === 'resolved' || status === 'closed' ? (t.resolvedAt ?? new Date().toISOString()) : null
    t.allowedStatuses = allowedMoves(t)
    tickets.activity[t.key] = [
      ...(tickets.activity[t.key] ?? []),
      { id: newId('e'), at: new Date().toISOString(), actorName: state.me?.name ?? null, action: 'status_changed', from, to: status, reason: null, cause: null, channel: null },
    ]
    return HttpResponse.json(t)
  }),
  http.put(`${API}/tickets/:key/assignee`, async ({ request, params }) => {
    const { assigneeId } = (await record(request)) as { assigneeId: string | null }
    const t = find(params.key)
    if (!t) return problem(404, 'not_found')
    const a = tickets.assignees.find((x) => x.id === assigneeId)
    t.assignee = a ? ref(a.id, a.name) : null
    return HttpResponse.json(t)
  }),
  http.put(`${API}/tickets/:key/priority`, async ({ request, params }) => {
    const body = (await record(request)) as { priority: Ticket['priority']; reason: string }
    const t = find(params.key)
    if (!t) return problem(404, 'not_found')
    t.priority = body.priority
    t.sla = { ...t.sla, priority: body.priority }
    return HttpResponse.json(t)
  }),
  http.get(`${API}/tickets/:key/activity`, ({ params }) => {
    if (isRequester()) return problem(403, 'forbidden')
    const t = find(params.key)
    return t ? HttpResponse.json(tickets.activity[t.key] ?? []) : problem(404, 'not_found')
  }),
  http.post(`${API}/attachments`, async ({ request }) => {
    const form = await request.formData()
    // undici's File, not jsdom's Blob: check the shape, not the class.
    const upload = form.get('file')
    if (upload === null || typeof upload === 'string') return problem(400, 'invalid_request')
    await record(request, { size: upload.size, type: upload.type })
    tickets.uploads.push({ size: upload.size, type: upload.type })
    if (upload.size > MAX_BYTES) return problem(413, 'attachment_too_large')
    await delay(1)
    const a: Attachment = { id: newId('a'), filename: 'upload', contentType: upload.type, sizeBytes: upload.size, createdAt: new Date().toISOString() }
    return HttpResponse.json(a, { status: 201 })
  }),
  http.get(`${API}/clients/:id/requesters`, ({ params }) => HttpResponse.json(tickets.requesters[String(params.id)] ?? [])),
  http.post(`${API}/clients/:id/requesters`, async ({ request, params }) => {
    const body = (await record(request)) as { name: string; email: string }
    const list = (tickets.requesters[String(params.id)] ??= [])
    if (Object.values(tickets.requesters).flat().some((r) => r.email === body.email.toLowerCase())) {
      return problem(409, 'email_taken')
    }
    const r: Requester = { id: newId('r'), name: body.name, email: body.email.toLowerCase(), jobTitle: null, status: 'invited' }
    list.push(r)
    return HttpResponse.json(r, { status: 201 })
  }),
  http.get(`${API}/clients/:id/assignees`, () => HttpResponse.json(tickets.assignees)),
  http.get(`${API}/catalog-items`, () => HttpResponse.json(tickets.catalogItems)),
  http.post(`${API}/catalog-items`, async ({ request }) => {
    const body = (await record(request)) as Omit<CatalogItem, 'id' | 'ticketCount'>
    const i: CatalogItem = { ...body, description: body.description ?? null, id: newId('i'), ticketCount: 0 }
    tickets.catalogItems.push(i)
    return HttpResponse.json(i, { status: 201 })
  }),
  http.put(`${API}/catalog-items/:id`, async ({ request, params }) => {
    const body = (await record(request)) as Partial<CatalogItem>
    const i = tickets.catalogItems.find((x) => x.id === params.id)
    if (!i) return problem(404, 'not_found')
    Object.assign(i, body)
    return HttpResponse.json(i)
  }),
]
