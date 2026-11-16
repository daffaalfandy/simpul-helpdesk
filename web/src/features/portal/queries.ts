import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, type Ticket, type UatAnswer } from '../../api/client'
import { queryKeys, useInvalidate } from '../../api/queries'
import { unwrap } from '../../api/unwrap'

/** The tests waiting on the signed-in requester (portal home). */
export const uatRequestsQueryOptions = queryOptions({
  queryKey: queryKeys.uatRequests,
  queryFn: () => unwrap(api.GET('/uat-requests')),
})

/**
 * The client's answer to a test, or a reopen: both return the ticket. Lists, the home's tests and
 * notifications refresh; a refused answer (someone answered first) reloads the ticket.
 */
function useClientAction<T>(key: string, call: (input: T) => Promise<Ticket>) {
  const queryClient = useQueryClient()
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: call,
    onSuccess: (ticket) => {
      queryClient.setQueryData(queryKeys.ticket(key), ticket)
      return invalidate(queryKeys.tickets, queryKeys.ticketMessages(key), queryKeys.notifications)
    },
    onError: () => invalidate(queryKeys.ticket(key), queryKeys.uatRequests),
  })
}

export function useAnswerUat(key: string) {
  return useClientAction(key, (input: { answer: UatAnswer; message?: string }) =>
    unwrap(api.POST('/tickets/{ticketKey}/uat', { params: { path: { ticketKey: key } }, body: input })),
  )
}

export function useReopen(key: string) {
  return useClientAction(key, () =>
    unwrap(api.POST('/tickets/{ticketKey}/reopen', { params: { path: { ticketKey: key } } })),
  )
}
