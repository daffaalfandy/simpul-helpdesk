import { createFileRoute } from '@tanstack/react-router'

import { ComingSoonSection } from '../../../features/settings/shared'

export const Route = createFileRoute('/_agent/settings/integrations')({
  component: () => <ComingSoonSection section="integrations" />,
})
