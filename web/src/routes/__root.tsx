import type { QueryClient } from '@tanstack/react-query'
import { createRootRouteWithContext, Outlet, useRouter } from '@tanstack/react-router'
import { I18nProvider, RouterProvider } from 'react-aria-components'

import { NotFoundPage } from '../features/shell/pages'
import { DATE_LOCALE } from '../i18n/locale'

export interface RouterContext {
  queryClient: QueryClient
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: Root,
  notFoundComponent: NotFoundPage,
})

/**
 * Date and time fields format with the app's locale, not the browser's. React Aria links (board
 * cards) take a built href and navigate through the app's router instead of reloading the page.
 */
function Root() {
  const router = useRouter()
  return (
    <RouterProvider navigate={(href) => router.navigate({ href })}>
      <I18nProvider locale={DATE_LOCALE}>
        <Outlet />
      </I18nProvider>
    </RouterProvider>
  )
}
