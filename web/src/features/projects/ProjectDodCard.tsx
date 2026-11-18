import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ApiError, ErrorCode, type DodConfig, type Project } from '../../api/client'
import { Checkbox } from '../../components/Checkbox'
import { EditableCard } from '../../components/EditableCard'
import { ErrorAlert } from '../../components/ErrorAlert'
import { DodConditions, DodPreview } from '../dod/DodConditions'
import { codePaused, forRepository } from '../dod/labels'
import { useSaveProjectDod } from './queries'

type ProjectDod = { dod: DodConfig; mandatory: boolean; allowTicketOverride: boolean }

type ProjectDodCardProps = {
  project: Project
  canEdit: boolean
  onEditStart: () => void
  onSaved: () => void
}

/**
 * The project's Definition of Done (HANDOFF §7, Projects mock): what has to happen before a
 * ticket resolves by itself, whether resolving without it needs a reason, read compactly, and a
 * form behind "Edit DoD". Give it key={project.id} so switching projects discards unsaved changes.
 */
export function ProjectDodCard({ project, canEdit, onEditStart, onSaved }: ProjectDodCardProps) {
  const { t } = useTranslation()
  const [draft, setDraft] = useState<ProjectDod | null>(null)
  const save = useSaveProjectDod(project.id)
  const hasRepository = project.repository !== null
  const branchInvalid = save.error instanceof ApiError && save.error.code === ErrorCode.InvalidBranch

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
        setDraft({
          // Without a repository the code condition can't be saved on, so the form starts with it off.
          dod: forRepository(structuredClone(project.dod), hasRepository),
          mandatory: project.dodMandatory,
          allowTicketOverride: project.allowTicketOverride,
        })
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
            legend={t('projects.dod.conditionsLabel')}
            hasRepository={hasRepository}
            branchInvalid={branchInvalid}
            onChange={(dod) => setDraft({ ...draft, dod })}
          />
          <DodPreview dod={draft.dod} />
          <Checkbox
            isSelected={draft.mandatory}
            onChange={(mandatory) => setDraft({ ...draft, mandatory })}
            description={t('projects.dod.mandatory.hint')}
          >
            {t('projects.dod.mandatory.label')}
          </Checkbox>
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
          <DodPreview dod={forRepository(project.dod, hasRepository)} />
          {codePaused(project.dod, hasRepository) && (
            <p className="text-sm text-ink-2">{t('projects.dod.codePaused', { branch: project.dod.codeOnBranch.branch })}</p>
          )}
          <p className="text-sm text-ink-2">{t(project.dodMandatory ? 'projects.dod.mandatoryOn' : 'projects.dod.mandatoryOff')}</p>
          <p className="text-sm text-ink-2">
            {t(project.allowTicketOverride ? 'projects.dod.overrideOn' : 'projects.dod.overrideOff')}
          </p>
        </>
      )}
    </EditableCard>
  )
}
