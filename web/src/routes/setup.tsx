import { createFileRoute, redirect } from '@tanstack/react-router'

import { organizationQueryOptions } from '../api/queries'
import { SetupPage } from '../features/auth/SetupPage'
import { paths } from '../lib/constants'

// First-run setup: open until the first administrator exists, then it sends people to sign in.
export const Route = createFileRoute('/setup')({
  beforeLoad: async ({ context }) => {
    const org = await context.queryClient.ensureQueryData(organizationQueryOptions)
    if (!org.setupRequired) throw redirect({ to: paths.login })
  },
  component: SetupPage,
})
