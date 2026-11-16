import { queryOptions, useMutation } from '@tanstack/react-query'

import type { RepositoryCreate } from '../../../api/client'
import { api } from '../../../api/client'
import { queryKeys, useInvalidate } from '../../../api/queries'
import { unwrap } from '../../../api/unwrap'

export const integrationsQueryOptions = queryOptions({
  queryKey: queryKeys.integrations,
  queryFn: () => unwrap(api.GET('/integrations')),
})

export const codeEventsQueryOptions = queryOptions({
  queryKey: queryKeys.codeEvents,
  queryFn: () => unwrap(api.GET('/integrations/events')),
})

/** What a commit message would do; the same message is read once. */
export function commitTestQueryOptions(message: string) {
  return queryOptions({
    queryKey: queryKeys.commitTest(message),
    queryFn: () => unwrap(api.POST('/integrations/commit-test', { body: { message } })),
    staleTime: Number.POSITIVE_INFINITY,
  })
}

/** A repository change moves projects between repositories, so projects reload too. */
function useInvalidateRepositories() {
  const invalidate = useInvalidate()
  return () => invalidate(queryKeys.integrations, queryKeys.projects)
}

export function useCreateRepository() {
  const invalidate = useInvalidateRepositories()
  return useMutation({
    mutationFn: (body: RepositoryCreate) => unwrap(api.POST('/repositories', { body })),
    onSuccess: invalidate,
  })
}

export function useUpdateRepository() {
  const invalidate = useInvalidateRepositories()
  return useMutation({
    mutationFn: ({ id, projectIds }: { id: string; projectIds: string[] }) =>
      unwrap(api.PATCH('/repositories/{repositoryId}', { params: { path: { repositoryId: id } }, body: { projectIds } })),
    onSuccess: invalidate,
  })
}

export function useRotateSecret() {
  const invalidate = useInvalidateRepositories()
  return useMutation({
    mutationFn: (id: string) =>
      unwrap(api.POST('/repositories/{repositoryId}/secret', { params: { path: { repositoryId: id } } })),
    onSuccess: invalidate,
  })
}

export function useDeleteRepository() {
  const invalidate = useInvalidateRepositories()
  return useMutation({
    mutationFn: (id: string) =>
      unwrap(api.DELETE('/repositories/{repositoryId}', { params: { path: { repositoryId: id } } })),
    onSuccess: invalidate,
  })
}
