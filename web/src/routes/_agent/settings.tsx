import { createFileRoute } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { ComingSoonPage } from '../../features/shell/pages'
import { requireRole } from '../../lib/guards'
import { can } from '../../lib/roles'

export const Route = createFileRoute('/_agent/settings')({
  beforeLoad: ({ context }) => requireRole(context.me, can.manageSettings),
  component: SettingsRoute,
})

function SettingsRoute() {
  const { t } = useTranslation()
  return <ComingSoonPage title={t('nav.settings')} />
}
