import { createFileRoute } from '@tanstack/react-router'

import { organizationQueryOptions } from '../../../api/queries'
import { PortalHomePage, portalListOptions } from '../../../features/portal/PortalHomePage'
import { DEFAULT_PORTAL_FILTER, isPortalFilter, type PortalFilter } from '../../../features/tickets/status'

type PortalSearch = { filter?: PortalFilter }

export const Route = createFileRoute('/_portal/portal/')({
  // The default filter stays out of the URL.
  validateSearch: (search: Record<string, unknown>): PortalSearch =>
    isPortalFilter(search.filter) && search.filter !== DEFAULT_PORTAL_FILTER ? { filter: search.filter } : {},
  loaderDeps: ({ search }) => ({ filter: search.filter ?? DEFAULT_PORTAL_FILTER }),
  loader: ({ context: { queryClient }, deps }) =>
    Promise.all([
      queryClient.ensureInfiniteQueryData(portalListOptions(deps.filter)),
      queryClient.ensureQueryData(organizationQueryOptions),
    ]),
  component: PortalHomeRoute,
})

function PortalHomeRoute() {
  const { filter = DEFAULT_PORTAL_FILTER } = Route.useSearch()
  const navigate = Route.useNavigate()
  return (
    <PortalHomePage
      filter={filter}
      onFilterChange={(next) =>
        navigate({ search: next === DEFAULT_PORTAL_FILTER ? {} : { filter: next }, replace: true })
      }
    />
  )
}
