import { createFileRoute } from '@tanstack/react-router'

import { slaReportQueryOptions } from '../../../api/reports'
import { validateReportSearch } from '../../../features/reports/months'
import { SlaReportPage } from '../../../features/reports/SlaReportPage'

export const Route = createFileRoute('/_agent/reports/sla')({
  validateSearch: validateReportSearch,
  loaderDeps: ({ search }) => search,
  loader: ({ context: { queryClient }, deps }) => queryClient.ensureQueryData(slaReportQueryOptions(deps.month)),
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
