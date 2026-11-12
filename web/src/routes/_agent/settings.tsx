import { createFileRoute } from '@tanstack/react-router'

import { SettingsLayout } from '../../features/settings/SettingsLayout'
import { requireRole } from '../../lib/guards'
import { can } from '../../lib/roles'

export const Route = createFileRoute('/_agent/settings')({
  beforeLoad: ({ context }) => requireRole(context.me, can.manageSettings),
  component: SettingsLayout,
})
