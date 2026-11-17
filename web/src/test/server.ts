import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'

import type { Install, Me, Organization, OrganizationInput, Problem } from '../api/client'
import { integrationsHandlers, resetIntegrations } from './integrationsApi'
import { membersHandlers, resetMembers } from './membersApi'
import { notificationsHandlers, resetNotifications } from './notificationsApi'
import { projectsHandlers, resetProjects } from './projectsApi'
import { reportsHandlers, resetReports } from './reportsApi'
import { resetSettings, settingsHandlers } from './settingsApi'
import { resetTickets, ticketsHandlers } from './ticketsApi'

export const PASSWORD = 'correct-horse-battery'

export const ORG: Organization = {
  name: 'Demo Vendor', timezone: 'Asia/Jakarta', footerNote: 'Data stored in Indonesia', ticketPrefix: 'HD',
  id: '01a1cedf-0000-7000-8000-0000000000e9', appUrl: 'https://helpdesk.demo-vendor.test', logoUrl: null,
}

/** The organization every fake user works in. */
export const ORG_ID = '01a1cedf-0000-7000-8000-0000000000e9'

/** A user working in the demo organization with role (an administrator there too). */
function inOrg(user: Omit<Me, 'isAdmin' | 'organization' | 'organizations' | 'role'> & { role: NonNullable<Me['role']> }): Me {
  const choice = { id: ORG_ID, name: ORG.name, role: user.role, logoUrl: null }
  return { ...user, isAdmin: user.role === 'admin', organization: choice, organizations: [choice] }
}

/** The invite link token the fake API accepts. */
export const INVITE_TOKEN = 'invite-token-0123456789abcdef'

/** Someone who accepted their invite: the API signs them in. */
export const INVITED_ME: Me = inOrg({ id: '01a1cedf-0000-7000-8000-0000000000a1', email: 'yoga@demo-vendor.test', name: 'Yoga Hidayat', role: 'agent' })

/** The logo URL the fake API hands out after an upload. */
export const LOGO_URL = '/api/v1/organization/logo?v=1'

export const USERS: Record<string, Me> = {
  'rina@demo-vendor.test': inOrg({
    id: '01a1cedf-0000-7000-8000-000000000001',
    email: 'rina@demo-vendor.test',
    name: 'Rina Pratiwi',
    role: 'agent',
    jobTitle: 'L2',
  }),
  'admin@demo-vendor.test': inOrg({
    id: '01a1cedf-0000-7000-8000-000000000002',
    email: 'admin@demo-vendor.test',
    name: 'Demo Admin',
    role: 'admin',
  }),
  'dewi@demo-vendor.test': inOrg({
    id: '01a1cedf-0000-7000-8000-000000000004',
    email: 'dewi@demo-vendor.test',
    name: 'Dewi Wulandari',
    role: 'supervisor',
    jobTitle: 'Team lead',
  }),
  'hendra@kominfo-jambi.test': inOrg({
    id: '01a1cedf-0000-7000-8000-000000000003',
    email: 'hendra@kominfo-jambi.test',
    name: 'Hendra Gunawan',
    role: 'requester',
    jobTitle: 'IT Officer',
    client: { id: '01a1cedf-0000-7000-8000-0000000000c1', name: 'Dinas Kominfo Jambi' },
  }),
}

/** Fake API state shared by the handlers; reset after each test. */
export const state: {
  me: Me | null
  install: Install
  org: Organization
  forgotRequests: string[]
  resetBodies: unknown[]
  acceptBodies: unknown[]
  setupBodies: unknown[]
  orgBodies: unknown[]
} = {
  me: null,
  install: { setupRequired: false, appUrl: ORG.appUrl },
  org: { ...ORG },
  forgotRequests: [],
  resetBodies: [],
  acceptBodies: [],
  setupBodies: [],
  orgBodies: [],
}

export function resetApi() {
  state.me = null
  state.install = { setupRequired: false, appUrl: ORG.appUrl }
  state.org = { ...ORG }
  state.forgotRequests = []
  state.resetBodies = []
  state.acceptBodies = []
  state.setupBodies = []
  state.orgBodies = []
  resetSettings()
  resetMembers()
  resetTickets()
  resetProjects()
  resetNotifications()
  resetIntegrations()
  resetReports()
}

export function problem(status: number, code: string, headers?: Record<string, string>) {
  const body: Problem = { type: 'about:blank', title: code, status, code }
  return HttpResponse.json(body, { status, headers: { 'Content-Type': 'application/problem+json', ...headers } })
}

const handlers = [
  http.get('*/api/v1/install', () => HttpResponse.json(state.install)),
  http.get('*/api/v1/organization', () => (state.me ? HttpResponse.json(state.org) : problem(401, 'unauthenticated'))),
  http.put('*/api/v1/organization', async ({ request }) => {
    const body = (await request.json()) as OrganizationInput
    state.orgBodies.push(body)
    state.org = { ...state.org, ...body, ticketPrefix: body.ticketPrefix.toUpperCase() }
    return HttpResponse.json(state.org)
  }),
  http.put('*/api/v1/organization/logo', () => {
    state.org = { ...state.org, logoUrl: LOGO_URL }
    return HttpResponse.json(state.org)
  }),
  http.delete('*/api/v1/organization/logo', () => {
    state.org = { ...state.org, logoUrl: null }
    return HttpResponse.json(state.org)
  }),
  http.get('*/api/v1/auth/me', () => (state.me ? HttpResponse.json(state.me) : problem(401, 'unauthenticated'))),
  http.post('*/api/v1/auth/login', async ({ request }) => {
    const { email, password } = (await request.json()) as { email: string; password: string }
    const user = USERS[email.trim().toLowerCase()]
    if (!user || password !== PASSWORD) return problem(401, 'invalid_credentials')
    state.me = user
    return HttpResponse.json(user)
  }),
  http.post('*/api/v1/auth/logout', () => {
    state.me = null
    return new HttpResponse(null, { status: 204 })
  }),
  http.post('*/api/v1/auth/password/forgot', async ({ request }) => {
    state.forgotRequests.push(((await request.json()) as { email: string }).email)
    return new HttpResponse(null, { status: 202 })
  }),
  http.post('*/api/v1/auth/password/reset', async ({ request }) => {
    const body = (await request.json()) as { token: string }
    state.resetBodies.push(body)
    return body.token === 'valid-token-0123456789abcdef'
      ? new HttpResponse(null, { status: 204 })
      : problem(400, 'invalid_token')
  }),
  http.post('*/api/v1/setup', async ({ request }) => {
    const body = (await request.json()) as { organizationName: string; name: string; email: string }
    state.setupBodies.push(body)
    if (!state.install.setupRequired) return problem(409, 'setup_closed')
    state.install = { ...state.install, setupRequired: false }
    state.org = { ...state.org, name: body.organizationName }
    state.me = inOrg({ id: '01a1cedf-0000-7000-8000-0000000000f1', email: body.email, name: body.name, role: 'admin' })
    return HttpResponse.json(state.me)
  }),
  http.post('*/api/v1/auth/invite/accept', async ({ request }) => {
    const body = (await request.json()) as { token: string }
    state.acceptBodies.push(body)
    if (body.token !== INVITE_TOKEN) return problem(400, 'invalid_token')
    state.me = INVITED_ME
    return HttpResponse.json(INVITED_ME)
  }),
]

export const server = setupServer(
  ...handlers,
  ...settingsHandlers,
  ...membersHandlers,
  ...ticketsHandlers,
  ...projectsHandlers,
  ...notificationsHandlers,
  ...integrationsHandlers,
  ...reportsHandlers,
)
