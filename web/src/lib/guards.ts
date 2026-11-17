import type { QueryClient } from '@tanstack/react-query'
import { redirect, type ParsedLocation } from '@tanstack/react-router'

import type { Me } from '../api/client'
import { installQueryOptions, meQueryOptions } from '../api/queries'
import { paths } from './constants'
import { homePathFor, type OrgRole } from './roles'

/**
 * Returns the signed-in user or redirects to sign-in, remembering where they were going. On a
 * fresh install nobody can sign in yet, so it sends people to first-run setup instead.
 */
export async function requireUser(queryClient: QueryClient, location: ParsedLocation): Promise<Me> {
  const me = await queryClient.ensureQueryData(meQueryOptions)
  if (me) return me
  await redirectToSetupIfOpen(queryClient)
  throw redirect({ to: paths.login, search: { redirect: location.href } })
}

/** Sends people to first-run setup while the install has no administrator. */
export async function redirectToSetupIfOpen(queryClient: QueryClient): Promise<void> {
  const install = await queryClient.ensureQueryData(installQueryOptions)
  if (install.setupRequired) throw redirect({ to: paths.setup })
}

/**
 * Everything in the app belongs to an organization. An administrator without one creates the
 * first (first run); staff who belong to none wait on their own page.
 */
export function requireOrganization(me: Me): void {
  if (!me.organization) throw redirect({ to: me.isAdmin ? paths.setupOrganization : paths.noOrganization })
}

/** Sends users who may not see a page to their own start page. */
export function requireRole(me: Me, allowed: (role: OrgRole) => boolean): void {
  if (!allowed(me.role)) throw redirect({ to: homePathFor(me.role) })
}
