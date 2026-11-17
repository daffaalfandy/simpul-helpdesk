import { queryOptions, useMutation } from '@tanstack/react-query'

import type { PersonalTokenInput } from '../../api/client'
import { api } from '../../api/client'
import { queryKeys, useInvalidate } from '../../api/queries'
import { unwrap } from '../../api/unwrap'

/** The user's personal access tokens in the active organization, and the MCP address. */
export const tokensQueryOptions = queryOptions({
  queryKey: queryKeys.tokens,
  queryFn: () => unwrap(api.GET('/me/tokens')),
})

export function useCreateToken() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: (body: PersonalTokenInput) => unwrap(api.POST('/me/tokens', { body })),
    onSuccess: () => invalidate(queryKeys.tokens),
  })
}

export function useRevokeToken() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: (tokenId: string) => unwrap(api.DELETE('/me/tokens/{tokenId}', { params: { path: { tokenId } } })),
    onSuccess: () => invalidate(queryKeys.tokens),
  })
}
