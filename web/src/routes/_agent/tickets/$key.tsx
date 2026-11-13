import { createFileRoute, notFound } from '@tanstack/react-router'

import { attachmentPolicyQueryOptions, loadOrNotFound, organizationQueryOptions } from '../../../api/queries'
import { businessHoursQueryOptions } from '../../../features/settings/queries'
import { AgentTicketPage } from '../../../features/tickets/AgentTicketPage'
import { ticketMessagesQueryOptions, ticketQueryOptions } from '../../../features/tickets/queries'
import { TicketNotFound } from '../../../features/tickets/TicketNotFound'
import { normalizeTicketKey } from '../../../lib/ticketKey'

export const Route = createFileRoute('/_agent/tickets/$key')({
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
  component: AgentTicketRoute,
  notFoundComponent: () => <TicketNotFound audience="agent" />,
})

function AgentTicketRoute() {
  const { key } = Route.useLoaderData()
  return <AgentTicketPage ticketKey={key} />
}
