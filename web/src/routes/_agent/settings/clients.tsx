import { createFileRoute } from '@tanstack/react-router'

import { ClientsPage } from '../../../features/settings/ClientsPage'
import { clientsQueryOptions, slaPlansQueryOptions, usersQueryOptions } from '../../../features/settings/queries'

export const Route = createFileRoute('/_agent/settings/clients')({
  loader: ({ context }) =>
    Promise.all([
      context.queryClient.ensureQueryData(clientsQueryOptions),
      context.queryClient.ensureQueryData(slaPlansQueryOptions),
      context.queryClient.ensureQueryData(usersQueryOptions),
    ]),
  component: ClientsPage,
})
