import { QueryClient } from '@tanstack/react-query'
import { createRouter, type RouterHistory } from '@tanstack/react-router'

import { ApiError } from './api/client'
import { RouteError } from './features/shell/pages'
import { routeTree } from './routeTree.gen'

const MAX_QUERY_RETRIES = 2
const HTTP_SERVER_ERROR = 500

export function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        // Client errors (4xx) won't fix themselves; retry only network and server errors.
        retry: (failures, error) =>
          failures < MAX_QUERY_RETRIES && !(error instanceof ApiError && error.status < HTTP_SERVER_ERROR),
      },
    },
  })
}

export function createAppRouter(queryClient: QueryClient, history?: RouterHistory) {
  return createRouter({
    routeTree,
    history,
    context: { queryClient },
    defaultPreload: 'intent',
    // Loaders read from TanStack Query, which has its own cache.
    defaultPreloadStaleTime: 0,
    scrollRestoration: true,
    defaultErrorComponent: RouteError,
  })
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof createAppRouter>
  }
}
