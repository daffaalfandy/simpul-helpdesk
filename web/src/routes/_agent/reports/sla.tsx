import { createFileRoute, redirect } from '@tanstack/react-router'

import { ApiError, ErrorCode } from '../../../api/client'
import { slaReportQueryOptions } from '../../../api/reports'
import { searchFromRange, validateReportSearch } from '../../../features/reports/months'
import { SlaReportPage } from '../../../features/reports/SlaReportPage'
import { paths } from '../../../lib/constants'

// Months the report doesn't serve (an old link or a hand-typed URL) open the current month.
const UNSERVED_RANGE: ReadonlySet<string> = new Set([
  ErrorCode.MonthInFuture,
  ErrorCode.MonthOutOfRange,
  ErrorCode.InvalidMonthRange,
])

export const Route = createFileRoute('/_agent/reports/sla')({
  validateSearch: validateReportSearch,
  loaderDeps: ({ search }) => search,
  loader: async ({ context: { queryClient }, deps }) => {
    try {
      return await queryClient.ensureQueryData(slaReportQueryOptions(deps))
    } catch (error) {
      if ((deps.from || deps.to) && error instanceof ApiError && UNSERVED_RANGE.has(error.code)) {
        throw redirect({ to: paths.reportsSla, search: {}, replace: true })
      }
      throw error
    }
  },
  component: SlaReportRoute,
})

function SlaReportRoute() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  return (
    <SlaReportPage search={search} onRangeChange={(range, current) => navigate({ search: searchFromRange(range, current) })} />
  )
}
