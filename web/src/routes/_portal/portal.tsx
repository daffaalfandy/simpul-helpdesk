import { createFileRoute } from '@tanstack/react-router'

import { PortalHomePage } from '../../features/shell/pages'

export const Route = createFileRoute('/_portal/portal')({ component: PortalHomePage })
