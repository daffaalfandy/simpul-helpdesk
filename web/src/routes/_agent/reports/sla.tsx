import { createFileRoute, redirect } from '@tanstack/react-router'

import { ApiError, ErrorCode } from '../../../api/client'
import { slaReportQueryOptions } from '../../../api/reports'
import { validateReportSearch } from '../../../features/reports/months'
import { SlaReportPage } from '../../../features/reports/SlaReportPage'
import { paths } from '../../../lib/constants'

// A month the report doesn't serve (an old link or a hand-typed URL) opens the current month.
const UNSERVED_MONTH: ReadonlySet<string> = new Set([ErrorCode.MonthInFuture, ErrorCode.MonthOutOfRange])

export const Route = createFileRoute('/_agent/reports/sla')({
  validateSearch: validateReportSearch,
  loaderDeps: ({ search }) => search,
  loader: async ({ context: { queryClient }, deps }) => {
    try {
      return await queryClient.ensureQueryData(slaReportQueryOptions(deps.month))
    } catch (error) {
      if (deps.month && error instanceof ApiError && UNSERVED_MONTH.has(error.code)) {
        throw redirect({ to: paths.reportsSla, search: {}, replace: true })
      }
      throw error
    }
  },
  component: SlaReportRoute,
})

function SlaReportRoute() {
  const { month } = Route.useSearch()
  const navigate = Route.useNavigate()
  return (
    <SlaReportPage
      month={month}
      onMonthChange={(m, current) => navigate({ search: m === current ? {} : { month: m } })}
    />
  )
}
