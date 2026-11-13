import { useSuspenseQuery } from '@tanstack/react-query'
import { Link, Outlet, useNavigate } from '@tanstack/react-router'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { Project } from '../../api/client'
import { Pill } from '../../components/Badges'
import { Button } from '../../components/Button'
import { formatTargetShort } from '../../lib/formatDuration'
import { can } from '../../lib/roles'
import { useSignedInUser } from '../shell/useSignedInUser'
import { ProjectDialog } from './ProjectDialog'
import { projectsQueryOptions } from './queries'

/** Projects page (Projects mock): the project list beside the selected project. */
export function ProjectsLayout() {
  const { t } = useTranslation()
  const me = useSignedInUser()
  const navigate = useNavigate()
  const { data: projects } = useSuspenseQuery(projectsQueryOptions)
  const [creating, setCreating] = useState(false)
  const canManage = can.manageProjects(me.role)

  return (
    <>
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex flex-col gap-1">
          <h1 className="text-3xl leading-tight font-extrabold tracking-title">{t('projects.title')}</h1>
          <p className="text-base text-muted">{t('projects.subtitle')}</p>
        </div>
        {canManage && (
          <Button icon={<Plus aria-hidden="true" className="size-(--icon-md)" />} onPress={() => setCreating(true)}>
            {t('projects.newProject')}
          </Button>
        )}
      </div>
      {projects.length === 0 ? (
        <Outlet />
      ) : (
        <div className="flex flex-wrap items-start gap-4.5">
          <nav aria-label={t('projects.listLabel')} className="flex min-w-0 flex-[1_1_300px] flex-col gap-2">
            {projects.map((p) => (
              <ProjectLink key={p.id} project={p} />
            ))}
          </nav>
          <div className="flex min-w-0 flex-[999_1_640px] flex-col gap-3.5">
            <Outlet />
          </div>
        </div>
      )}
      {creating && (
        <ProjectDialog
          onClose={() => setCreating(false)}
          onSaved={(p) => {
            setCreating(false)
            void navigate({ to: '/projects/$projectId', params: { projectId: p.id } })
          }}
        />
      )}
    </>
  )
}

function ProjectLink({ project }: { project: Project }) {
  const { t } = useTranslation()
  const p1 = project.sla.targets.find((target) => target.priority === 'P1')
  return (
    <Link
      to="/projects/$projectId"
      params={{ projectId: project.id }}
      className={
        'flex flex-col gap-0.5 rounded-2xl border-2 border-border bg-surface px-4 py-3.5 text-ink no-underline ' +
        'transition-colors duration-(--duration-fast) ease-standard hover:border-border-strong hover:text-ink ' +
        'data-[status=active]:border-primary data-[status=active]:bg-primary-tint'
      }
    >
      <span className="text-md font-extrabold">{project.name}</span>
      <span className="text-xs text-muted">{project.client.name}</span>
      <span className="mt-1.5 flex flex-wrap gap-1.5">
        {p1 && <Pill tone="danger">{t('projects.chips.sla', { duration: formatTargetShort(t, p1.resolve) })}</Pill>}
        {project.isDefault && <Pill tone="info">{t('projects.chips.default')}</Pill>}
        {!project.active && <Pill tone="neutral">{t('projects.chips.hidden')}</Pill>}
      </span>
    </Link>
  )
}
