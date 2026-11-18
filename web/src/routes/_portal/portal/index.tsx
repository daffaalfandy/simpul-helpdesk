import { createFileRoute } from '@tanstack/react-router'

import { organizationQueryOptions, useOrgTimeZone } from '../../../api/queries'
import { PortalHomePage, portalListOptions } from '../../../features/portal/PortalHomePage'
import { uatRequestsQueryOptions } from '../../../features/portal/queries'
import { DEFAULT_PORTAL_FILTER, isPortalFilter, type PortalFilter } from '../../../features/tickets/status'
import { rangeFromSearch, validateDateRangeSearch, withRange, type DateRangeSearch } from '../../../lib/dateRange'

type PortalSearch = { filter?: PortalFilter } & DateRangeSearch

export const Route = createFileRoute('/_portal/portal/')({
  // The defaults (open tickets, this month) stay out of the URL.
  validateSearch: (search: Record<string, unknown>): PortalSearch => ({
    ...(isPortalFilter(search.filter) && search.filter !== DEFAULT_PORTAL_FILTER ? { filter: search.filter } : {}),
    ...validateDateRangeSearch(search),
  }),
  loaderDeps: ({ search }) => ({ filter: search.filter ?? DEFAULT_PORTAL_FILTER, from: search.from, to: search.to }),
  loader: async ({ context: { queryClient }, deps }) => {
    // The default range is this month in the organization's time zone.
    const organization = await queryClient.ensureQueryData(organizationQueryOptions)
    return Promise.all([
      queryClient.ensureInfiniteQueryData(portalListOptions(deps.filter, rangeFromSearch(deps, organization.timezone))),
      queryClient.ensureQueryData(uatRequestsQueryOptions),
    ])
  },
  component: PortalHomeRoute,
})

function PortalHomeRoute() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  const timeZone = useOrgTimeZone()
  const { filter = DEFAULT_PORTAL_FILTER, ...rest } = search
  return (
    <PortalHomePage
      filter={filter}
      range={rangeFromSearch(search, timeZone)}
      onFilterChange={(next) => navigate({ search: next === DEFAULT_PORTAL_FILTER ? rest : { ...rest, filter: next }, replace: true })}
      onRangeChange={(next) => navigate({ search: withRange(search, next, timeZone), replace: true })}
    />
  )
}
