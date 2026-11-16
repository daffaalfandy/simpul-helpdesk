import { useSuspenseQuery } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { Pencil } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert } from '../../components/Alert'
import { Button } from '../../components/Button'
import { ExternalLink } from '../../components/ExternalLink'
import { paths } from '../../lib/constants'
import { can } from '../../lib/roles'
import { businessHoursQueryOptions } from '../settings/queries'
import { useSignedInUser } from '../shell/useSignedInUser'
import { ProjectDialog } from './ProjectDialog'
import { ProjectDodCard } from './ProjectDodCard'
import { projectQueryOptions } from './queries'
import { SlaCard } from './SlaCard'

/**
 * One project: its details, its Definition of Done and its SLA card (Projects mock order). Keyed
 * by project, so switching resets everything.
 */
export function ProjectPage({ projectId }: { projectId: string }) {
  const { t } = useTranslation()
  const me = useSignedInUser()
  const navigate = useNavigate()
  const { data: project } = useSuspenseQuery(projectQueryOptions(projectId))
  const { data: hours } = useSuspenseQuery(businessHoursQueryOptions)
  const [editing, setEditing] = useState(false)
  const [flash, setFlash] = useState<string | null>(null)
  const canManage = can.manageProjects(me.role)
  const meta = [project.client.name, project.isDefault && t('projects.header.default'), !project.active && t('projects.header.hidden')]

  return (
    <>
      <section className="flex flex-wrap items-start justify-between gap-4 rounded-3xl border border-border bg-surface px-5.5 py-5 shadow-card">
        <div className="flex min-w-0 flex-col gap-1">
          <h2 className="text-xl font-extrabold tracking-title">{project.name}</h2>
          <p className="text-sm text-muted">{meta.filter(Boolean).join(t('common.separator'))}</p>
          {project.description && <p className="text-base text-ink-2">{project.description}</p>}
        </div>
        <div className="flex flex-wrap items-start gap-x-6 gap-y-3">
          <div className="flex flex-col gap-0.5 text-sm">
            <span className="text-xs font-bold text-muted">{t('projects.header.repository')}</span>
            {project.repository ? (
              <ExternalLink href={project.repository.webUrl} className="font-bold break-all text-ink">
                {t('projects.header.repositoryName', {
                  provider: t(`code.providers.${project.repository.provider}`),
                  name: project.repository.fullName,
                })}
              </ExternalLink>
            ) : (
              <span className="font-bold text-ink-2">{t('projects.header.noRepository')}</span>
            )}
            {can.manageSettings(me.role) && (
              <Link to={paths.settingsIntegrations} className="font-bold text-primary hover:text-primary-hover">
                {t('projects.header.manageConnection')}
              </Link>
            )}
          </div>
          {canManage && (
            <Button
              variant="outline"
              icon={<Pencil aria-hidden="true" className="size-(--icon-md)" />}
              onPress={() => {
                setFlash(null)
                setEditing(true)
              }}
            >
              {t('projects.editProject')}
            </Button>
          )}
        </div>
      </section>
      {flash && <Alert tone="success">{flash}</Alert>}
      <ProjectDodCard
        key={`dod-${project.id}`}
        project={project}
        canEdit={canManage}
        onEditStart={() => setFlash(null)}
        onSaved={() => setFlash(t('projects.dod.saved'))}
      />
      <SlaCard
        key={project.id}
        project={project}
        canEdit={canManage}
        canOpenSettings={can.manageSettings(me.role)}
        workingDayMinutes={hours.workingDayMinutes}
        onEditStart={() => setFlash(null)}
        onSaved={() => setFlash(t('projects.sla.saved'))}
      />
      {editing && (
        <ProjectDialog
          project={project}
          onClose={() => setEditing(false)}
          onSaved={() => setEditing(false)}
          onDeleted={() => {
            setEditing(false)
            void navigate({ to: paths.projects })
          }}
        />
      )}
    </>
  )
}
