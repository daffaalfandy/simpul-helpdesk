import { createFileRoute } from '@tanstack/react-router'

import { businessHoursQueryOptions, slaPlansQueryOptions } from '../../../features/settings/queries'
import { SlaPage } from '../../../features/settings/SlaPage'

export const Route = createFileRoute('/_agent/settings/sla')({
  loader: ({ context }) =>
    Promise.all([
      context.queryClient.ensureQueryData(slaPlansQueryOptions),
      context.queryClient.ensureQueryData(businessHoursQueryOptions),
    ]),
  component: SlaPage,
})
