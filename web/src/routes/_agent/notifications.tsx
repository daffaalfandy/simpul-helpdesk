import { createFileRoute } from '@tanstack/react-router'

import { organizationQueryOptions } from '../../api/queries'
import { NotificationsPage } from '../../features/notifications/NotificationsPage'
import { DEFAULT_FILTER } from '../../features/notifications/notice'
import { notificationSettingsQueryOptions, notificationsQueryOptions } from '../../features/notifications/queries'

export const Route = createFileRoute('/_agent/notifications')({
  loader: ({ context: { queryClient } }) =>
    Promise.all([
      queryClient.ensureQueryData(notificationSettingsQueryOptions),
      queryClient.ensureInfiniteQueryData(notificationsQueryOptions(DEFAULT_FILTER)),
      queryClient.ensureQueryData(organizationQueryOptions),
    ]),
  component: NotificationsPage,
})
