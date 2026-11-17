import { createFileRoute, redirect } from '@tanstack/react-router'

import { NoOrganizationPage } from '../features/auth/NoOrganizationPage'
import { requireUser } from '../lib/guards'
import { homePathFor } from '../lib/roles'

// Staff who belong to no organization wait here until an administrator adds them to one.
export const Route = createFileRoute('/no-organization')({
  beforeLoad: async ({ context, location }) => {
    const me = await requireUser(context.queryClient, location)
    if (me.organization || me.isAdmin) throw redirect({ to: homePathFor(me.role) })
  },
  component: NoOrganizationPage,
})
