import { createFileRoute } from '@tanstack/react-router'

import { CatalogPage } from '../../../features/settings/CatalogPage'
import { businessHoursQueryOptions, catalogItemsQueryOptions, systemsQueryOptions } from '../../../features/settings/queries'

export const Route = createFileRoute('/_agent/settings/catalog')({
  loader: ({ context }) =>
    Promise.all([
      context.queryClient.ensureQueryData(systemsQueryOptions),
      context.queryClient.ensureQueryData(catalogItemsQueryOptions),
      context.queryClient.ensureQueryData(businessHoursQueryOptions),
    ]),
  component: CatalogPage,
})
