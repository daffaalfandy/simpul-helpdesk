import { infiniteQueryOptions, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import type {
  Priority,
  RequesterInput,
  Ticket,
  TicketAssigneeFilter,
  TicketInput,
  TicketMessageInput,
  TicketStatus,
} from '../../api/client'
import { api } from '../../api/client'
import { queryKeys, useInvalidate } from '../../api/queries'
import { unwrap } from '../../api/unwrap'
import { TICKET_PAGE_SIZE } from '../../lib/constants'

export type TicketListFilters = {
  status?: TicketStatus[]
  clientId?: string
  assignee?: TicketAssigneeFilter
  q?: string
}

/** Newest first, paged by the last id seen. */
export const ticketListQueryOptions = (filters: TicketListFilters) =>
  infiniteQueryOptions({
    queryKey: queryKeys.ticketList(filters),
    queryFn: ({ pageParam }) =>
      unwrap(api.GET('/tickets', { params: { query: { ...filters, before: pageParam, limit: TICKET_PAGE_SIZE } } })),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => (last.length === TICKET_PAGE_SIZE ? last[last.length - 1]?.id : undefined),
  })

export const ticketQueryOptions = (key: string) =>
  queryOptions({
    queryKey: queryKeys.ticket(key),
    queryFn: () => unwrap(api.GET('/tickets/{ticketKey}', { params: { path: { ticketKey: key } } })),
  })

export const ticketMessagesQueryOptions = (key: string) =>
  queryOptions({
    queryKey: queryKeys.ticketMessages(key),
    queryFn: () => unwrap(api.GET('/tickets/{ticketKey}/messages', { params: { path: { ticketKey: key } } })),
  })

export const ticketActivityQueryOptions = (key: string) =>
  queryOptions({
    queryKey: queryKeys.ticketActivity(key),
    queryFn: () => unwrap(api.GET('/tickets/{ticketKey}/activity', { params: { path: { ticketKey: key } } })),
  })

/** Report-form choices: the requester's own client (null) or a client staff picked. */
export const intakeQueryOptions = (clientId: string | null) =>
  queryOptions({
    queryKey: queryKeys.intakeFor(clientId),
    queryFn: () => unwrap(api.GET('/intake', { params: { query: clientId ? { clientId } : {} } })),
  })

export const clientRequestersQueryOptions = (clientId: string) =>
  queryOptions({
    queryKey: queryKeys.clientRequesters(clientId),
    queryFn: () => unwrap(api.GET('/clients/{clientId}/requesters', { params: { path: { clientId } } })),
  })

export const clientAssigneesQueryOptions = (clientId: string) =>
  queryOptions({
    queryKey: queryKeys.clientAssignees(clientId),
    queryFn: () => unwrap(api.GET('/clients/{clientId}/assignees', { params: { path: { clientId } } })),
  })

export function useCreateTicket() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: (body: TicketInput) => unwrap(api.POST('/tickets', { body })),
    onSuccess: () => invalidate(queryKeys.tickets),
  })
}

export function useAddMessage(key: string) {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: (body: TicketMessageInput) =>
      unwrap(api.POST('/tickets/{ticketKey}/messages', { params: { path: { ticketKey: key } }, body })),
    onSuccess: () => invalidate(queryKeys.ticketMessages(key), queryKeys.ticket(key), queryKeys.ticketActivity(key)),
  })
}

/** Status, assignee and priority changes return the updated ticket; lists and the log refresh. */
function useTicketAction<T>(key: string, call: (input: T) => Promise<Ticket>) {
  const queryClient = useQueryClient()
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: call,
    onSuccess: (ticket) => {
      queryClient.setQueryData(queryKeys.ticket(key), ticket)
      return invalidate(queryKeys.tickets, queryKeys.clientAssignees(ticket.client.id))
    },
  })
}

export function useSetStatus(key: string) {
  return useTicketAction(key, (status: TicketStatus) =>
    unwrap(api.PUT('/tickets/{ticketKey}/status', { params: { path: { ticketKey: key } }, body: { status } })),
  )
}

export function useSetAssignee(key: string) {
  return useTicketAction(key, (assigneeId: string | null) =>
    unwrap(api.PUT('/tickets/{ticketKey}/assignee', { params: { path: { ticketKey: key } }, body: { assigneeId } })),
  )
}

export function useSetPriority(key: string) {
  return useTicketAction(key, (input: { priority: Priority; reason: string }) =>
    unwrap(api.PUT('/tickets/{ticketKey}/priority', { params: { path: { ticketKey: key } }, body: input })),
  )
}

export function useCreateRequester(clientId: string) {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: (body: RequesterInput) =>
      unwrap(api.POST('/clients/{clientId}/requesters', { params: { path: { clientId } }, body })),
    onSuccess: () => invalidate(queryKeys.clientRequesters(clientId)),
  })
}
