import { createFileRoute } from '@tanstack/react-router'

import { attachmentPolicyQueryOptions, organizationQueryOptions } from '../../../api/queries'
import { ReportIssuePage } from '../../../features/portal/ReportIssuePage'
import { intakeQueryOptions } from '../../../features/tickets/queries'

export const Route = createFileRoute('/_portal/portal/new')({
  loader: ({ context: { queryClient } }) =>
    Promise.all([
      queryClient.ensureQueryData(intakeQueryOptions(null)),
      queryClient.ensureQueryData(attachmentPolicyQueryOptions),
      queryClient.ensureQueryData(organizationQueryOptions),
    ]),
  component: ReportIssuePage,
})
