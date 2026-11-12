import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { ME_STALE_TIME_MS } from '../lib/constants'
import { api, ApiError, HttpStatus, type Me } from './client'

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
} as const

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
