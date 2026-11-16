import { createFileRoute, notFound } from '@tanstack/react-router'
import { useCallback } from 'react'

import type { UatAnswer } from '../../../../api/client'

import { attachmentPolicyQueryOptions, loadOrNotFound, organizationQueryOptions } from '../../../../api/queries'
import { PortalTicketPage } from '../../../../features/portal/PortalTicketPage'
import { ticketMessagesQueryOptions, ticketQueryOptions } from '../../../../features/tickets/queries'
import { TicketNotFound } from '../../../../features/tickets/TicketNotFound'
import { normalizeTicketKey } from '../../../../lib/ticketKey'

/** The test email's answer buttons (?answer=approve|reject). */
const UAT_ANSWERS: readonly UatAnswer[] = ['approve', 'reject']
type PortalTicketSearch = { answer?: UatAnswer }

export const Route = createFileRoute('/_portal/portal/tickets/$key')({
  validateSearch: (search: Record<string, unknown>): PortalTicketSearch =>
    UAT_ANSWERS.includes(search.answer as UatAnswer) ? { answer: search.answer as UatAnswer } : {},
  loader: async ({ context: { queryClient }, params }) => {
    const key = normalizeTicketKey(params.key)
    if (!key) throw notFound()
    await loadOrNotFound(() => queryClient.ensureQueryData(ticketQueryOptions(key)))
    await Promise.all([
      queryClient.ensureQueryData(ticketMessagesQueryOptions(key)),
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
  const { answer } = Route.useSearch()
  const navigate = Route.useNavigate()
  const clearAnswer = useCallback(() => void navigate({ search: {}, replace: true }), [navigate])
  return <PortalTicketPage ticketKey={key} answer={answer} onAnswerShown={clearAnswer} />
}
