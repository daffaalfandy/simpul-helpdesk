import { createFileRoute } from '@tanstack/react-router'

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
  component: PortalLayout,
})
