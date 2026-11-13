import { useQuery } from '@tanstack/react-query'
import { Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { Project } from '../../api/client'
import { clientsQueryOptions } from '../../api/queries'
import { Button } from '../../components/Button'
import { Checkbox } from '../../components/Checkbox'
import { ConfirmDialog, FormDialog } from '../../components/Dialog'
import { Dropdown } from '../../components/Dropdown'
import { ErrorAlert } from '../../components/ErrorAlert'
import { TextInput } from '../../components/TextInput'
import { useCreateProject, useDeleteProject, useUpdateProject } from './queries'

// Field limits, matching the API (api/internal/projects).
const MAX_NAME_LENGTH = 100
const MAX_DESCRIPTION_LENGTH = 300

type ProjectDialogProps = {
  /** Omit to add a project. */
  project?: Project
  onClose: () => void
  onSaved: (project: Project) => void
  onDeleted?: () => void
}

/** Add or edit a project. Not in the mock: built from the standard form dialog (decision 6). */
export function ProjectDialog({ project, onClose, onSaved, onDeleted }: ProjectDialogProps) {
  const { t } = useTranslation()
  const clients = useQuery({ ...clientsQueryOptions, enabled: !project })
  const create = useCreateProject()
  const update = useUpdateProject(project?.id ?? '')
  const remove = useDeleteProject(project?.id ?? '')
  const [clientId, setClientId] = useState<string | null>(project?.client.id ?? null)
  const [name, setName] = useState(project?.name ?? '')
  const [description, setDescription] = useState(project?.description ?? '')
  const [isDefault, setIsDefault] = useState(project?.isDefault ?? false)
  const [active, setActive] = useState(project?.active ?? true)
  const [submitted, setSubmitted] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const lockedDefault = project?.isDefault ?? false
  const saving = project ? update : create

  const submit = () => {
    setSubmitted(true)
    if (!clientId || name.trim() === '') return
    const fields = { name: name.trim(), description: description.trim() || null, isDefault, active: active || isDefault }
    if (project) update.mutate(fields, { onSuccess: onSaved })
    else create.mutate({ clientId, ...fields }, { onSuccess: onSaved })
  }

  return (
    <>
      <FormDialog
        isOpen
        onOpenChange={(open) => !open && onClose()}
        title={project ? t('projects.dialog.editTitle', { name: project.name }) : t('projects.dialog.createTitle')}
        submitLabel={project ? t('projects.dialog.save') : t('projects.dialog.create')}
        onSubmit={submit}
        isPending={saving.isPending}
        error={<ErrorAlert error={saving.error} />}
        footerStart={
          project && (
            <Button
              variant="ghost"
              className="text-danger"
              icon={<Trash2 aria-hidden="true" className="size-(--icon-md)" />}
              onPress={() => setConfirmDelete(true)}
            >
              {t('projects.dialog.delete')}
            </Button>
          )
        }
      >
        {!project && (
          <Dropdown
            label={t('projects.dialog.client')}
            options={(clients.data ?? []).map((c) => ({ id: c.id, label: c.name }))}
            value={clientId}
            onChange={setClientId}
            placeholder={t('common.choose')}
            isRequired
            isInvalid={submitted && !clientId}
            errorMessage={t('projects.dialog.errors.client')}
          />
        )}
        <TextInput
          label={t('projects.dialog.name')}
          value={name}
          onChange={setName}
          maxLength={MAX_NAME_LENGTH}
          isRequired
          isInvalid={submitted && name.trim() === ''}
          errorMessage={t('projects.dialog.errors.name')}
        />
        <TextInput
          label={t('projects.dialog.description')}
          description={t('projects.dialog.descriptionHint')}
          value={description}
          onChange={setDescription}
          maxLength={MAX_DESCRIPTION_LENGTH}
        />
        <Checkbox
          isSelected={isDefault}
          isDisabled={lockedDefault}
          onChange={setIsDefault}
          description={lockedDefault ? t('projects.dialog.isDefaultLocked') : t('projects.dialog.isDefaultHint')}
        >
          {t('projects.dialog.isDefault')}
        </Checkbox>
        <Checkbox
          isSelected={active || isDefault}
          isDisabled={isDefault}
          onChange={setActive}
          description={t('projects.dialog.activeHint')}
        >
          {t('projects.dialog.active')}
        </Checkbox>
      </FormDialog>
      {project && (
        <ConfirmDialog
          isOpen={confirmDelete}
          onOpenChange={setConfirmDelete}
          tone="danger"
          title={t('projects.dialog.deleteTitle', { name: project.name })}
          confirmLabel={t('projects.dialog.delete')}
          isPending={remove.isPending}
          error={<ErrorAlert error={remove.error} />}
          onConfirm={() => remove.mutate(undefined, { onSuccess: () => onDeleted?.() })}
        >
          {t('projects.dialog.deleteBody')}
        </ConfirmDialog>
      )}
    </>
  )
}
