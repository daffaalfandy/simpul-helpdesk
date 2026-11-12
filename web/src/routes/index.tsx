import { createFileRoute, redirect } from '@tanstack/react-router'

import { requireUser } from '../lib/guards'
import { homePathFor } from '../lib/roles'

// "/" only routes people to their start page.
export const Route = createFileRoute('/')({
  beforeLoad: async ({ context, location }) => {
    const me = await requireUser(context.queryClient, location)
    throw redirect({ to: homePathFor(me.role) })
  },
})
