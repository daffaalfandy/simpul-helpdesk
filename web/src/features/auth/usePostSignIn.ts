import { useNavigate } from '@tanstack/react-router'

import type { Me } from '../../api/client'
import { paths } from '../../lib/constants'
import { homePathFor, picksOrganization } from '../../lib/roles'
import { safeRedirect } from '../../lib/text'

/** Where a fresh session goes: people who work in several organizations pick one first. */
export function usePostSignIn() {
  const navigate = useNavigate()
  return (me: Me, redirectTo?: string) => {
    const target = safeRedirect(redirectTo)
    if (picksOrganization(me)) {
      void navigate({ to: paths.selectOrganization, search: target ? { redirect: target } : {}, replace: true })
      return
    }
    void navigate({ href: target ?? homePathFor(me.role), replace: true })
  }
}
