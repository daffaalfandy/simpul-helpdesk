import { http, HttpResponse } from 'msw'

import type { PriorityLevel, Project, ProjectCreate, ProjectSla, ProjectTarget, ProjectUpdate } from '../api/client'
import { IDS } from './settingsApi'

const API = '*/api/v1'

export const PROJECT_IDS = {
  eOffice: '01a1cedf-0000-7000-8000-00000000e001',
  email: '01a1cedf-0000-7000-8000-00000000e002',
  network: '01a1cedf-0000-7000-8000-00000000e003',
} as const

export const defaultTargets: ProjectTarget[] = [
  { priority: 'P1', firstResponse: { value: 15, unit: 'min' }, resolve: { value: 4, unit: 'h' }, deliver: { value: 4, unit: 'h' }, calendar: 'always' },
  { priority: 'P2', firstResponse: { value: 30, unit: 'min' }, resolve: { value: 8, unit: 'h' }, deliver: { value: 1, unit: 'wd' }, calendar: 'business_hours' },
  { priority: 'P3', firstResponse: { value: 2, unit: 'h' }, resolve: { value: 2, unit: 'wd' }, deliver: { value: 2, unit: 'wd' }, calendar: 'business_hours' },
  { priority: 'P4', firstResponse: { value: 4, unit: 'h' }, resolve: { value: 5, unit: 'wd' }, deliver: { value: 5, unit: 'wd' }, calendar: 'business_hours' },
]

export const defaultSla: ProjectSla = {
  targets: defaultTargets,
  pauseOnWaitingClient: true,
  pauseOnUat: true,
  atRiskPercent: 75,
  notifySupervisorOnBreach: true,
  notifyPicOnP1Breach: false,
}

const fastSla: ProjectSla = {
  ...defaultSla,
  targets: defaultTargets.map((t) => (t.priority === 'P1' ? { ...t, resolve: { value: 2, unit: 'h' }, deliver: { value: 2, unit: 'h' } } : t)),
  atRiskPercent: 50,
  notifyPicOnP1Breach: true,
}

const STAMP = '2026-11-13T00:00:00Z'
const JAMBI = { id: IDS.jambi, name: 'Dinas Kominfo Jambi' }
const BATAM = { id: IDS.batam, name: 'BPKAD Kota Batam' }

function fixtures() {
  const levels: PriorityLevel[] = [
    { code: 'P1', name: 'Critical', description: 'A public service or the whole office is down' },
    { code: 'P2', name: 'High', description: 'A team or department cannot work' },
    { code: 'P3', name: 'Standard', description: 'One person is slowed down, with a workaround' },
    { code: 'P4', name: 'Low', description: 'Minor issue or question, no rush' },
  ]
  const projects: Project[] = [
    {
      id: PROJECT_IDS.eOffice, client: JAMBI, name: 'e-Office', description: 'Letters, dispositions, archives',
      isDefault: true, active: true, sla: fastSla, openTicketCount: 2, createdAt: STAMP, updatedAt: STAMP,
    },
    {
      id: PROJECT_IDS.email, client: JAMBI, name: 'Email', description: 'Mailboxes and accounts',
      isDefault: false, active: true, sla: defaultSla, openTicketCount: 0, createdAt: STAMP, updatedAt: STAMP,
    },
    {
      id: PROJECT_IDS.network, client: BATAM, name: 'Network & infra support', description: null,
      isDefault: true, active: true, sla: defaultSla, openTicketCount: 1, createdAt: STAMP, updatedAt: STAMP,
    },
  ]
  return { levels, projects }
}

type Recorded = { method: string; path: string; body: unknown }

/** Fake priority and project data; reset after each test. */
export const projectsApi = { ...fixtures(), requests: [] as Recorded[] }

export function resetProjects() {
  Object.assign(projectsApi, fixtures(), { requests: [] })
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
  projectsApi.requests.push({ method: request.method, path: new URL(request.url).pathname, body })
  return body
}

let nextId = 0
const newId = () => `01a1cedf-0000-7000-8000-${String(++nextId).padStart(12, 'f')}`

function find(id: string) {
  return projectsApi.projects.find((p) => p.id === id)
}

export const projectsHandlers = [
  http.get(`${API}/priorities`, () => HttpResponse.json(projectsApi.levels)),
  http.put(`${API}/priorities`, async ({ request }) => {
    projectsApi.levels = (await record(request)) as PriorityLevel[]
    return HttpResponse.json(projectsApi.levels)
  }),
  http.get(`${API}/projects`, ({ request }) => {
    const clientId = new URL(request.url).searchParams.get('clientId')
    return HttpResponse.json(projectsApi.projects.filter((p) => !clientId || p.client.id === clientId))
  }),
  http.get(`${API}/projects/:id`, ({ params }) => {
    const p = find(String(params.id))
    return p ? HttpResponse.json(p) : problem(404, 'not_found')
  }),
  http.post(`${API}/projects`, async ({ request }) => {
    const body = (await record(request)) as ProjectCreate
    if (projectsApi.projects.some((p) => p.client.id === body.clientId && p.name === body.name)) {
      return problem(409, 'project_name_taken')
    }
    const client = body.clientId === IDS.batam ? BATAM : JAMBI
    const first = !projectsApi.projects.some((p) => p.client.id === body.clientId)
    const project: Project = {
      id: newId(), client, name: body.name, description: body.description ?? null,
      isDefault: body.isDefault || first, active: body.active || first, sla: defaultSla, openTicketCount: 0,
      createdAt: STAMP, updatedAt: STAMP,
    }
    projectsApi.projects.push(project)
    return HttpResponse.json(project, { status: 201 })
  }),
  http.put(`${API}/projects/:id`, async ({ request, params }) => {
    const body = (await record(request)) as ProjectUpdate
    const p = find(String(params.id))
    if (!p) return problem(404, 'not_found')
    if (p.isDefault && !body.isDefault) return problem(409, 'project_is_default')
    if (body.isDefault) {
      for (const other of projectsApi.projects) if (other.client.id === p.client.id) other.isDefault = false
    }
    Object.assign(p, { name: body.name, description: body.description ?? null, isDefault: body.isDefault, active: body.active })
    return HttpResponse.json(p)
  }),
  http.delete(`${API}/projects/:id`, async ({ request, params }) => {
    await record(request)
    projectsApi.projects = projectsApi.projects.filter((p) => p.id !== params.id)
    return new HttpResponse(null, { status: 204 })
  }),
  http.put(`${API}/projects/:id/sla`, async ({ request, params }) => {
    const body = (await record(request)) as ProjectSla
    const p = find(String(params.id))
    if (!p) return problem(404, 'not_found')
    p.sla = body
    return HttpResponse.json(p)
  }),
]
