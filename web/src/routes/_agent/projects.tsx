import { createFileRoute } from '@tanstack/react-router'

import { ProjectsLayout } from '../../features/projects/ProjectsLayout'
import { projectsQueryOptions } from '../../features/projects/queries'
import { requireRole } from '../../lib/guards'
import { can } from '../../lib/roles'

export const Route = createFileRoute('/_agent/projects')({
  beforeLoad: ({ context }) => requireRole(context.me, can.viewProjects),
  loader: ({ context }) => context.queryClient.ensureQueryData(projectsQueryOptions),
  component: ProjectsLayout,
})
