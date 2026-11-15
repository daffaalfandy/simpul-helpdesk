import { infiniteQueryOptions, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, type EmailFrequency, type NotificationEvent, type NotificationFilter, type NotificationSettings } from '../../api/client'
import { queryKeys, useInvalidate } from '../../api/queries'
import { unwrap } from '../../api/unwrap'
import { BELL_ITEM_COUNT, NOTIFICATION_PAGE_SIZE, NOTIFICATION_POLL_MS } from '../../lib/constants'

/** The inbox, newest first, paged by the last id seen. */
export const notificationsQueryOptions = (filter: NotificationFilter) =>
  infiniteQueryOptions({
    queryKey: queryKeys.notificationList(filter),
    queryFn: ({ pageParam }) =>
      unwrap(api.GET('/notifications', { params: { query: { filter, before: pageParam, limit: NOTIFICATION_PAGE_SIZE } } })),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.nextBefore ?? undefined,
  })

/** The bell's newest items; fetched when its panel opens. */
export const bellQueryOptions = (filter: NotificationFilter) =>
  queryOptions({
    queryKey: [...queryKeys.notificationList(filter), 'bell'],
    queryFn: () => unwrap(api.GET('/notifications', { params: { query: { filter, limit: BELL_ITEM_COUNT } } })),
  })

/** The bell's count, polled (and refetched when the window regains focus). */
export const unreadCountQueryOptions = queryOptions({
  queryKey: queryKeys.unreadCount,
  queryFn: () => unwrap(api.GET('/notifications/unread-count')),
  refetchInterval: NOTIFICATION_POLL_MS,
})

export const notificationSettingsQueryOptions = queryOptions({
  queryKey: queryKeys.notificationSettings,
  queryFn: () => unwrap(api.GET('/notification-settings')),
})

export function useMarkRead() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: ({ id, read }: { id: string; read: boolean }) =>
      unwrap(api.PUT('/notifications/{notificationId}/read', { params: { path: { notificationId: id } }, body: { read } })),
    onSettled: () => invalidate(queryKeys.notifications),
  })
}

export function useMarkAllRead() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: () => unwrap(api.POST('/notifications/read-all')),
    onSettled: () => invalidate(queryKeys.notifications),
  })
}

/** Switches one event; the switch moves at once and moves back if the save fails. */
export function useSetEventPreference() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ event, web, email }: { event: NotificationEvent; web: boolean; email: boolean }) =>
      unwrap(api.PUT('/notification-settings/events/{event}', { params: { path: { event } }, body: { web, email } })),
    onMutate: async ({ event, web, email }) => {
      await queryClient.cancelQueries({ queryKey: queryKeys.notificationSettings })
      const before = queryClient.getQueryData<NotificationSettings>(queryKeys.notificationSettings)
      if (before) {
        queryClient.setQueryData<NotificationSettings>(queryKeys.notificationSettings, {
          ...before,
          events: before.events.map((e) => (e.event === event ? { ...e, web, email } : e)),
        })
      }
      return { before }
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.before) queryClient.setQueryData(queryKeys.notificationSettings, ctx.before)
    },
    onSuccess: (settings) => queryClient.setQueryData(queryKeys.notificationSettings, settings),
  })
}

export function useSetEmailFrequency() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (emailFrequency: EmailFrequency) =>
      unwrap(api.PUT('/notification-settings/email-frequency', { body: { emailFrequency } })),
    onSuccess: (settings) => queryClient.setQueryData(queryKeys.notificationSettings, settings),
  })
}
