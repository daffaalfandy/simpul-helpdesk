import { queryOptions } from '@tanstack/react-query'

import { api, ApiError, type Problem } from './client'
import { queryKeys } from './queries'
import { unwrap } from './unwrap'

/** The SLA report for a month ("2026-11"), or for this month when left out. */
export const slaReportQueryOptions = (month: string | undefined) =>
  queryOptions({
    queryKey: queryKeys.slaReport(month),
    queryFn: () => unwrap(api.GET('/reports/sla', { params: { query: { month } } })),
  })

/** The month for every project as an Excel workbook. */
export function downloadSlaExcel(month: string): Promise<void> {
  return save(api.GET('/reports/sla/excel', { params: { query: { month } }, parseAs: 'blob' }))
}

/** One project's month as a PDF, to send to its client. */
export function downloadSlaPdf(projectId: string, month: string): Promise<void> {
  return save(
    api.GET('/reports/sla/projects/{projectId}/pdf', { params: { path: { projectId }, query: { month } }, parseAs: 'blob' }),
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
