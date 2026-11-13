import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'

import type { Me, Organization, Problem } from '../api/client'
import { resetSettings, settingsHandlers } from './settingsApi'
import { resetTickets, ticketsHandlers } from './ticketsApi'

export const PASSWORD = 'correct-horse-battery'

export const ORG: Organization = { name: 'Demo Vendor', timezone: 'Asia/Jakarta', footerNote: 'Data stored in Indonesia' }

export const USERS: Record<string, Me> = {
  'rina@demo-vendor.test': {
    id: '01a1cedf-0000-7000-8000-000000000001',
    email: 'rina@demo-vendor.test',
    name: 'Rina Pratiwi',
    role: 'agent',
    jobTitle: 'L2',
  },
  'admin@demo-vendor.test': {
    id: '01a1cedf-0000-7000-8000-000000000002',
    email: 'admin@demo-vendor.test',
    name: 'Demo Admin',
    role: 'admin',
  },
  'hendra@kominfo-jambi.test': {
    id: '01a1cedf-0000-7000-8000-000000000003',
    email: 'hendra@kominfo-jambi.test',
    name: 'Hendra Gunawan',
    role: 'requester',
    jobTitle: 'IT Officer',
    client: { id: '01a1cedf-0000-7000-8000-0000000000c1', name: 'Dinas Kominfo Jambi' },
  },
}

/** Fake API state shared by the handlers; reset after each test. */
export const state: { me: Me | null; forgotRequests: string[]; resetBodies: unknown[] } = {
  me: null,
  forgotRequests: [],
  resetBodies: [],
}

export function resetApi() {
  state.me = null
  state.forgotRequests = []
  state.resetBodies = []
  resetSettings()
  resetTickets()
}

export function problem(status: number, code: string, headers?: Record<string, string>) {
  const body: Problem = { type: 'about:blank', title: code, status, code }
  return HttpResponse.json(body, { status, headers: { 'Content-Type': 'application/problem+json', ...headers } })
}

const handlers = [
  http.get('*/api/v1/organization', () => HttpResponse.json(ORG)),
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
]

export const server = setupServer(...handlers, ...settingsHandlers, ...ticketsHandlers)
