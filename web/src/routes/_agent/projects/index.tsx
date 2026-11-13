import { createFileRoute, redirect } from '@tanstack/react-router'
import { FolderKanban } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '../../../components/EmptyState'
import { projectsQueryOptions } from '../../../features/projects/queries'
import { useSignedInUser } from '../../../features/shell/useSignedInUser'
import { can } from '../../../lib/roles'

// /projects opens the first project; with none, it explains how projects get added.
export const Route = createFileRoute('/_agent/projects/')({
  beforeLoad: async ({ context }) => {
    const [first] = await context.queryClient.ensureQueryData(projectsQueryOptions)
    if (first) throw redirect({ to: '/projects/$projectId', params: { projectId: first.id }, replace: true })
  },
  component: NoProjects,
})

function NoProjects() {
  const { t } = useTranslation()
  const me = useSignedInUser()
  return (
    <EmptyState icon={FolderKanban} title={t('projects.empty.title')}>
      {can.manageProjects(me.role) ? t('projects.empty.body') : t('projects.empty.bodyReadOnly')}
    </EmptyState>
  )
}
