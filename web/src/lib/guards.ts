import type { QueryClient } from '@tanstack/react-query'
import { redirect, type ParsedLocation } from '@tanstack/react-router'

import type { Me, Role } from '../api/client'
import { meQueryOptions } from '../api/queries'
import { paths } from './constants'
import { homePathFor } from './roles'

/** Returns the signed-in user or redirects to sign-in, remembering where they were going. */
export async function requireUser(queryClient: QueryClient, location: ParsedLocation): Promise<Me> {
  const me = await queryClient.ensureQueryData(meQueryOptions)
  if (!me) throw redirect({ to: paths.login, search: { redirect: location.href } })
  return me
}

/** Sends users who may not see a page to their own start page. */
export function requireRole(me: Me, allowed: (role: Role) => boolean): void {
  if (!allowed(me.role)) throw redirect({ to: homePathFor(me.role) })
}
