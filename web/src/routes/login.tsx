import { createFileRoute, redirect } from '@tanstack/react-router'

import { meQueryOptions } from '../api/queries'
import { LoginPage } from '../features/auth/LoginPage'
import { homePathFor } from '../lib/roles'
import { safeRedirect } from '../lib/text'

type LoginSearch = { redirect?: string }

export const Route = createFileRoute('/login')({
  validateSearch: (search: Record<string, unknown>): LoginSearch => {
    const target = safeRedirect(search.redirect)
    return target ? { redirect: target } : {}
  },
  beforeLoad: async ({ context }) => {
    const me = await context.queryClient.ensureQueryData(meQueryOptions)
    if (me) throw redirect({ to: homePathFor(me.role) })
  },
  component: LoginRoute,
})

function LoginRoute() {
  const search = Route.useSearch()
  return <LoginPage redirectTo={search.redirect} />
}
