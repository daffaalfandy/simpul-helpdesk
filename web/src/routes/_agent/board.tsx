import { createFileRoute } from '@tanstack/react-router'

import { attachmentPolicyQueryOptions, clientsQueryOptions, organizationQueryOptions } from '../../api/queries'
import { BoardPage } from '../../features/board/BoardPage'
import { boardQueryOptions, validateBoardSearch } from '../../features/board/boardSearch'
import { projectFilterQueryOptions, projectsQueryOptions } from '../../features/projects/queries'
import { rangeFromSearch } from '../../lib/dateRange'

export const Route = createFileRoute('/_agent/board')({
  validateSearch: validateBoardSearch,
  loaderDeps: ({ search }) => search,
  loader: async ({ context: { queryClient }, deps }) => {
    // The default range is this month in the organization's time zone.
    const organization = await queryClient.ensureQueryData(organizationQueryOptions)
    return Promise.all([
      queryClient.ensureQueryData(boardQueryOptions({ ...deps, ...rangeFromSearch(deps, organization.timezone) })),
      queryClient.ensureQueryData(clientsQueryOptions),
      queryClient.ensureQueryData(attachmentPolicyQueryOptions),
      queryClient.ensureQueryData(projectsQueryOptions),
      queryClient.ensureQueryData(projectFilterQueryOptions),
    ])
  },
  component: BoardRoute,
})

function BoardRoute() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  return <BoardPage search={search} onSearchChange={(next) => navigate({ search: next, replace: true })} />
}
