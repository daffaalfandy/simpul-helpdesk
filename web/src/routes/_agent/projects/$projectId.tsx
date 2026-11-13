import { createFileRoute } from '@tanstack/react-router'

import { loadOrNotFound } from '../../../api/queries'
import { ProjectPage } from '../../../features/projects/ProjectPage'
import { projectQueryOptions } from '../../../features/projects/queries'
import { businessHoursQueryOptions } from '../../../features/settings/queries'

export const Route = createFileRoute('/_agent/projects/$projectId')({
  loader: ({ context: { queryClient }, params }) =>
    Promise.all([
      loadOrNotFound(() => queryClient.ensureQueryData(projectQueryOptions(params.projectId))),
      queryClient.ensureQueryData(businessHoursQueryOptions),
    ]),
  component: ProjectRoute,
})

function ProjectRoute() {
  const { projectId } = Route.useParams()
  return <ProjectPage key={projectId} projectId={projectId} />
}
