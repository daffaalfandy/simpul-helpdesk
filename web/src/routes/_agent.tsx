import { createFileRoute } from '@tanstack/react-router'

import { AgentLayout } from '../features/shell/pages'
import { requireRole, requireUser } from '../lib/guards'
import { can } from '../lib/roles'

// Agent app: admins, supervisors and agents. Requesters are sent to the portal.
export const Route = createFileRoute('/_agent')({
  beforeLoad: async ({ context, location }) => {
    const me = await requireUser(context.queryClient, location)
    requireRole(me, can.useAgentApp)
    return { me }
  },
  component: AgentLayout,
})
