import { http, HttpResponse } from 'msw'

import type { SlaFigures, SlaReport, SlaReportClient } from '../api/client'
import { problem } from './server'

const API = '*/api/v1'

export const REPORT_CLIENT_IDS = { jambi: 'c-jambi', riau: 'c-riau', setda: 'c-setda' } as const
export const CURRENT_MONTH = '2026-11'

const NO_TICKETS: SlaFigures = {
  tickets: 0, incidents: 0, requests: 0, breaches: 0, withinSlaPercent: null, avgFirstResponseMs: null,
}

function client(id: string, name: string, figures: SlaFigures, belowTarget = false): SlaReportClient {
  return { id, name, figures, belowTarget }
}

/** Every client, none with tickets: a month before the fixtures. */
function emptyMonth(): SlaReportClient[] {
  return [
    client(REPORT_CLIENT_IDS.jambi, 'Dinas Kominfo Jambi', NO_TICKETS),
    client(REPORT_CLIENT_IDS.riau, 'Bappeda Prov. Riau', NO_TICKETS),
    client(REPORT_CLIENT_IDS.setda, 'Setda Kab. Muaro Jambi', NO_TICKETS),
  ]
}

function report(month: string, clients: SlaReportClient[], totals: SlaFigures, below: number): SlaReport {
  return {
    month, currentMonth: CURRENT_MONTH, firstMonth: '2026-08', toDate: month === CURRENT_MONTH, targetPercent: 90,
    waitingPause: totals.tickets > 0 ? 'all' : null, totals,
    clientsWithTickets: clients.filter((c) => c.figures.tickets > 0).length, clientsBelowTarget: below, clients,
  }
}

function fixtures() {
  const reports: Record<string, SlaReport> = {
    '2026-11': report(
      '2026-11',
      [
        client(REPORT_CLIENT_IDS.jambi, 'Dinas Kominfo Jambi', {
          tickets: 30, incidents: 21, requests: 9, breaches: 2, withinSlaPercent: 93.33, avgFirstResponseMs: 14 * 60_000,
        }),
        client(REPORT_CLIENT_IDS.riau, 'Bappeda Prov. Riau', {
          tickets: 8, incidents: 5, requests: 3, breaches: 2, withinSlaPercent: 75, avgFirstResponseMs: 83 * 60_000,
        }, true),
        client(REPORT_CLIENT_IDS.setda, 'Setda Kab. Muaro Jambi', NO_TICKETS),
      ],
      { tickets: 38, incidents: 26, requests: 12, breaches: 4, withinSlaPercent: 89.47, avgFirstResponseMs: 30 * 60_000 },
      1,
    ),
    '2026-10': report(
      '2026-10',
      [
        client(REPORT_CLIENT_IDS.jambi, 'Dinas Kominfo Jambi', {
          tickets: 71, incidents: 45, requests: 26, breaches: 4, withinSlaPercent: 94.37, avgFirstResponseMs: 15 * 60_000,
        }),
        client(REPORT_CLIENT_IDS.riau, 'Bappeda Prov. Riau', NO_TICKETS),
        client(REPORT_CLIENT_IDS.setda, 'Setda Kab. Muaro Jambi', NO_TICKETS),
      ],
      { tickets: 71, incidents: 45, requests: 26, breaches: 4, withinSlaPercent: 94.37, avgFirstResponseMs: 15 * 60_000 },
      0,
    ),
  }
  return { reports, requests: [] as string[], failDownloads: false, noClients: false }
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
    const found = reportsApi.reports[month] ?? report(month, emptyMonth(), NO_TICKETS, 0)
    return HttpResponse.json(reportsApi.noClients ? { ...found, clients: [] } : found)
  }),
  http.get(`${API}/reports/sla/excel`, ({ request }) => {
    const url = new URL(request.url)
    reportsApi.requests.push(url.pathname + url.search)
    if (reportsApi.failDownloads) return problem(500, 'internal')
    return file(`sla-report-${url.searchParams.get('month')}.xlsx`, 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet')
  }),
  http.get(`${API}/reports/sla/clients/:clientId/pdf`, ({ request, params }) => {
    const url = new URL(request.url)
    reportsApi.requests.push(url.pathname + url.search)
    if (reportsApi.failDownloads) return problem(500, 'internal')
    return file(`sla-report-${url.searchParams.get('month')}-${String(params.clientId)}.pdf`, 'application/pdf')
  }),
]
