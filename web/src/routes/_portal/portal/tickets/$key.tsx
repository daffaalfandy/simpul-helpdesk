import { createFileRoute, notFound } from '@tanstack/react-router'

import { attachmentPolicyQueryOptions, loadOrNotFound, organizationQueryOptions } from '../../../../api/queries'
import { PortalTicketPage } from '../../../../features/portal/PortalTicketPage'
import { businessHoursQueryOptions } from '../../../../features/settings/queries'
import { ticketMessagesQueryOptions, ticketQueryOptions } from '../../../../features/tickets/queries'
import { TicketNotFound } from '../../../../features/tickets/TicketNotFound'
import { normalizeTicketKey } from '../../../../lib/ticketKey'

export const Route = createFileRoute('/_portal/portal/tickets/$key')({
  loader: async ({ context: { queryClient }, params }) => {
    const key = normalizeTicketKey(params.key)
    if (!key) throw notFound()
    await loadOrNotFound(() => queryClient.ensureQueryData(ticketQueryOptions(key)))
    await Promise.all([
      queryClient.ensureQueryData(ticketMessagesQueryOptions(key)),
      queryClient.ensureQueryData(businessHoursQueryOptions),
      queryClient.ensureQueryData(organizationQueryOptions),
      queryClient.ensureQueryData(attachmentPolicyQueryOptions),
    ])
    return { key }
  },
  component: PortalTicketRoute,
  notFoundComponent: () => <TicketNotFound audience="requester" />,
})

function PortalTicketRoute() {
  const { key } = Route.useLoaderData()
  return <PortalTicketPage ticketKey={key} />
}
