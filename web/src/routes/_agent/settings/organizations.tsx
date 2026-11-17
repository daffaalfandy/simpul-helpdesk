import { createFileRoute } from '@tanstack/react-router'

import { OrganizationsPage } from '../../../features/settings/OrganizationsPage'
import { organizationsQueryOptions } from '../../../features/settings/queries'

export const Route = createFileRoute('/_agent/settings/organizations')({
  loader: ({ context }) => context.queryClient.ensureQueryData(organizationsQueryOptions),
  component: OrganizationsPage,
})
