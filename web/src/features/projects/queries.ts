import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, type Project, type ProjectCreate, type ProjectSla, type ProjectUpdate } from '../../api/client'
import { queryKeys, useInvalidate } from '../../api/queries'
import { unwrap } from '../../api/unwrap'

export const projectsQueryOptions = queryOptions({
  queryKey: queryKeys.projects,
  queryFn: () => unwrap(api.GET('/projects')),
})

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
