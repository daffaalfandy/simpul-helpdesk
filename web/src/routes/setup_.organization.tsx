import { createFileRoute, redirect } from '@tanstack/react-router'

import { OrganizationSetupPage } from '../features/auth/OrganizationSetupPage'
import { requireUser } from '../lib/guards'
import { homePathFor } from '../lib/roles'

// First-run setup, step 2: the administrator creates the first organization. Once one exists
// (or for anyone else) this page has nothing to do.
export const Route = createFileRoute('/setup_/organization')({
  beforeLoad: async ({ context, location }) => {
    const me = await requireUser(context.queryClient, location)
    if (!me.isAdmin || me.organizations.length > 0) throw redirect({ to: homePathFor(me.role) })
  },
  component: OrganizationSetupPage,
})
