import { createFileRoute } from '@tanstack/react-router'

import { CatalogPage } from '../../../features/settings/CatalogPage'
import { catalogItemsQueryOptions } from '../../../features/settings/queries'

export const Route = createFileRoute('/_agent/settings/catalog')({
  loader: ({ context }) => context.queryClient.ensureQueryData(catalogItemsQueryOptions),
  component: CatalogPage,
})
