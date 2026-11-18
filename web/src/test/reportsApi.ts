import { http, HttpResponse } from 'msw'

import type { SlaFigures, SlaReport, SlaReportProject } from '../api/client'
import { problem } from './server'

const API = '*/api/v1'

export const REPORT_PROJECT_IDS = { eOffice: 'p-eoffice', website: 'p-website', legacy: 'p-legacy', network: 'p-network' } as const
export const CURRENT_MONTH = '2026-11'
/** The earliest month the API serves: 12 before the current one. */
export const OLDEST_MONTH = '2025-11'

const JAMBI = { id: 'c-jambi', name: 'Dinas Kominfo Jambi' }
const RIAU = { id: 'c-riau', name: 'Bappeda Prov. Riau' }
const SETDA = { id: 'c-setda', name: 'Setda Kab. Muaro Jambi' }

const NO_TICKETS: SlaFigures = {
  tickets: 0, incidents: 0, requests: 0, breaches: 0, withinSlaPercent: null, avgFirstResponseMs: null,
}

function project(
  id: string,
  name: string,
  client: { id: string; name: string },
  figures: SlaFigures,
  { active = true, belowTarget = false } = {},
): SlaReportProject {
  return { id, name, client, active, figures, belowTarget }
}

/** Every active project, none with tickets: a month before the fixtures. */
function emptyMonth(): SlaReportProject[] {
  return [
    project(REPORT_PROJECT_IDS.website, 'Website', RIAU, NO_TICKETS),
    project(REPORT_PROJECT_IDS.eOffice, 'e-Office', JAMBI, NO_TICKETS),
    project(REPORT_PROJECT_IDS.network, 'Network & infra support', SETDA, NO_TICKETS),
  ]
}

function report(month: string, projects: SlaReportProject[], totals: SlaFigures, below: number): SlaReport {
  return {
    month, currentMonth: CURRENT_MONTH, oldestMonth: OLDEST_MONTH, firstMonth: '2026-08', toDate: month === CURRENT_MONTH,
    targetPercent: 90, waitingPause: totals.tickets > 0 ? 'all' : null, totals,
    projectsWithTickets: projects.filter((p) => p.figures.tickets > 0).length, projectsBelowTarget: below, projects,
  }
}

function fixtures() {
  const reports: Record<string, SlaReport> = {
    '2026-11': report(
      '2026-11',
      [
        project(REPORT_PROJECT_IDS.eOffice, 'e-Office', JAMBI, {
          tickets: 30, incidents: 21, requests: 9, breaches: 2, withinSlaPercent: 93.33, avgFirstResponseMs: 14 * 60_000,
        }),
        project(REPORT_PROJECT_IDS.website, 'Website', RIAU, {
          tickets: 7, incidents: 5, requests: 2, breaches: 2, withinSlaPercent: 71.43, avgFirstResponseMs: 83 * 60_000,
        }, { belowTarget: true }),
        // Hidden from the report form, listed because it has a ticket this month.
        project(REPORT_PROJECT_IDS.legacy, 'Legacy portal', RIAU, {
          tickets: 1, incidents: 0, requests: 1, breaches: 0, withinSlaPercent: 100, avgFirstResponseMs: null,
        }, { active: false }),
        project(REPORT_PROJECT_IDS.network, 'Network & infra support', SETDA, NO_TICKETS),
      ],
      { tickets: 38, incidents: 26, requests: 12, breaches: 4, withinSlaPercent: 89.47, avgFirstResponseMs: 30 * 60_000 },
      1,
    ),
    '2026-10': report(
      '2026-10',
      [
        project(REPORT_PROJECT_IDS.eOffice, 'e-Office', JAMBI, {
          tickets: 71, incidents: 45, requests: 26, breaches: 4, withinSlaPercent: 94.37, avgFirstResponseMs: 15 * 60_000,
        }),
        project(REPORT_PROJECT_IDS.website, 'Website', RIAU, NO_TICKETS),
        project(REPORT_PROJECT_IDS.network, 'Network & infra support', SETDA, NO_TICKETS),
      ],
      { tickets: 71, incidents: 45, requests: 26, breaches: 4, withinSlaPercent: 94.37, avgFirstResponseMs: 15 * 60_000 },
      0,
    ),
  }
  return { reports, requests: [] as string[], failDownloads: false, noProjects: false }
}

export let reportsApi = fixtures()

export function resetReports() {
  reportsApi = fixtures()
}

// A string body: jsdom's Blob can't be streamed into a fetch Response.
function file(name: string, type: string) {
  return new HttpResponse('file', {
    headers: { 'Content-Type': type, 'Content-Disposition': `attachment; filename=${name}` },
  })
}

export const reportsHandlers = [
  http.get(`${API}/reports/sla`, ({ request }) => {
    const month = new URL(request.url).searchParams.get('month') ?? CURRENT_MONTH
    if (month > CURRENT_MONTH) return problem(400, 'month_in_future')
    if (month < OLDEST_MONTH) return problem(400, 'month_out_of_range')
    const found = reportsApi.reports[month] ?? report(month, emptyMonth(), NO_TICKETS, 0)
    return HttpResponse.json(reportsApi.noProjects ? { ...found, projects: [] } : found)
  }),
  http.get(`${API}/reports/sla/excel`, ({ request }) => {
    const url = new URL(request.url)
    reportsApi.requests.push(url.pathname + url.search)
    if (reportsApi.failDownloads) return problem(500, 'internal')
    return file(`sla-report-${url.searchParams.get('month')}.xlsx`, 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet')
  }),
  http.get(`${API}/reports/sla/projects/:projectId/pdf`, ({ request, params }) => {
    const url = new URL(request.url)
    reportsApi.requests.push(url.pathname + url.search)
    if (reportsApi.failDownloads) return problem(500, 'internal')
    return file(`sla-report-${url.searchParams.get('month')}-${String(params.projectId)}.pdf`, 'application/pdf')
  }),
]
