import { createFileRoute, redirect } from '@tanstack/react-router'

import { installQueryOptions } from '../api/queries'
import { SetupPage } from '../features/auth/SetupPage'
import { paths } from '../lib/constants'

// First-run setup: open until the first administrator exists, then it sends people to sign in.
export const Route = createFileRoute('/setup')({
  beforeLoad: async ({ context }) => {
    const install = await context.queryClient.ensureQueryData(installQueryOptions)
    if (!install.setupRequired) throw redirect({ to: paths.login })
  },
  component: SetupPage,
})
