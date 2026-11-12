import type { QueryClient } from '@tanstack/react-query'
import { createRootRouteWithContext, Outlet } from '@tanstack/react-router'
import { I18nProvider } from 'react-aria-components'

import { NotFoundPage } from '../features/shell/pages'
import { DATE_LOCALE } from '../i18n/locale'

export interface RouterContext {
  queryClient: QueryClient
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: Root,
  notFoundComponent: NotFoundPage,
})

/** Date and time fields format with the app's locale, not the browser's. */
function Root() {
  return (
    <I18nProvider locale={DATE_LOCALE}>
      <Outlet />
    </I18nProvider>
  )
}
