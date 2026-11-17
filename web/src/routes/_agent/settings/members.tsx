import { createFileRoute } from '@tanstack/react-router'

import { organizationQueryOptions } from '../../../api/queries'
import { MembersPage } from '../../../features/settings/MembersPage'
import { membersQueryOptions } from '../../../features/settings/queries'

export const Route = createFileRoute('/_agent/settings/members')({
  loader: ({ context }) =>
    Promise.all([context.queryClient.ensureQueryData(membersQueryOptions), context.queryClient.ensureQueryData(organizationQueryOptions)]),
  component: MembersPage,
})
