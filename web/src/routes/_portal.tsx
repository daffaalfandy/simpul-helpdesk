import { createFileRoute } from '@tanstack/react-router'

import { priorityLevelsQueryOptions } from '../api/queries'
import { PortalLayout } from '../features/shell/pages'
import { requireRole, requireUser } from '../lib/guards'
import { can } from '../lib/roles'

// Client portal: requesters only. Vendor staff are sent to the board.
export const Route = createFileRoute('/_portal')({
  beforeLoad: async ({ context, location }) => {
    const me = await requireUser(context.queryClient, location)
    requireRole(me, can.usePortal)
    return { me }
  },
  // Priority names label tickets and the report form; load them once per session.
  loader: ({ context }) => context.queryClient.ensureQueryData(priorityLevelsQueryOptions),
  component: PortalLayout,
})
