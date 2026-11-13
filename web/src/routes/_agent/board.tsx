import { createFileRoute } from '@tanstack/react-router'

import { attachmentPolicyQueryOptions, clientsQueryOptions, organizationQueryOptions } from '../../api/queries'
import { BoardPage } from '../../features/board/BoardPage'
import { boardListOptions, validateBoardSearch } from '../../features/board/boardSearch'

export const Route = createFileRoute('/_agent/board')({
  validateSearch: validateBoardSearch,
  loaderDeps: ({ search }) => search,
  loader: ({ context: { queryClient }, deps }) =>
    Promise.all([
      queryClient.ensureInfiniteQueryData(boardListOptions(deps)),
      queryClient.ensureQueryData(clientsQueryOptions),
      queryClient.ensureQueryData(organizationQueryOptions),
      queryClient.ensureQueryData(attachmentPolicyQueryOptions),
    ]),
  component: BoardRoute,
})

function BoardRoute() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  return <BoardPage search={search} onSearchChange={(next) => navigate({ search: next, replace: true })} />
}
