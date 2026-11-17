import { createFileRoute } from '@tanstack/react-router'

import { organizationQueryOptions } from '../../../api/queries'
import { GeneralPage } from '../../../features/settings/GeneralPage'
import { businessHoursQueryOptions, holidayPresetsQueryOptions } from '../../../features/settings/queries'

export const Route = createFileRoute('/_agent/settings/general')({
  loader: ({ context }) =>
    Promise.all([
      context.queryClient.ensureQueryData(organizationQueryOptions),
      context.queryClient.ensureQueryData(businessHoursQueryOptions),
      context.queryClient.ensureQueryData(holidayPresetsQueryOptions),
    ]),
  component: GeneralPage,
})
