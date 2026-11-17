import { http, HttpResponse } from 'msw'

import type { CreatedPersonalToken, PersonalToken, PersonalTokenInput } from '../api/client'
import { ORG } from './server'

const API = '*/api/v1'

/** The secret the fake API hands out for a new token. */
export const NEW_SECRET = 'simdesk_pat_test0123456789abcdWXYZ'

function fixtures() {
  const items: PersonalToken[] = [
    {
      id: '01a1cedf-0000-7000-8000-0000000000c1', name: 'Claude Code on my laptop', lastFour: 'q9Zx', access: 'write',
      createdAt: '2026-11-10T02:00:00Z', expiresAt: '2027-02-08T02:00:00Z', lastUsedAt: '2026-11-16T03:00:00Z',
    },
    {
      id: '01a1cedf-0000-7000-8000-0000000000c2', name: 'Old desktop', lastFour: 'Ab12', access: 'read',
      createdAt: '2026-08-01T02:00:00Z', expiresAt: '2026-08-31T02:00:00Z', lastUsedAt: null,
    },
  ]
  return { items, created: [] as PersonalTokenInput[], revoked: [] as string[] }
}

/** Fake personal access tokens; reset after each test. */
export const tokens = fixtures()

export function resetTokens() {
  Object.assign(tokens, fixtures())
}

export const accountHandlers = [
  http.get(`${API}/me/tokens`, () => HttpResponse.json({ mcpUrl: `${ORG.appUrl}/api/mcp`, items: tokens.items })),
  http.post(`${API}/me/tokens`, async ({ request }) => {
    const body = (await request.json()) as PersonalTokenInput
    tokens.created.push(body)
    const token: PersonalToken = {
      id: '01a1cedf-0000-7000-8000-0000000000c3', name: body.name, lastFour: NEW_SECRET.slice(-4), access: body.access,
      createdAt: '2026-11-17T02:00:00Z', expiresAt: '2027-02-15T02:00:00Z', lastUsedAt: null,
    }
    tokens.items = [token, ...tokens.items]
    const out: CreatedPersonalToken = { token, secret: NEW_SECRET }
    return HttpResponse.json(out, { status: 201 })
  }),
  http.delete(`${API}/me/tokens/:tokenId`, ({ params }) => {
    tokens.revoked.push(String(params.tokenId))
    tokens.items = tokens.items.filter((t) => t.id !== params.tokenId)
    return new HttpResponse(null, { status: 204 })
  }),
]
