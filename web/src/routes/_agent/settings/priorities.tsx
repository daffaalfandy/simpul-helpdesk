import { createFileRoute } from '@tanstack/react-router'

import { priorityLevelsQueryOptions } from '../../../api/queries'
import { PrioritiesPage } from '../../../features/settings/PrioritiesPage'

export const Route = createFileRoute('/_agent/settings/priorities')({
  loader: ({ context }) => context.queryClient.ensureQueryData(priorityLevelsQueryOptions),
  component: PrioritiesPage,
})
