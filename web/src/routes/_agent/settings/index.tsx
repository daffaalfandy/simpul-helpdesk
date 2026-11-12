import { createFileRoute, redirect } from '@tanstack/react-router'

import { paths } from '../../../lib/constants'

export const Route = createFileRoute('/_agent/settings/')({
  beforeLoad: () => {
    throw redirect({ to: paths.settingsGeneral })
  },
})
