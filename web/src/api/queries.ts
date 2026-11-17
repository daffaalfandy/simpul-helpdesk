import { type QueryClient, queryOptions, useMutation, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { notFound, useRouter } from '@tanstack/react-router'

import { ME_STALE_TIME_MS } from '../lib/constants'
import { api, ApiError, ErrorCode, HttpStatus, type Me, type Priority, type PriorityLevel, type SetupRequest } from './client'
import { setCurrentOrganization } from './organization'
import { unwrap } from './unwrap'

export const queryKeys = {
  me: ['me'],
  install: ['install'],
  organization: ['organization'],
  organizations: ['organizations'],
  users: ['users'],
  clients: ['clients'],
  priorities: ['priorities'],
  projects: ['projects'],
  project: (id: string) => ['projects', id] as const,
  businessHours: ['business-hours'],
  holidays: ['holidays'],
  holidaysForYear: (year: number) => ['holidays', year] as const,
  holidayPresets: ['holiday-presets'],
  catalogItems: ['catalog-items'],
  attachmentPolicy: ['attachment-policy'],
  intake: ['intake'],
  intakeFor: (clientId: string | null) => ['intake', clientId ?? 'own'] as const,
  tickets: ['tickets'],
  ticketList: (filters: object) => ['tickets', 'list', filters] as const,
  board: (filters: object) => ['tickets', 'board', filters] as const,
  boards: ['tickets', 'board'],
  ticket: (key: string) => ['tickets', 'detail', key] as const,
  ticketMessages: (key: string) => ['tickets', 'messages', key] as const,
  ticketActivity: (key: string) => ['tickets', 'activity', key] as const,
  uatRequests: ['tickets', 'uat-requests'],
  clientRequesters: (clientId: string) => ['clients', clientId, 'requesters'] as const,
  clientAssignees: (clientId: string) => ['clients', clientId, 'assignees'] as const,
  ticketWatchers: (key: string) => ['tickets', 'watchers', key] as const,
  watcherCandidatesFor: (key: string) => ['tickets', 'watcher-candidates', key] as const,
  watcherCandidates: (key: string, q: string) => ['tickets', 'watcher-candidates', key, q] as const,
  notifications: ['notifications'],
  notificationList: (filter: string) => ['notifications', 'list', filter] as const,
  unreadCount: ['notifications', 'unread'],
  notificationSettings: ['notification-settings'],
  integrations: ['integrations'],
  codeEvents: ['integrations', 'events'],
  commitTest: (message: string) => ['integrations', 'commit-test', message] as const,
  ticketCode: (key: string) => ['tickets', 'code', key] as const,
  slaReport: (month: string | undefined) => ['reports', 'sla', month ?? 'current'] as const,
  members: ['members'],
  tokens: ['tokens'],
  projectFilter: ['project-filter'],
} as const

/** Invalidates queries by key prefix. */
export function useInvalidate() {
  const queryClient = useQueryClient()
  return (...keys: ReadonlyArray<readonly unknown[]>) =>
    Promise.all(keys.map((queryKey) => queryClient.invalidateQueries({ queryKey })))
}

/** Not-found data for a record in another organization the user works in. */
type Elsewhere = { organizationId: string }

/**
 * Runs a route loader's fetch, turning a 404 into the router's not-found page. A record in
 * another of the user's organizations is not found here too, naming that organization so the
 * page can offer the switch.
 */
export async function loadOrNotFound<T>(load: () => Promise<T>): Promise<T> {
  try {
    return await load()
  } catch (error) {
    if (error instanceof ApiError && error.status === HttpStatus.NotFound) throw notFound()
    if (error instanceof ApiError && error.code === ErrorCode.OtherOrganization && error.detail) {
      throw notFound({ data: { organizationId: error.detail } satisfies Elsewhere })
    }
    throw error
  }
}

/** The organization a not-found record lives in, when it is one the user works in. */
export function organizationElsewhere(data: unknown): string | undefined {
  if (typeof data !== 'object' || data === null || !('organizationId' in data)) return undefined
  const { organizationId } = data as Elsewhere
  return organizationId
}

export const clientsQueryOptions = queryOptions({
  queryKey: queryKeys.clients,
  queryFn: () => unwrap(api.GET('/clients')),
})

/** Priority names and descriptions (Settings › Priorities); the signed-in layouts preload them. */
export const priorityLevelsQueryOptions = queryOptions({
  queryKey: queryKeys.priorities,
  queryFn: () => unwrap(api.GET('/priorities')),
  staleTime: Infinity,
})

export type PriorityNames = Record<Priority, { name: string; description: string }>

function toNames(levels: PriorityLevel[]): PriorityNames {
  return Object.fromEntries(levels.map((l) => [l.code, { name: l.name, description: l.description }])) as PriorityNames
}

/** Priority names by code, for labels like "P1 · Critical". */
export function usePriorityNames(): PriorityNames {
  return useSuspenseQuery({ ...priorityLevelsQueryOptions, select: toNames }).data
}

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
    // The API answers for the tab's organization, or the user's last one: keep the tab on it.
    setCurrentOrganization(data.organization?.id ?? null)
    return data
  },
  staleTime: ME_STALE_TIME_MS,
})

/** The install: whether first-run setup is open. Public, for the sign-in and setup pages. */
export const installQueryOptions = queryOptions({
  queryKey: queryKeys.install,
  queryFn: async () => {
    const { data, error, response } = await api.GET('/install')
    if (error || !data) throw new ApiError(response, error)
    return data
  },
  staleTime: Infinity,
})

/** The organization the user works in (signed in). */
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
    onSuccess: (me) => signedIn(queryClient, me),
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
      setCurrentOrganization(null)
      queryClient.setQueryData(queryKeys.me, null)
      queryClient.removeQueries({ predicate: (q) => !outlivesOrganization(q.queryKey) })
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

/** Sets the first password from an invite link; the person is signed in straight away. */
export function useAcceptInvite() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (body: { token: string; password: string }) => unwrap(api.POST('/auth/invite/accept', { body })),
    onSuccess: (me) => signedIn(queryClient, me),
  })
}

/** First-run setup signs the new administrator in and renames the organisation. */
export function useCompleteSetup() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (body: SetupRequest) => unwrap(api.POST('/setup', { body })),
    onSuccess: (me) => signedIn(queryClient, me),
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

/** The install and the signed-in user stay; everything else belongs to one organization. */
function outlivesOrganization(queryKey: readonly unknown[]): boolean {
  return queryKey[0] === queryKeys.install[0] || queryKey[0] === queryKeys.me[0]
}

/** A fresh session: the tab works where the API put the user. */
function signedIn(queryClient: QueryClient, me: Me) {
  setCurrentOrganization(me.organization?.id ?? null)
  queryClient.setQueryData(queryKeys.me, me)
}

/**
 * Switches the tab to another organization: every request names it from now on, the API
 * remembers it for the next sign-in, and everything cached for the old one goes. What is on
 * screen refetches, and route guards and loaders run again for the new role.
 */
export function useSwitchOrganization() {
  const queryClient = useQueryClient()
  const router = useRouter()
  return useMutation({
    // The answer names the organization; the tab moves only once the API accepted it.
    mutationFn: (organizationId: string) => unwrap(api.PUT('/me/organization', { body: { organizationId } })),
    onSuccess: async (me) => {
      await queryClient.cancelQueries()
      signedIn(queryClient, me)
      await queryClient.resetQueries({ predicate: (q) => !outlivesOrganization(q.queryKey) })
      await router.invalidate()
    },
  })
}
