import { infiniteQueryOptions, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import type {
  Priority,
  RequesterInput,
  Ticket,
  TicketAssigneeFilter,
  TicketInput,
  TicketMessageInput,
  TicketDodInput,
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

/**
 * Status, assignee and priority changes return the updated ticket; lists and the log refresh. A
 * refused change (someone else moved the ticket first) reloads the ticket, so the page shows
 * the latest and the next try starts from it.
 */
function useTicketAction<T>(key: string, call: (input: T) => Promise<Ticket>) {
  const queryClient = useQueryClient()
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: call,
    onSuccess: (ticket) => {
      queryClient.setQueryData(queryKeys.ticket(key), ticket)
      return invalidate(queryKeys.tickets, queryKeys.clientAssignees(ticket.client.id))
    },
    onError: () => invalidate(queryKeys.ticket(key), queryKeys.ticketActivity(key)),
  })
}

/**
 * Sends the status the page showed, so a move someone else made first isn't undone. Resolving
 * with the Definition of Done unmet carries the reason.
 */
export function useSetStatus(key: string) {
  return useTicketAction(key, (move: { status: TicketStatus; from: TicketStatus; reason?: string }) =>
    unwrap(api.PUT('/tickets/{ticketKey}/status', { params: { path: { ticketKey: key } }, body: move })),
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

/** Colleagues following a ticket ("People notified", besides the requester). */
export const watchersQueryOptions = (key: string) =>
  queryOptions({
    queryKey: queryKeys.ticketWatchers(key),
    queryFn: () => unwrap(api.GET('/tickets/{ticketKey}/watchers', { params: { path: { ticketKey: key } } })),
  })

/** The client's other requesters who could follow the ticket, narrowed by name. */
export const watcherCandidatesQueryOptions = (key: string, q: string) =>
  queryOptions({
    queryKey: queryKeys.watcherCandidates(key, q),
    queryFn: () =>
      unwrap(api.GET('/tickets/{ticketKey}/watcher-candidates', { params: { path: { ticketKey: key }, query: { q } } })),
  })

export function useAddWatcher(key: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (userId: string) =>
      unwrap(api.POST('/tickets/{ticketKey}/watchers', { params: { path: { ticketKey: key } }, body: { userId } })),
    onSuccess: (watchers) => {
      queryClient.setQueryData(queryKeys.ticketWatchers(key), watchers)
      void queryClient.invalidateQueries({ queryKey: queryKeys.watcherCandidatesFor(key) })
    },
  })
}

export function useRemoveWatcher(key: string) {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: (userId: string) =>
      unwrap(api.DELETE('/tickets/{ticketKey}/watchers/{userId}', { params: { path: { ticketKey: key, userId } } })),
    onSettled: () => invalidate(queryKeys.ticketWatchers(key), queryKeys.watcherCandidatesFor(key)),
  })
}

/** Gives the ticket its own Definition of Done with a reason, or returns it to the project's (null). */
export function useSetDod(key: string) {
  return useTicketAction(key, (input: TicketDodInput) =>
    unwrap(api.PUT('/tickets/{ticketKey}/dod', { params: { path: { ticketKey: key } }, body: input })),
  )
}
