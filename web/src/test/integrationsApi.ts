import { http, HttpResponse } from 'msw'

import type { CodeEventLog, CommitRefAction, CommitTestRef, Repository, RepositoryCreate } from '../api/client'
import { PROJECT_IDS, projectsApi } from './projectsApi'
import { problem } from './server'

const API = '*/api/v1'
const WEBHOOK_BASE = 'http://localhost:5173/api/v1/webhooks/'
export const DEMO_SECRET = 'S3CRETS3CRETS3CRETS3CRETAB'

export const REPO_IDS = { eOffice: 'r-eoffice', sipd: 'r-sipd' } as const

type Recorded = { method: string; path: string; body: unknown }

function fixtures() {
  const repositories: Repository[] = [
    {
      id: REPO_IDS.eOffice, provider: 'github', host: 'github.com', fullName: 'vendor-id/eoffice-jambi',
      webUrl: 'https://github.com/vendor-id/eoffice-jambi', webhookUrl: WEBHOOK_BASE + REPO_IDS.eOffice, status: 'connected',
      lastDeliveryAt: '2026-11-16T02:05:00Z', lastError: null, lastErrorAt: null,
      projects: [{ id: PROJECT_IDS.eOffice, name: 'e-Office', client: { id: 'c1', name: 'Dinas Kominfo Jambi' } }],
    },
    {
      id: REPO_IDS.sipd, provider: 'gitlab', host: 'gitlab.vendor.co.id', fullName: 'gov/sipd-connector',
      webUrl: 'https://gitlab.vendor.co.id/gov/sipd-connector', webhookUrl: WEBHOOK_BASE + REPO_IDS.sipd, status: 'failing',
      lastDeliveryAt: '2026-11-15T02:00:00Z', lastError: 'bad_signature', lastErrorAt: '2026-11-16T03:10:00Z', projects: [],
    },
  ]
  const events: CodeEventLog[] = [
    {
      id: 'l-1', at: '2026-11-16T02:05:00Z', provider: 'github', repository: 'vendor-id/eoffice-jambi', kind: 'pr_opened', ref: '42',
      title: 'Revert SSO token TTL change', branch: 'fix/HD-24871-sso', target: 'main', url: 'u', ticketKey: 'HD-24871',
      ticketFound: true, outcome: 'in_review', items: [], statusFrom: null, statusTo: null,
    },
    {
      id: 'l-2', at: '2026-11-16T01:30:00Z', provider: 'gitlab', repository: 'gov/sipd-connector', kind: 'pr_merged', ref: '57',
      title: 'SIPD export streams large files', branch: 'fix/HD-24826-export', target: 'release/2026.11', url: 'u',
      ticketKey: 'HD-24826', ticketFound: true, outcome: 'merged', items: ['merged'], statusFrom: 'in_progress', statusTo: 'in_uat',
    },
    {
      id: 'l-3', at: '2026-11-16T00:58:00Z', provider: 'github', repository: 'vendor-id/eoffice-jambi', kind: 'commit', ref: 'f00ba12',
      title: 'refs HD-99999 clean up old session keys', branch: 'main', target: '', url: 'u', ticketKey: 'HD-99999',
      ticketFound: false, outcome: 'unknown_ticket', items: [], statusFrom: null, statusTo: null,
    },
  ]
  return { repositories, events, requests: [] as Recorded[] }
}

export let integrationsApi = fixtures()

export function resetIntegrations() {
  integrationsApi = fixtures()
}

async function record(request: Request) {
  const text = await request.text()
  const body: unknown = text ? JSON.parse(text) : undefined
  integrationsApi.requests.push({ method: request.method, path: new URL(request.url).pathname, body })
  return body
}

const KEYWORDS: Record<string, CommitRefAction> = { fix: 'fix', fixes: 'fix', closes: 'fix', close: 'fix', reopen: 'reopen' }
const EXISTING = ['HD-24871', 'HD-24830', 'HD-24859']

/** A rough stand-in for the API's parser: enough for the tester's states. */
function parse(message: string): CommitTestRef[] {
  const out: CommitTestRef[] = []
  for (const m of message.matchAll(/(?:\b(\w+)[\s:(]+)?\b(HD-[1-9]\d*)\b/gi)) {
    const key = (m[2] ?? '').toUpperCase()
    if (out.some((r) => r.key === key)) continue
    out.push({ key, action: KEYWORDS[(m[1] ?? '').toLowerCase()] ?? 'link', ticketExists: EXISTING.includes(key) })
  }
  return out
}

const find = (id: unknown) => integrationsApi.repositories.find((r) => r.id === id)

export const integrationsHandlers = [
  http.get(`${API}/integrations`, () => HttpResponse.json({ repositories: integrationsApi.repositories, ticketPrefix: 'HD' })),
  http.get(`${API}/integrations/events`, () => HttpResponse.json(integrationsApi.events)),
  http.post(`${API}/integrations/commit-test`, async ({ request }) => {
    const { message } = (await request.json()) as { message: string }
    return HttpResponse.json({ refs: parse(message) })
  }),
  http.post(`${API}/repositories`, async ({ request }) => {
    const body = (await record(request)) as RepositoryCreate
    const match = /^https?:\/\/([^/]+)\/(.+?)\/?$/.exec(body.url)
    if (!match) return problem(400, 'invalid_repository_url')
    const [, host = '', fullName = ''] = match
    if (integrationsApi.repositories.some((r) => r.fullName === fullName)) return problem(409, 'repository_taken')
    const id = `r-${integrationsApi.repositories.length + 1}`
    const repo: Repository = {
      id, provider: body.provider, host, fullName, webUrl: body.url, webhookUrl: WEBHOOK_BASE + id, status: 'waiting',
      lastDeliveryAt: null, lastError: null, lastErrorAt: null,
      projects: projectsApi.projects
        .filter((p) => body.projectIds.includes(p.id))
        .map((p) => ({ id: p.id, name: p.name, client: p.client })),
    }
    integrationsApi.repositories.push(repo)
    return HttpResponse.json({ repository: repo, secret: DEMO_SECRET }, { status: 201 })
  }),
  http.patch(`${API}/repositories/:id`, async ({ request, params }) => {
    const body = (await record(request)) as { projectIds: string[] }
    const repo = find(params.id)
    if (!repo) return problem(404, 'not_found')
    repo.projects = projectsApi.projects
      .filter((p) => body.projectIds.includes(p.id))
      .map((p) => ({ id: p.id, name: p.name, client: p.client }))
    return HttpResponse.json(repo)
  }),
  http.post(`${API}/repositories/:id/secret`, async ({ request, params }) => {
    await record(request)
    const repo = find(params.id)
    return repo ? HttpResponse.json({ repository: repo, secret: 'N3WS3CRETN3WS3CRETN3WS3CRE' }) : problem(404, 'not_found')
  }),
  http.delete(`${API}/repositories/:id`, async ({ request, params }) => {
    await record(request)
    if (!find(params.id)) return problem(404, 'not_found')
    integrationsApi.repositories = integrationsApi.repositories.filter((r) => r.id !== params.id)
    return new HttpResponse(null, { status: 204 })
  }),
]
