import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { DodConfig, Project } from '../../api/client'
import { Checkbox } from '../../components/Checkbox'
import { EditableCard } from '../../components/EditableCard'
import { ErrorAlert } from '../../components/ErrorAlert'
import { DodConditions, DodPreview } from '../dod/DodConditions'
import { useSaveProjectDod } from './queries'

type ProjectDod = { dod: DodConfig; allowTicketOverride: boolean }

type ProjectDodCardProps = {
  project: Project
  canEdit: boolean
  onEditStart: () => void
  onSaved: () => void
}

/**
 * The project's Definition of Done (HANDOFF §7, Projects mock): what has to happen before a
 * ticket resolves by itself, read compactly, and a form behind "Edit DoD". Give it
 * key={project.id} so switching projects discards unsaved changes.
 */
export function ProjectDodCard({ project, canEdit, onEditStart, onSaved }: ProjectDodCardProps) {
  const { t } = useTranslation()
  const [draft, setDraft] = useState<ProjectDod | null>(null)
  const save = useSaveProjectDod(project.id)

  return (
    <EditableCard
      tone="emphasis"
      title={t('projects.dod.title')}
      description={t('projects.dod.description')}
      editLabel={t('projects.dod.edit')}
      saveLabel={t('projects.dod.save')}
      canEdit={canEdit}
      isEditing={draft !== null}
      isPending={save.isPending}
      onEdit={() => {
        onEditStart()
        setDraft({ dod: structuredClone(project.dod), allowTicketOverride: project.allowTicketOverride })
      }}
      onCancel={() => {
        save.reset()
        setDraft(null)
      }}
      onSave={() => {
        if (!draft) return
        save.mutate(draft, {
          onSuccess: () => {
            setDraft(null)
            onSaved()
          },
        })
      }}
    >
      {draft ? (
        <>
          <DodConditions
            dod={draft.dod}
            hasRepository={project.hasRepository}
            legend={t('projects.dod.conditionsLabel')}
            onChange={(dod) => setDraft({ ...draft, dod })}
          />
          <DodPreview dod={draft.dod} />
          <Checkbox
            isSelected={draft.allowTicketOverride}
            onChange={(allowTicketOverride) => setDraft({ ...draft, allowTicketOverride })}
            description={t('projects.dod.allowOverride.hint')}
          >
            {t('projects.dod.allowOverride.label')}
          </Checkbox>
          <ErrorAlert error={save.error} />
        </>
      ) : (
        <>
          <DodPreview dod={project.dod} />
          <p className="text-sm text-ink-2">
            {t(project.allowTicketOverride ? 'projects.dod.overrideOn' : 'projects.dod.overrideOff')}
          </p>
        </>
      )}
    </EditableCard>
  )
}
