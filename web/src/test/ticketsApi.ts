import { delay, http, HttpResponse } from 'msw'

import type {
  Assignee,
  Attachment,
  AttachmentPolicy,
  CatalogItem,
  Intake,
  Requester,
  Ticket,
  TicketActivity,
  TicketMessage,
  TicketStatus,
  TicketSummary,
} from '../api/client'
import { state } from './server'
import { defaultTargets, PROJECT_IDS, projectsApi } from './projectsApi'
import { IDS, settings } from './settingsApi'

const API = '*/api/v1'
const MAX_BYTES = 10 * 1024 * 1024

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
      pauseOnUat: true,
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
        pauseOnUat: true,
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
  return { tickets, messages, activity, catalogItems, requesters, assignees }
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

export const ticketsHandlers = [
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
        pauseOnUat: true,
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
    const { status } = (await record(request)) as { status: TicketStatus }
    const t = find(params.key)
    if (!t) return problem(404, 'not_found')
    const from = t.status
    t.status = status
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
