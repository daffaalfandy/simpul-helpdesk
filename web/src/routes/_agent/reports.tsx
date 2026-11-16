import { createFileRoute } from '@tanstack/react-router'

import { requireRole } from '../../lib/guards'
import { can } from '../../lib/roles'

export const Route = createFileRoute('/_agent/reports')({
  beforeLoad: ({ context }) => requireRole(context.me, can.viewReports),
})
