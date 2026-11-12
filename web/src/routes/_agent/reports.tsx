import { createFileRoute } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { ComingSoonPage } from '../../features/shell/pages'
import { requireRole } from '../../lib/guards'
import { can } from '../../lib/roles'

export const Route = createFileRoute('/_agent/reports')({
  beforeLoad: ({ context }) => requireRole(context.me, can.viewReports),
  component: ReportsRoute,
})

function ReportsRoute() {
  const { t } = useTranslation()
  return <ComingSoonPage title={t('nav.reports')} />
}
