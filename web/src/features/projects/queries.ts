import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, type Project, type ProjectCreate, type ProjectDodInput, type ProjectSla, type ProjectUpdate } from '../../api/client'
import { queryKeys, useInvalidate } from '../../api/queries'
import { unwrap } from '../../api/unwrap'

export const projectsQueryOptions = queryOptions({
  queryKey: queryKeys.projects,
  queryFn: () => unwrap(api.GET('/projects')),
})

/** The projects this user sees on the board and in ticket lists; none saved means all. */
export const projectFilterQueryOptions = queryOptions({
  queryKey: queryKeys.projectFilter,
  queryFn: () => unwrap(api.GET('/me/project-filter')),
})

/** Saves the project filter at once (the checkbox shows it before the API answers). */
export function useSetProjectFilter() {
  const queryClient = useQueryClient()
  const key = projectFilterQueryOptions.queryKey
  return useMutation({
    mutationFn: (projectIds: string[]) => unwrap(api.PUT('/me/project-filter', { body: { projectIds } })),
    onMutate: async (projectIds) => {
      await queryClient.cancelQueries({ queryKey: key })
      const previous = queryClient.getQueryData(key)
      queryClient.setQueryData(key, { projectIds })
      return { previous }
    },
    onError: (_error, _ids, context) => {
      if (context?.previous) queryClient.setQueryData(key, context.previous)
    },
    onSuccess: (saved) => queryClient.setQueryData(key, saved),
    onSettled: () => queryClient.invalidateQueries({ queryKey: queryKeys.tickets }),
  })
}

export function projectQueryOptions(id: string) {
  return queryOptions({
    queryKey: queryKeys.project(id),
    queryFn: () => unwrap(api.GET('/projects/{projectId}', { params: { path: { projectId: id } } })),
  })
}

/** Projects change what the report form offers and the clients list shows. */
function useAfterProjectChange() {
  const queryClient = useQueryClient()
  const invalidate = useInvalidate()
  return (project?: Project) => {
    if (project) queryClient.setQueryData(queryKeys.project(project.id), project)
    return invalidate(queryKeys.projects, queryKeys.intake, queryKeys.clients)
  }
}

export function useCreateProject() {
  const after = useAfterProjectChange()
  return useMutation({
    mutationFn: (input: ProjectCreate) => unwrap(api.POST('/projects', { body: input })),
    onSuccess: (project) => after(project),
  })
}

export function useUpdateProject(id: string) {
  const after = useAfterProjectChange()
  return useMutation({
    mutationFn: (input: ProjectUpdate) =>
      unwrap(api.PUT('/projects/{projectId}', { params: { path: { projectId: id } }, body: input })),
    onSuccess: (project) => after(project),
  })
}

export function useDeleteProject(id: string) {
  const queryClient = useQueryClient()
  const after = useAfterProjectChange()
  return useMutation({
    mutationFn: () => unwrap(api.DELETE('/projects/{projectId}', { params: { path: { projectId: id } } })),
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: queryKeys.project(id) })
      return after()
    },
  })
}

export function useSaveProjectSla(id: string) {
  const after = useAfterProjectChange()
  return useMutation({
    mutationFn: (sla: ProjectSla) =>
      unwrap(api.PUT('/projects/{projectId}/sla', { params: { path: { projectId: id } }, body: sla })),
    onSuccess: (project) => after(project),
  })
}

export function useSaveProjectDod(id: string) {
  const after = useAfterProjectChange()
  return useMutation({
    mutationFn: (input: ProjectDodInput) =>
      unwrap(api.PUT('/projects/{projectId}/dod', { params: { path: { projectId: id } }, body: input })),
    onSuccess: (project) => after(project),
  })
}
