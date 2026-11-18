import { http, HttpResponse } from 'msw'

import type { SlaFigures, SlaReport, SlaReportProject } from '../api/client'
import { problem } from './server'

const API = '*/api/v1'

export const REPORT_PROJECT_IDS = { eOffice: 'p-eoffice', website: 'p-website', legacy: 'p-legacy', network: 'p-network' } as const
export const CURRENT_MONTH = '2026-11'
/** The earliest month the API serves: 12 before the current one. */
export const OLDEST_MONTH = '2025-11'
const FIRST_MONTH = '2026-08'
const TARGET_PERCENT = 90
const MONTH = /^(\d{4})-(\d{2})$/
const MONTHS_PER_YEAR = 12
const PERCENT = 100
/** Shares keep two decimals, as the API sends them. */
const HUNDREDTHS = 100
const MINUTE_MS = 60_000

const JAMBI = { id: 'c-jambi', name: 'Dinas Kominfo Jambi' }
const RIAU = { id: 'c-riau', name: 'Bappeda Prov. Riau' }
const SETDA = { id: 'c-setda', name: 'Setda Kab. Muaro Jambi' }

const NO_TICKETS: SlaFigures = {
  tickets: 0, incidents: 0, requests: 0, breaches: 0, withinSlaPercent: null, avgFirstResponseMs: null,
}

/** A month's figures for a project: the share within SLA follows from tickets and breaches. */
function figures(tickets: number, incidents: number, breaches: number, avgFirstResponseMinutes: number | null): SlaFigures {
  return {
    tickets, incidents, requests: tickets - incidents, breaches, withinSlaPercent: within(tickets, breaches),
    avgFirstResponseMs: avgFirstResponseMinutes === null ? null : avgFirstResponseMinutes * MINUTE_MS,
  }
}

function within(tickets: number, breaches: number): number | null {
  return tickets === 0 ? null : Math.round(((tickets - breaches) / tickets) * PERCENT * HUNDREDTHS) / HUNDREDTHS
}

function project(
  id: string,
  name: string,
  client: { id: string; name: string },
  f: SlaFigures,
  { active = true } = {},
): SlaReportProject {
  return { id, name, client, active, figures: f, belowTarget: isBelowTarget(f) }
}

function isBelowTarget(f: SlaFigures): boolean {
  return f.withinSlaPercent !== null && f.withinSlaPercent < TARGET_PERCENT
}

/** Every active project, none with tickets: a month without fixtures. */
function emptyMonth(): SlaReportProject[] {
  return [
    project(REPORT_PROJECT_IDS.website, 'Website', RIAU, NO_TICKETS),
    project(REPORT_PROJECT_IDS.eOffice, 'e-Office', JAMBI, NO_TICKETS),
    project(REPORT_PROJECT_IDS.network, 'Network & infra support', SETDA, NO_TICKETS),
  ]
}

/** Each month's projects; a hidden project is listed only in months it has tickets, as the API does. */
function fixtures() {
  const months: Record<string, SlaReportProject[]> = {
    '2026-11': [
      project(REPORT_PROJECT_IDS.eOffice, 'e-Office', JAMBI, figures(30, 21, 2, 14)),
      project(REPORT_PROJECT_IDS.website, 'Website', RIAU, figures(7, 5, 2, 83)),
      // Hidden from the report form, listed because it has a ticket this month.
      project(REPORT_PROJECT_IDS.legacy, 'Legacy portal', RIAU, figures(1, 0, 0, null), { active: false }),
      project(REPORT_PROJECT_IDS.network, 'Network & infra support', SETDA, NO_TICKETS),
    ],
    '2026-10': [
      project(REPORT_PROJECT_IDS.eOffice, 'e-Office', JAMBI, figures(71, 45, 4, 15)),
      project(REPORT_PROJECT_IDS.website, 'Website', RIAU, NO_TICKETS),
      project(REPORT_PROJECT_IDS.network, 'Network & infra support', SETDA, NO_TICKETS),
    ],
  }
  return { months, requests: [] as string[], failDownloads: false, noProjects: false }
}

export let reportsApi = fixtures()

export function resetReports() {
  reportsApi = fixtures()
}

function nextMonth(m: string): string {
  const [, year, month] = MONTH.exec(m) ?? []
  const y = Number(year)
  const n = Number(month)
  return n === MONTHS_PER_YEAR ? `${y + 1}-01` : `${y}-${String(n + 1).padStart(2, '0')}`
}

/** Adds b to a; the average first response is weighed by tickets. */
function add(a: SlaFigures, b: SlaFigures): SlaFigures {
  const tickets = a.tickets + b.tickets
  const breaches = a.breaches + b.breaches
  const timed = [a, b].filter((f) => f.avgFirstResponseMs !== null && f.tickets > 0)
  const weight = timed.reduce((n, f) => n + f.tickets, 0)
  return {
    tickets, breaches, incidents: a.incidents + b.incidents, requests: a.requests + b.requests,
    withinSlaPercent: within(tickets, breaches),
    avgFirstResponseMs: weight === 0 ? null : Math.round(timed.reduce((n, f) => n + (f.avgFirstResponseMs ?? 0) * f.tickets, 0) / weight),
  }
}

const byName = (a: string, b: string) => a.localeCompare(b, undefined, { sensitivity: 'base' })

/** The report over from..to: each project's months added up, busiest first, then by client and project. */
function report(from: string, to: string): SlaReport {
  const merged = new Map<string, SlaReportProject>()
  for (let m = from; m <= to; m = nextMonth(m)) {
    for (const p of reportsApi.months[m] ?? emptyMonth()) {
      const seen = merged.get(p.id)
      merged.set(p.id, seen ? project(p.id, p.name, p.client, add(seen.figures, p.figures), { active: p.active }) : p)
    }
  }
  const projects = [...merged.values()].sort(
    (a, b) => b.figures.tickets - a.figures.tickets || byName(a.client.name, b.client.name) || byName(a.name, b.name),
  )
  const totals = projects.reduce((sum, p) => add(sum, p.figures), NO_TICKETS)
  return {
    from, to, currentMonth: CURRENT_MONTH, oldestMonth: OLDEST_MONTH, firstMonth: FIRST_MONTH, toDate: to === CURRENT_MONTH,
    targetPercent: TARGET_PERCENT, waitingPause: totals.tickets > 0 ? 'all' : null, totals,
    projectsWithTickets: projects.filter((p) => p.figures.tickets > 0).length,
    projectsBelowTarget: projects.filter((p) => p.belowTarget).length,
    projects: reportsApi.noProjects ? [] : projects,
  }
}

/** The months asked for, each defaulting to the current one, or the API's refusal. */
function months(url: URL): { from: string; to: string } | { refused: Response } {
  const from = url.searchParams.get('from') ?? CURRENT_MONTH
  const to = url.searchParams.get('to') ?? CURRENT_MONTH
  if (from > CURRENT_MONTH || to > CURRENT_MONTH) return { refused: problem(400, 'month_in_future') }
  if (from < OLDEST_MONTH || to < OLDEST_MONTH) return { refused: problem(400, 'month_out_of_range') }
  if (to < from) return { refused: problem(400, 'invalid_month_range') }
  return { from, to }
}

/** "2026-11" for one month, "2026-09-to-2026-11" for several, as the API names its files. */
function fileMonths(from: string, to: string): string {
  return from === to ? from : `${from}-to-${to}`
}

// A string body: jsdom's Blob can't be streamed into a fetch Response.
function file(name: string, type: string) {
  return new HttpResponse('file', {
    headers: { 'Content-Type': type, 'Content-Disposition': `attachment; filename=${name}` },
  })
}

export const reportsHandlers = [
  http.get(`${API}/reports/sla`, ({ request }) => {
    const asked = months(new URL(request.url))
    if ('refused' in asked) return asked.refused
    return HttpResponse.json(report(asked.from, asked.to))
  }),
  http.get(`${API}/reports/sla/excel`, ({ request }) => {
    const url = new URL(request.url)
    reportsApi.requests.push(url.pathname + url.search)
    const asked = months(url)
    if ('refused' in asked) return asked.refused
    if (reportsApi.failDownloads) return problem(500, 'internal')
    return file(`sla-report-${fileMonths(asked.from, asked.to)}.xlsx`, 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet')
  }),
  http.get(`${API}/reports/sla/projects/:projectId/pdf`, ({ request, params }) => {
    const url = new URL(request.url)
    reportsApi.requests.push(url.pathname + url.search)
    const asked = months(url)
    if ('refused' in asked) return asked.refused
    if (reportsApi.failDownloads) return problem(500, 'internal')
    return file(`sla-report-${fileMonths(asked.from, asked.to)}-${String(params.projectId)}.pdf`, 'application/pdf')
  }),
]
