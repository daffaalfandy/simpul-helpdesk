import { queryOptions, useMutation, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { notFound } from '@tanstack/react-router'

import { ME_STALE_TIME_MS } from '../lib/constants'
import { api, ApiError, HttpStatus, type Me } from './client'
import { unwrap } from './unwrap'

export const queryKeys = {
  me: ['me'],
  organization: ['organization'],
  users: ['users'],
  clients: ['clients'],
  slaPlans: ['sla-plans'],
  businessHours: ['business-hours'],
  holidays: ['holidays'],
  holidaysForYear: (year: number) => ['holidays', year] as const,
  holidayPresets: ['holiday-presets'],
  systems: ['systems'],
  catalogItems: ['catalog-items'],
  attachmentPolicy: ['attachment-policy'],
  intake: ['intake'],
  intakeFor: (clientId: string | null) => ['intake', clientId ?? 'own'] as const,
  tickets: ['tickets'],
  ticketList: (filters: object) => ['tickets', 'list', filters] as const,
  ticket: (key: string) => ['tickets', 'detail', key] as const,
  ticketMessages: (key: string) => ['tickets', 'messages', key] as const,
  ticketActivity: (key: string) => ['tickets', 'activity', key] as const,
  clientRequesters: (clientId: string) => ['clients', clientId, 'requesters'] as const,
  clientAssignees: (clientId: string) => ['clients', clientId, 'assignees'] as const,
} as const

/** Invalidates queries by key prefix. */
export function useInvalidate() {
  const queryClient = useQueryClient()
  return (...keys: ReadonlyArray<readonly unknown[]>) =>
    Promise.all(keys.map((queryKey) => queryClient.invalidateQueries({ queryKey })))
}

/** Runs a route loader's fetch, turning a 404 into the router's not-found page. */
export async function loadOrNotFound<T>(load: () => Promise<T>): Promise<T> {
  try {
    return await load()
  } catch (error) {
    if (error instanceof ApiError && error.status === HttpStatus.NotFound) throw notFound()
    throw error
  }
}

export const clientsQueryOptions = queryOptions({
  queryKey: queryKeys.clients,
  queryFn: () => unwrap(api.GET('/clients')),
})

export const attachmentPolicyQueryOptions = queryOptions({
  queryKey: queryKeys.attachmentPolicy,
  queryFn: () => unwrap(api.GET('/attachment-policy')),
  staleTime: Infinity,
})

/** The signed-in user, or null when signed out. */
export const meQueryOptions = queryOptions({
  queryKey: queryKeys.me,
  queryFn: async (): Promise<Me | null> => {
    const { data, error, response } = await api.GET('/auth/me')
    if (response.status === HttpStatus.Unauthorized) return null
    if (error || !data) throw new ApiError(response, error)
    return data
  },
  staleTime: ME_STALE_TIME_MS,
})

export const organizationQueryOptions = queryOptions({
  queryKey: queryKeys.organization,
  queryFn: async () => {
    const { data, error, response } = await api.GET('/organization')
    if (error || !data) throw new ApiError(response, error)
    return data
  },
  staleTime: Infinity,
})

/** The organisation's time zone; every timestamp is shown in it. Routes preload the organisation. */
export function useOrgTimeZone(): string {
  return useSuspenseQuery(organizationQueryOptions).data.timezone
}

export function useLogin() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body: { email: string; password: string }) => {
      const { data, error, response } = await api.POST('/auth/login', { body })
      if (error || !data) throw new ApiError(response, error)
      return data
    },
    onSuccess: (me) => queryClient.setQueryData(queryKeys.me, me),
  })
}

export function useLogout() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async () => {
      const { error, response } = await api.POST('/auth/logout')
      if (error) throw new ApiError(response, error)
    },
    onSettled: () => {
      queryClient.setQueryData(queryKeys.me, null)
      queryClient.removeQueries({ predicate: (q) => q.queryKey[0] !== queryKeys.organization[0] && q.queryKey[0] !== queryKeys.me[0] })
    },
  })
}

export function useForgotPassword() {
  return useMutation({
    mutationFn: async (email: string) => {
      const { error, response } = await api.POST('/auth/password/forgot', { body: { email } })
      if (error) throw new ApiError(response, error)
    },
  })
}

export function useResetPassword() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body: { token: string; password: string }) => {
      const { error, response } = await api.POST('/auth/password/reset', { body })
      if (error) throw new ApiError(response, error)
    },
    onSuccess: () => queryClient.setQueryData(queryKeys.me, null),
  })
}
