import { http, HttpResponse } from 'msw'

import type { BusinessHours, Client, Holiday, HolidayPreset, SlaPlan, SlaTarget, User } from '../api/client'

const API = '*/api/v1'
const WORKING_DAY = 540
const MINUTES_PER_HOUR = 60

export const IDS = {
  standard: '01a1cedf-0000-7000-8000-00000000a001',
  gold: '01a1cedf-0000-7000-8000-00000000a002',
  jambi: '01a1cedf-0000-7000-8000-0000000000c1',
  batam: '01a1cedf-0000-7000-8000-0000000000c2',
  rina: '01a1cedf-0000-7000-8000-000000000001',
  adi: '01a1cedf-0000-7000-8000-000000000004',
  holiday: '01a1cedf-0000-7000-8000-00000000d001',
} as const

export const THIS_YEAR = new Date().getFullYear()

export const standardTargets: SlaTarget[] = [
  { priority: 'P1', firstResponseMinutes: 15, resolveMinutes: 240, deliverMinutes: 240, deliverFromCatalog: false, calendar: 'always' },
  { priority: 'P2', firstResponseMinutes: 30, resolveMinutes: 480, deliverMinutes: 540, deliverFromCatalog: false, calendar: 'business_hours' },
  { priority: 'P3', firstResponseMinutes: 120, resolveMinutes: 1080, deliverMinutes: 1080, deliverFromCatalog: true, calendar: 'business_hours' },
  { priority: 'P4', firstResponseMinutes: 240, resolveMinutes: 2700, deliverMinutes: 2700, deliverFromCatalog: false, calendar: 'business_hours' },
]

function fixtures() {
  const plans: SlaPlan[] = [
    { id: IDS.standard, name: 'Standard', isDefault: true, clientCount: 1, targets: standardTargets },
    {
      id: IDS.gold,
      name: 'Gold',
      isDefault: false,
      clientCount: 1,
      targets: standardTargets.map((t) => (t.priority === 'P2' ? { ...t, calendar: 'always' } : t)),
    },
  ]
  const clients: Client[] = [
    {
      id: IDS.batam,
      name: 'BPKAD Kota Batam',
      contractNo: 'KTR/BPKAD-BTM/014/2026',
      contractStart: '2026-03-01',
      contractEnd: '2027-02-28',
      picName: 'Sri Lestari',
      picEmail: 'sri@batam.test',
      slaPlan: { id: IDS.standard, name: 'Standard' },
      requesterCount: 0,
      agents: [],
      createdAt: '2026-11-12T00:00:00Z',
      updatedAt: '2026-11-12T00:00:00Z',
    },
    {
      id: IDS.jambi,
      name: 'Dinas Kominfo Jambi',
      contractNo: 'KTR/KOMINFO-JMB/006/2026',
      contractStart: '2026-01-01',
      contractEnd: '2026-12-31',
      picName: 'Hendra Gunawan',
      picEmail: null,
      slaPlan: { id: IDS.gold, name: 'Gold' },
      requesterCount: 46,
      agents: [{ id: IDS.rina, name: 'Rina Pratiwi' }],
      createdAt: '2026-11-12T00:00:00Z',
      updatedAt: '2026-11-12T00:00:00Z',
    },
  ]
  const hours: BusinessHours = {
    timezone: 'Asia/Jakarta',
    weekdays: ['mon', 'tue', 'wed', 'thu', 'fri'],
    opens: '08:00',
    closes: '17:00',
    workingDayMinutes: WORKING_DAY,
  }
  const holidays: Holiday[] = [{ id: IDS.holiday, date: `${THIS_YEAR}-06-01`, name: 'Hari Lahir Pancasila' }]
  const presets: HolidayPreset[] = [{ year: THIS_YEAR, country: 'ID', holidayCount: 25, missingCount: 3 }]
  const users: User[] = [
    { id: IDS.adi, email: 'adi@demo-vendor.test', name: 'Adi Saputra', role: 'agent', status: 'active', jobTitle: 'L2' },
    { id: IDS.rina, email: 'rina@demo-vendor.test', name: 'Rina Pratiwi', role: 'agent', status: 'active', jobTitle: 'L2' },
    { id: 'u-dewi', email: 'dewi@demo-vendor.test', name: 'Dewi Wulandari', role: 'supervisor', status: 'active' },
  ]
  return { plans, clients, hours, holidays, presets, users }
}

type Recorded = { method: string; path: string; body: unknown }

/** Fake settings data; reset after each test. */
export const settings = { ...fixtures(), requests: [] as Recorded[] }

export function resetSettings() {
  Object.assign(settings, fixtures(), { requests: [] })
}

function problem(status: number, code: string) {
  return HttpResponse.json(
    { type: 'about:blank', title: code, status, code },
    { status, headers: { 'Content-Type': 'application/problem+json' } },
  )
}

async function record(request: Request) {
  const text = await request.text()
  const body: unknown = text ? JSON.parse(text) : undefined
  settings.requests.push({ method: request.method, path: new URL(request.url).pathname, body })
  return body
}

let nextId = 0
const newId = () => `01a1cedf-0000-7000-8000-${String(++nextId).padStart(12, '0')}`

function planRef(id: string) {
  const p = settings.plans.find((x) => x.id === id)
  return { id, name: p?.name ?? '' }
}

export const settingsHandlers = [
  http.get(`${API}/clients`, () => HttpResponse.json(settings.clients)),
  http.post(`${API}/clients`, async ({ request }) => {
    const body = (await record(request)) as Client & { slaPlanId: string }
    if (settings.clients.some((c) => c.name === body.name)) return problem(409, 'client_name_taken')
    const client: Client = {
      ...body,
      id: newId(),
      contractNo: body.contractNo ?? null,
      contractStart: body.contractStart ?? null,
      contractEnd: body.contractEnd ?? null,
      picName: body.picName ?? null,
      picEmail: body.picEmail ?? null,
      slaPlan: planRef(body.slaPlanId),
      requesterCount: 0,
      agents: [],
      createdAt: '2026-11-12T00:00:00Z',
      updatedAt: '2026-11-12T00:00:00Z',
    }
    settings.clients.push(client)
    return HttpResponse.json(client, { status: 201 })
  }),
  http.put(`${API}/clients/:id`, async ({ request, params }) => {
    const body = (await record(request)) as Client & { slaPlanId: string }
    const c = settings.clients.find((x) => x.id === params.id)
    if (!c) return problem(404, 'not_found')
    Object.assign(c, body, { slaPlan: planRef(body.slaPlanId) })
    return HttpResponse.json(c)
  }),
  http.delete(`${API}/clients/:id`, async ({ request, params }) => {
    await record(request)
    const c = settings.clients.find((x) => x.id === params.id)
    if (!c) return problem(404, 'not_found')
    if (c.requesterCount > 0) return problem(409, 'client_in_use')
    settings.clients = settings.clients.filter((x) => x !== c)
    return new HttpResponse(null, { status: 204 })
  }),
  http.put(`${API}/clients/:id/agents`, async ({ request, params }) => {
    const { userIds } = (await record(request)) as { userIds: string[] }
    const c = settings.clients.find((x) => x.id === params.id)
    if (!c) return problem(404, 'not_found')
    c.agents = settings.users.filter((u) => userIds.includes(u.id)).map((u) => ({ id: u.id, name: u.name }))
    return HttpResponse.json(c)
  }),
  http.get(`${API}/users`, () => HttpResponse.json(settings.users)),
  http.get(`${API}/sla-plans`, () => HttpResponse.json(settings.plans)),
  http.post(`${API}/sla-plans`, async ({ request }) => {
    const body = (await record(request)) as Pick<SlaPlan, 'name' | 'targets'>
    if (settings.plans.some((p) => p.name.toLowerCase() === body.name.toLowerCase())) return problem(409, 'sla_plan_name_taken')
    const plan: SlaPlan = { ...body, id: newId(), isDefault: false, clientCount: 0 }
    settings.plans.push(plan)
    return HttpResponse.json(plan, { status: 201 })
  }),
  http.put(`${API}/sla-plans/:id`, async ({ request, params }) => {
    const body = (await record(request)) as Pick<SlaPlan, 'name' | 'targets'>
    const p = settings.plans.find((x) => x.id === params.id)
    if (!p) return problem(404, 'not_found')
    Object.assign(p, body)
    return HttpResponse.json(p)
  }),
  http.delete(`${API}/sla-plans/:id`, async ({ request, params }) => {
    await record(request)
    const p = settings.plans.find((x) => x.id === params.id)
    if (!p) return problem(404, 'not_found')
    if (p.isDefault) return problem(409, 'sla_plan_is_default')
    if (p.clientCount > 0) return problem(409, 'sla_plan_in_use')
    settings.plans = settings.plans.filter((x) => x !== p)
    return new HttpResponse(null, { status: 204 })
  }),
  http.post(`${API}/sla-plans/:id/default`, async ({ request, params }) => {
    await record(request)
    const p = settings.plans.find((x) => x.id === params.id)
    if (!p) return problem(404, 'not_found')
    settings.plans.forEach((x) => (x.isDefault = x === p))
    return HttpResponse.json(p)
  }),
  http.get(`${API}/business-hours`, () => HttpResponse.json(settings.hours)),
  http.put(`${API}/business-hours`, async ({ request }) => {
    const body = (await record(request)) as Pick<BusinessHours, 'weekdays' | 'opens' | 'closes'>
    const minutes = (s: string) => Number(s.slice(0, 2)) * MINUTES_PER_HOUR + Number(s.slice(3))
    settings.hours = { ...settings.hours, ...body, workingDayMinutes: minutes(body.closes) - minutes(body.opens) }
    return HttpResponse.json(settings.hours)
  }),
  http.get(`${API}/holidays`, ({ request }) => {
    const year = new URL(request.url).searchParams.get('year')
    return HttpResponse.json(settings.holidays.filter((h) => !year || h.date.startsWith(year)))
  }),
  http.post(`${API}/holidays`, async ({ request }) => {
    const body = (await record(request)) as Omit<Holiday, 'id'>
    if (settings.holidays.some((h) => h.date === body.date)) return problem(409, 'holiday_date_taken')
    const h = { ...body, id: newId() }
    settings.holidays.push(h)
    return HttpResponse.json(h, { status: 201 })
  }),
  http.delete(`${API}/holidays/:id`, async ({ request, params }) => {
    await record(request)
    settings.holidays = settings.holidays.filter((h) => h.id !== params.id)
    return new HttpResponse(null, { status: 204 })
  }),
  http.get(`${API}/holiday-presets`, () => HttpResponse.json(settings.presets)),
  http.post(`${API}/holiday-presets/:year/load`, async ({ request, params }) => {
    await record(request)
    const p = settings.presets.find((x) => x.year === Number(params.year))
    if (!p) return problem(404, 'not_found')
    const added = p.missingCount
    p.missingCount = 0
    return HttpResponse.json({ year: p.year, addedCount: added, skippedCount: p.holidayCount - added })
  }),
]
