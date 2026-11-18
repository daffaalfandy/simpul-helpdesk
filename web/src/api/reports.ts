import { queryOptions } from '@tanstack/react-query'

import { api, ApiError, type Problem } from './client'
import { queryKeys } from './queries'
import { unwrap } from './unwrap'

/** The report's months ("2026-09" to "2026-11"); a missing end is the current month. */
export type SlaReportMonths = { from?: string; to?: string }

/** The SLA report over a range of months, or for this month when both are left out. */
export const slaReportQueryOptions = ({ from, to }: SlaReportMonths) =>
  queryOptions({
    queryKey: queryKeys.slaReport(from, to),
    queryFn: () => unwrap(api.GET('/reports/sla', { params: { query: { from, to } } })),
  })

/** The months for every project as an Excel workbook. */
export function downloadSlaExcel({ from, to }: SlaReportMonths): Promise<void> {
  return save(api.GET('/reports/sla/excel', { params: { query: { from, to } }, parseAs: 'blob' }))
}

/** One project's months as a PDF, to send to its client. */
export function downloadSlaPdf(projectId: string, { from, to }: SlaReportMonths): Promise<void> {
  return save(
    api.GET('/reports/sla/projects/{projectId}/pdf', { params: { path: { projectId }, query: { from, to } }, parseAs: 'blob' }),
  )
}

// The object URL lives a moment past the click, so the browser has started the download.
const REVOKE_DELAY_MS = 1_000
const DISPOSITION_HEADER = 'Content-Disposition'
const FILENAME_PATTERN = /filename="?([^";]+)"?/

/** Saves a file response under the name the API gives it; errors arrive as ApiError. */
async function save(call: Promise<{ data?: Blob; error?: unknown; response: Response }>): Promise<void> {
  const { data, error, response } = await call
  if (error !== undefined || !response.ok || !data) throw new ApiError(response, error as Partial<Problem> | undefined)
  const url = URL.createObjectURL(data)
  const link = document.createElement('a')
  link.href = url
  link.download = FILENAME_PATTERN.exec(response.headers.get(DISPOSITION_HEADER) ?? '')?.[1] ?? ''
  link.click()
  setTimeout(() => URL.revokeObjectURL(url), REVOKE_DELAY_MS)
}
