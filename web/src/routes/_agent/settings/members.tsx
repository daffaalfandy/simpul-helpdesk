import { createFileRoute } from '@tanstack/react-router'

import { ComingSoonSection } from '../../../features/settings/shared'

export const Route = createFileRoute('/_agent/settings/members')({
  component: () => <ComingSoonSection section="members" />,
})
