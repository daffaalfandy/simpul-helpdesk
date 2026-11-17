import { createFileRoute } from '@tanstack/react-router'

import { McpAccessPage } from '../../../features/account/McpAccessPage'
import { tokensQueryOptions } from '../../../features/account/queries'

export const Route = createFileRoute('/_agent/account/mcp')({
  loader: ({ context: { queryClient } }) => queryClient.ensureQueryData(tokensQueryOptions),
  component: McpAccessPage,
})
