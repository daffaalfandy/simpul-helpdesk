import { http, HttpResponse } from 'msw'

import type { Member, MemberInvite, Members, StaffRole } from '../api/client'
import { tickets } from './ticketsApi'

const API = '*/api/v1'

export const MEMBER_IDS = {
  admin: '01a1cedf-0000-7000-8000-000000000002',
  rina: '01a1cedf-0000-7000-8000-000000000001',
  yoga: '01a1cedf-0000-7000-8000-0000000000a1',
  bayu: '01a1cedf-0000-7000-8000-0000000000a2',
} as const

function fixtures() {
  const team: Member[] = [
    {
      id: MEMBER_IDS.admin, email: 'admin@demo-vendor.test', name: 'Demo Admin', role: 'admin', jobTitle: null, status: 'active',
      lastLoginAt: '2026-11-16T02:00:00Z', inviteSentAt: null,
    },
    {
      id: MEMBER_IDS.rina, email: 'rina@demo-vendor.test', name: 'Rina Pratiwi', role: 'agent', jobTitle: 'L2', status: 'active',
      lastLoginAt: '2026-11-15T03:00:00Z', inviteSentAt: null,
    },
    {
      id: MEMBER_IDS.yoga, email: 'yoga@demo-vendor.test', name: 'Yoga Hidayat', role: 'agent', jobTitle: null, status: 'invited',
      lastLoginAt: null, inviteSentAt: '2026-11-14T03:00:00Z',
    },
    {
      id: MEMBER_IDS.bayu, email: 'bayu@demo-vendor.test', name: 'Bayu Nugroho', role: 'supervisor', jobTitle: null, status: 'suspended',
      lastLoginAt: '2026-10-01T03:00:00Z', inviteSentAt: null,
    },
  ]
  return { team, requesters: 46, actions: [] as string[] }
}

/** Fake team data; reset after each test. */
export const members = fixtures()

function problem(status: number, code: string) {
  return HttpResponse.json(
    { type: 'about:blank', title: code, status, code },
    { status, headers: { 'Content-Type': 'application/problem+json' } },
  )
}

export function resetMembers() {
  Object.assign(members, fixtures())
}

function list(): Members {
  const count = (r: StaffRole) => members.team.filter((m) => m.role === r && m.status !== 'suspended').length
  return {
    counts: { admin: count('admin'), supervisor: count('supervisor'), agent: count('agent'), requester: members.requesters },
    team: members.team,
  }
}

export const membersHandlers = [
  http.get(`${API}/members`, () => HttpResponse.json(list())),
  http.post(`${API}/members`, async ({ request }) => {
    const body = (await request.json()) as MemberInvite
    if (members.team.some((m) => m.email === body.email)) return problem(409, 'email_taken')
    members.team.push({
      id: `01a1cedf-0000-7000-8000-0000000000b${members.team.length}`, email: body.email, name: body.name, role: body.role,
      jobTitle: body.jobTitle ?? null, status: 'invited', lastLoginAt: null, inviteSentAt: '2026-11-17T03:00:00Z',
    })
    return new HttpResponse(null, { status: 201 })
  }),
  http.put(`${API}/members/:id/role`, async ({ request, params }) => {
    const { role } = (await request.json()) as { role: StaffRole }
    const m = members.team.find((x) => x.id === params.id)
    if (!m) return problem(404, 'not_found')
    m.role = role
    members.actions.push(`role:${m.name}:${role}`)
    return new HttpResponse(null, { status: 204 })
  }),
  // The actions work for client requesters too (Settings › Clients).
  http.post(`${API}/members/:id/:action`, ({ params }) => {
    const people: Array<Pick<Member, 'id' | 'name' | 'status'> & { lastLoginAt?: string | null; inviteSentAt?: string | null }> = [
      ...members.team,
      ...Object.values(tickets.requesters).flat(),
    ]
    const m = people.find((x) => x.id === params.id)
    if (!m) return problem(404, 'not_found')
    members.actions.push(`${String(params.action)}:${m.name}`)
    switch (params.action) {
      case 'suspend':
        m.status = 'suspended'
        break
      case 'reactivate':
        m.status = m.lastLoginAt === null ? 'invited' : 'active'
        break
      case 'invite':
        if (m.status !== 'invited') return problem(409, 'not_invited')
        m.inviteSentAt = '2026-11-17T03:00:00Z'
        break
      default:
        return problem(404, 'not_found')
    }
    return new HttpResponse(null, { status: 204 })
  }),
]

