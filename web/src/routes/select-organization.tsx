import { createFileRoute, redirect } from '@tanstack/react-router'

import { OrganizationPicker } from '../features/auth/OrganizationPicker'
import { requireUser } from '../lib/guards'
import { homePathFor, picksOrganization } from '../lib/roles'
import { safeRedirect } from '../lib/text'

type PickSearch = { redirect?: string }

// After sign-in, people who work in several organizations choose where this tab works.
export const Route = createFileRoute('/select-organization')({
  validateSearch: (search: Record<string, unknown>): PickSearch => {
    const target = safeRedirect(search.redirect)
    return target ? { redirect: target } : {}
  },
  beforeLoad: async ({ context, location }) => {
    const me = await requireUser(context.queryClient, location)
    if (!picksOrganization(me)) throw redirect({ to: homePathFor(me.role) })
  },
  component: PickRoute,
})

function PickRoute() {
  const search = Route.useSearch()
  return <OrganizationPicker redirectTo={search.redirect} />
}
