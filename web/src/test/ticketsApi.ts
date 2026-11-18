import { delay, http, HttpResponse } from 'msw'

import type {
  Assignee,
  BoardCardSla,
  Colleague,
  Attachment,
  AttachmentPolicy,
  Board,
  BoardCard,
  BoardCardCode,
  BoardCardUat,
  CatalogItem,
  DodConfig,
  DodItem,
  Intake,
  Requester,
  Ticket,
  TicketActivity,
  TicketCode,
  TicketDod,
  TicketDodInput,
  TicketMessage,
  UatAnswer,
  UatRequest,
  TicketStatus,
  TicketSummary,
} from '../api/client'
import { BOARD_COLUMNS } from '../features/board/columns'
import { can } from '../lib/roles'
import { allowedMoves, columnOf } from './lifecycle'
import { state } from './server'
import { defaultTargets, PROJECT_IDS, projectsApi, uatDod } from './projectsApi'
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

/** The client items a configuration asks for, as the API's dod.Required (no code until M7). */
function requiredItems(c: DodConfig): DodItem[] {
  const out: DodItem[] = []
  if (c.uat.enabled) out.push('uat_approved')
  if (c.requesterConfirms) out.push('requester_confirmed')
  return out
}

/** A ticket's Definition of Done as staff see it, with nothing met yet. */
export function ticketDod(project: DodConfig, own?: { config: DodConfig; reason: string }): TicketDod {
  const config = own?.config ?? project
  const items = requiredItems(config)
  return {
    source: own ? 'ticket' : 'project',
    config,
    projectConfig: project,
    overrideReason: own?.reason ?? null,
    items: items.map((item) => ({ item, doneAt: null, doneBy: null })),
    met: items.length === 0,
    pending: items,
    canOverride: true,
  }
}
const rina = ref(IDS.rina, 'Rina Pratiwi')
const adi = ref(IDS.adi, 'Adi Saputra')
const hendra = ref(TICKET_IDS.hendra, 'Hendra Gunawan')
const jambi = ref(IDS.jambi, 'Dinas Kominfo Jambi')
const batam = ref(IDS.batam, 'BPKAD Kota Batam')

function file(id: string, filename: string): Attachment {
  return { id, filename, contentType: 'image/png', sizeBytes: 84_000, createdAt: '2026-11-12T01:14:00Z' }
}

export function ticket(over: Partial<Ticket> & Pick<Ticket, 'id' | 'key' | 'subject'>): Ticket {
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
    uat: null,
    reopen: null,
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
      dod: ticketDod(uatDod),
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
      { id: 'e-1', at: '2026-11-12T01:14:00Z', actorName: 'Hendra Gunawan', action: 'created', from: null, to: null, reason: null, cause: null, channel: 'portal', code: null },
      { id: 'e-2', at: '2026-11-12T01:16:00Z', actorName: 'Rina Pratiwi', action: 'assignee_changed', from: null, to: 'Rina Pratiwi', reason: null, cause: null, channel: null, code: null },
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
  /** Which day of the client's test each In UAT card is on. */
  const cardUat: Record<string, BoardCardUat> = { 'HD-24915': { day: 2, days: 3 } }
  const cardCode: Record<string, BoardCardCode> = {
    'HD-24871': { provider: 'github', commits: 2, sha: 'a1f9c2e', branch: 'fix/HD-24871-sso' },
  }
  const code: Record<string, TicketCode> = {
    'HD-24871': {
      repository: { id: 'r-1', provider: 'github', fullName: 'vendor-id/eoffice-jambi', webUrl: 'https://github.com/vendor-id/eoffice-jambi' },
      commits: [
        {
          provider: 'github', sha: 'a1f9c2e', title: 'Fix HD-24871: revert SSO token TTL to 8h', branches: ['fix/HD-24871-sso', 'main'],
          author: 'Adi Saputra', url: 'https://github.com/vendor-id/eoffice-jambi/commit/a1f9c2e', at: '2026-11-12T01:52:00Z',
        },
        {
          provider: 'github', sha: '7c3d019', title: 'refs HD-24871 add logging around token refresh', branches: ['fix/HD-24871-sso'],
          author: 'Adi Saputra', url: 'https://github.com/vendor-id/eoffice-jambi/commit/7c3d019', at: '2026-11-12T01:40:00Z',
        },
      ],
    },
  }
  return { tickets, messages, activity, catalogItems, requesters, assignees, watchers, cardSla, cardUat, cardCode, code }
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

/** Working days are business time; the fake just adds three real days for the reopen window. */
const REOPEN_WINDOW_MS = 3 * 24 * HOUR_MS

/**
 * A ticket as the signed-in user gets it: client staff never see the Definition of Done, and
 * only they answer tests and reopen.
 */
function asViewer(t: Ticket): Ticket {
  const requester = isRequester()
  return {
    ...t,
    dod: requester ? null : t.dod,
    uat: t.uat ? { ...t.uat, canAnswer: requester } : null,
    reopen: t.reopen ? { ...t.reopen, canReopen: requester && t.reopen.canReopen } : null,
  }
}

/** Marks a progress step and moves "current" along, roughly as ticket.Progress does. */
function progressAfter(t: Ticket, status: TicketStatus): Ticket['progress'] {
  const now = new Date().toISOString()
  const reached = status === 'resolved' ? 'resolved' : 'in_progress'
  const order = t.progress.map((p) => p.step)
  const at = order.indexOf(reached)
  return t.progress.map((p, i) => {
    if (i < at) return { ...p, state: p.state === 'upcoming' && p.step === 'ready_for_test' ? 'skipped' : 'done', at: p.at ?? now }
    if (i === at) return { ...p, state: status === 'resolved' ? 'done' : 'current', at: now }
    return { ...p, state: 'upcoming', at: null }
  })
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
    uat: tickets.cardUat[key] ?? null,
    code: tickets.cardCode[key] ?? null,
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
  http.get(`${API}/uat-requests`, async ({ request }) => {
    await record(request)
    if (!isRequester()) return problem(403, 'forbidden')
    const me = state.me?.id ?? ''
    const mine = tickets.tickets.filter(
      (t) =>
        t.status === 'in_uat' &&
        visible(t) &&
        (t.requester.id === me || (tickets.watchers[t.key] ?? []).some((w) => w.id === me)),
    )
    return HttpResponse.json(
      mine.map(
        (t): UatRequest => ({
          key: t.key,
          subject: t.subject,
          project: t.project,
          requester: t.requester,
          autoApproveAt: t.uat?.autoApproveAt ?? null,
          instructions: t.uat?.instructions ?? null,
        }),
      ),
    )
  }),
  http.post(`${API}/tickets/:key/uat`, async ({ request, params }) => {
    const body = (await record(request)) as { answer: UatAnswer; message?: string }
    const t = find(params.key)
    if (!t || !visible(t)) return problem(404, 'not_found')
    if (!isRequester()) return problem(403, 'forbidden')
    if (t.status !== 'in_uat') return problem(409, 'uat_not_waiting')
    const now = new Date()
    if (body.answer === 'approve') {
      t.status = 'resolved'
      t.resolvedAt = now.toISOString()
      t.reopen = { until: new Date(now.getTime() + REOPEN_WINDOW_MS).toISOString(), canReopen: true }
    } else {
      if (!body.message?.trim()) return problem(400, 'invalid_message')
      t.status = t.assignee ? 'in_progress' : 'new'
      const me = state.me
      tickets.messages[t.key] = [
        ...(tickets.messages[t.key] ?? []),
        {
          id: newId('m'), author: ref(me?.id ?? '', me?.name ?? ''), authorJobTitle: me?.jobTitle ?? null, fromSupport: false,
          visibility: 'public', body: body.message, attachments: [], createdAt: now.toISOString(),
        },
      ]
    }
    t.progress = progressAfter(t, t.status)
    t.uat = null
    return HttpResponse.json(asViewer(t))
  }),
  http.post(`${API}/tickets/:key/reopen`, async ({ request, params }) => {
    await record(request)
    const t = find(params.key)
    if (!t || !visible(t)) return problem(404, 'not_found')
    if (!isRequester()) return problem(403, 'forbidden')
    if (t.status === 'closed') return problem(409, 'ticket_closed')
    if (t.status !== 'resolved' || !t.reopen?.canReopen) return problem(409, 'reopen_not_allowed')
    t.status = t.assignee ? 'in_progress' : 'new'
    t.resolvedAt = null
    t.reopen = null
    t.progress = progressAfter(t, t.status)
    return HttpResponse.json(asViewer(t))
  }),
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
    const q = (params.get('q') ?? '').toLowerCase()
    const list = tickets.tickets
      .filter(visible)
      .filter((t) => statuses.length === 0 || statuses.includes(t.status))
      .filter((t) => assignee !== 'me' || t.assignee?.id === state.me?.id)
      .filter((t) => assignee !== 'unassigned' || t.assignee === null)
      .filter((t) => !client || t.client.id === client)
      .filter((t) => !q || t.key.toLowerCase() === q || t.subject.toLowerCase().includes(q) || t.client.name.toLowerCase().includes(q))
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
    return t ? HttpResponse.json(asViewer(t)) : problem(404, 'not_found')
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
    const body = (await record(request)) as { status: TicketStatus; from?: TicketStatus; reason?: string }
    const t = find(params.key)
    if (!t) return problem(404, 'not_found')
    if (body.from !== undefined && body.from !== t.status) return problem(409, 'status_changed')
    if (body.status !== t.status && !allowedMoves(t).includes(body.status)) return problem(409, 'invalid_status_change')
    const skipped = body.status === 'resolved' && t.dod != null && !t.dod.met
    if (skipped && !body.reason) return problem(409, 'dod_unmet')
    const { status } = body
    const from = t.status
    t.status = status
    t.resolvedAt = status === 'resolved' || status === 'closed' ? (t.resolvedAt ?? new Date().toISOString()) : null
    t.allowedStatuses = allowedMoves(t)
    tickets.activity[t.key] = [
      ...(tickets.activity[t.key] ?? []),
      {
        id: newId('e'), at: new Date().toISOString(), actorName: state.me?.name ?? null, action: 'status_changed', from, to: status,
        reason: skipped ? (body.reason ?? null) : null, cause: skipped ? 'dod_skipped' : null, channel: null, code: null,
      },
    ]
    return HttpResponse.json(t)
  }),
  http.put(`${API}/tickets/:key/dod`, async ({ request, params }) => {
    const body = (await record(request)) as TicketDodInput
    const t = find(params.key)
    if (!t?.dod) return problem(404, 'not_found')
    if (!t.dod.canOverride) return problem(403, 'dod_override_not_allowed')
    if (body.dod && !body.reason?.trim()) return problem(400, 'reason_required')
    t.dod = body.dod ? ticketDod(t.dod.projectConfig, { config: body.dod, reason: body.reason ?? '' }) : ticketDod(t.dod.projectConfig)
    tickets.activity[t.key] = [
      ...(tickets.activity[t.key] ?? []),
      {
        id: newId('e'), at: new Date().toISOString(), actorName: state.me?.name ?? null, action: 'dod_changed',
        from: body.dod ? 'project' : 'ticket', to: body.dod ? 'ticket' : 'project', reason: body.reason ?? null, cause: null,
        channel: null, code: null,
      },
    ]
    return HttpResponse.json(t)
  }),
  http.put(`${API}/tickets/:key/assignee`, async ({ request, params }) => {
    const { assigneeId } = (await record(request)) as { assigneeId: string | null }
    const t = find(params.key)
    if (!t) return problem(404, 'not_found')
    // Like the API: agents only take an unassigned ticket or release their own.
    const me = state.me
    const current = t.assignee?.id ?? null
    const selfOnly = (current === null && assigneeId === me?.id) || (assigneeId === null && current === me?.id)
    if (assigneeId !== current && !can.assignAnyone(me?.role ?? null) && !selfOnly) return problem(403, 'assignee_not_allowed')
    const a = tickets.assignees.find((x) => x.id === assigneeId)
    t.assignee = a ? ref(a.id, a.name) : me && assigneeId === me.id ? ref(me.id, me.name) : null
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
  http.get(`${API}/tickets/:key/code`, ({ params }) => {
    if (isRequester()) return problem(403, 'forbidden')
    const t = find(params.key)
    if (!t) return problem(404, 'not_found')
    return HttpResponse.json(tickets.code[t.key] ?? { repository: null, requests: [], commits: [] })
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
  http.get(`${API}/clients/:id/requesters`, ({ request, params }) => {
    const all = new URL(request.url).searchParams.get('includeSuspended') === 'true'
    return HttpResponse.json((tickets.requesters[String(params.id)] ?? []).filter((r) => all || r.status !== 'suspended'))
  }),
  http.post(`${API}/clients/:id/requesters`, async ({ request, params }) => {
    const body = (await record(request)) as { name: string; email: string; jobTitle?: string | null }
    const list = (tickets.requesters[String(params.id)] ??= [])
    if (Object.values(tickets.requesters).flat().some((r) => r.email === body.email.toLowerCase())) {
      return problem(409, 'email_taken')
    }
    const r: Requester = { id: newId('r'), name: body.name, email: body.email.toLowerCase(), jobTitle: body.jobTitle ?? null, status: 'invited' }
    list.push(r)
    return HttpResponse.json(r, { status: 201 })
  }),
  http.get(`${API}/clients/:id/assignees`, async ({ request }) => {
    await record(request)
    return HttpResponse.json(tickets.assignees)
  }),
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
