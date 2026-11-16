import { createFileRoute } from '@tanstack/react-router'

import { projectsQueryOptions } from '../../../features/projects/queries'
import { IntegrationsPage } from '../../../features/settings/integrations/IntegrationsPage'
import { integrationsQueryOptions } from '../../../features/settings/integrations/queries'

export const Route = createFileRoute('/_agent/settings/integrations')({
  loader: ({ context }) =>
    Promise.all([
      context.queryClient.ensureQueryData(integrationsQueryOptions),
      context.queryClient.ensureQueryData(projectsQueryOptions),
    ]),
  component: IntegrationsPage,
})
