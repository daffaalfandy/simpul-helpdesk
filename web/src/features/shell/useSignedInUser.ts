import { useSuspenseQuery } from '@tanstack/react-query'

import { meQueryOptions } from '../../api/queries'

/** The signed-in user inside guarded layouts. */
export function useSignedInUser() {
  const { data: me } = useSuspenseQuery(meQueryOptions)
  if (!me) throw new Error('route guard should have redirected anonymous users')
  return me
}
