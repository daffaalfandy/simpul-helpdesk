import { createFileRoute, redirect } from '@tanstack/react-router'

import { paths } from '../../../lib/constants'

export const Route = createFileRoute('/_agent/reports/')({
  beforeLoad: () => {
    throw redirect({ to: paths.reportsSla })
  },
})
