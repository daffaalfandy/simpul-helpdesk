import { useSuspenseQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { GitProvider, Project, Repository, RepositoryWithSecret } from '../../../api/client'
import { CopyField } from '../../../components/CopyField'
import { Checkbox, CheckboxGroup } from '../../../components/Checkbox'
import { ConfirmDialog, FormDialog } from '../../../components/Dialog'
import { TextInput } from '../../../components/TextInput'
import { useCodeLabels } from '../../code/labels'
import { projectsQueryOptions } from '../../projects/queries'
import { ErrorAlert } from '../shared'
import { useCreateRepository, useDeleteRepository, useRotateSecret, useUpdateRepository } from './queries'

export function AddRepositoryDialog({
  provider,
  onClose,
  onCreated,
}: {
  provider: GitProvider
  onClose: () => void
  onCreated: (result: RepositoryWithSecret) => void
}) {
  const { t } = useTranslation()
  const labels = useCodeLabels()
  const create = useCreateRepository()
  const [url, setUrl] = useState('')
  const [projectIds, setProjectIds] = useState<string[]>([])
  const [submitted, setSubmitted] = useState(false)
  const missing = url.trim() === ''

  const submit = () => {
    setSubmitted(true)
    if (missing) return
    create.mutate({ provider, url: url.trim(), projectIds }, { onSuccess: onCreated })
  }

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => !open && onClose()}
      title={t('settings.integrations.addTitle', { provider: labels.provider(provider) })}
      subtitle={t('settings.integrations.addSubtitle')}
      submitLabel={t('settings.integrations.add')}
      onSubmit={submit}
      isPending={create.isPending}
      error={<ErrorAlert error={create.error} />}
    >
      <TextInput
        label={t('settings.integrations.url')}
        description={t(`settings.integrations.urlHint.${provider}`)}
        placeholder={t(`settings.integrations.urlPlaceholder.${provider}`)}
        type="url"
        value={url}
        onChange={setUrl}
        isRequired
        isInvalid={submitted && missing}
        errorMessage={t('settings.integrations.urlRequired')}
      />
      <ProjectPicker value={projectIds} onChange={setProjectIds} />
    </FormDialog>
  )
}

export function EditProjectsDialog({ repo, onClose, onDone }: { repo: Repository; onClose: () => void; onDone: () => void }) {
  const { t } = useTranslation()
  const update = useUpdateRepository()
  const [projectIds, setProjectIds] = useState(repo.projects.map((p) => p.id))
  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => !open && onClose()}
      title={t('settings.integrations.projectsTitle', { name: repo.fullName })}
      submitLabel={t('common.save')}
      onSubmit={() => update.mutate({ id: repo.id, projectIds }, { onSuccess: onDone })}
      isPending={update.isPending}
      error={<ErrorAlert error={update.error} />}
    >
      <ProjectPicker value={projectIds} onChange={setProjectIds} repoId={repo.id} />
    </FormDialog>
  )
}

/** The projects that read code from the repository, grouped by client. */
function ProjectPicker({ value, onChange, repoId }: { value: string[]; onChange: (ids: string[]) => void; repoId?: string }) {
  const { t } = useTranslation()
  const { data: projects } = useSuspenseQuery(projectsQueryOptions)
  if (projects.length === 0) {
    return <p className="text-sm text-muted">{t('settings.integrations.pickerEmpty')}</p>
  }
  const elsewhere = (p: Project) => (p.repository && p.repository.id !== repoId ? p.repository.fullName : null)
  return (
    <CheckboxGroup
      label={t('settings.integrations.projectsLabel')}
      description={t('settings.integrations.pickerHint')}
      value={value}
      onChange={onChange}
    >
      {projects.map((p) => {
        const other = elsewhere(p)
        return (
          <Checkbox
            key={p.id}
            value={p.id}
            description={
              other
                ? t('settings.integrations.pickerMoves', { client: p.client.name, repo: other })
                : p.client.name
            }
          >
            {p.name}
          </Checkbox>
        )
      })}
    </CheckboxGroup>
  )
}

export function RotateSecretDialog({
  repo,
  onClose,
  onRotated,
}: {
  repo: Repository
  onClose: () => void
  onRotated: (result: RepositoryWithSecret) => void
}) {
  const { t } = useTranslation()
  const rotate = useRotateSecret()
  return (
    <ConfirmDialog
      isOpen
      onOpenChange={(open) => !open && onClose()}
      title={t('settings.integrations.rotateTitle', { name: repo.fullName })}
      confirmLabel={t('settings.integrations.rotateConfirm')}
      onConfirm={() => rotate.mutate(repo.id, { onSuccess: onRotated })}
      isPending={rotate.isPending}
      error={<ErrorAlert error={rotate.error} />}
    >
      {t('settings.integrations.rotateBody')}
    </ConfirmDialog>
  )
}

export function RemoveRepositoryDialog({ repo, onClose, onDone }: { repo: Repository; onClose: () => void; onDone: () => void }) {
  const { t } = useTranslation()
  const remove = useDeleteRepository()
  return (
    <ConfirmDialog
      isOpen
      tone="danger"
      onOpenChange={(open) => !open && onClose()}
      title={t('settings.integrations.removeTitle', { name: repo.fullName })}
      confirmLabel={t('settings.integrations.remove')}
      onConfirm={() => remove.mutate(repo.id, { onSuccess: onDone })}
      isPending={remove.isPending}
      error={<ErrorAlert error={remove.error} />}
    >
      {t('settings.integrations.removeBody')}
    </ConfirmDialog>
  )
}

/** Where to paste the webhook URL and secret on GitHub or GitLab. The secret shows only here. */
export function SetupDialog({ result, created, onClose }: { result: RepositoryWithSecret; created: boolean; onClose: () => void }) {
  const { t } = useTranslation()
  const { repository: repo, secret } = result
  const p = repo.provider
  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => !open && onClose()}
      title={t(created ? 'settings.integrations.setupTitle' : 'settings.integrations.setupSecretTitle', { name: repo.fullName })}
      subtitle={t(`settings.integrations.setupWhere.${p}`)}
      submitLabel={t('settings.integrations.setupDone')}
      onSubmit={onClose}
      hideCancel
    >
      <dl className="flex flex-col gap-4">
        <CopyField label={t(`settings.integrations.setupUrl.${p}`)} value={repo.webhookUrl} />
        {p === 'github' && <Field label={t('settings.integrations.setupContentType')} value={t('settings.integrations.contentTypeJson')} mono />}
        <CopyField label={t(`settings.integrations.setupSecret.${p}`)} value={secret} hint={t('settings.integrations.secretOnce')} />
        <Field label={t(`settings.integrations.setupEventsLabel.${p}`)} value={t(`settings.integrations.setupEvents.${p}`)} />
      </dl>
      <p className="text-sm text-muted">{t(`settings.integrations.setupReachable.${p}`)}</p>
    </FormDialog>
  )
}

function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex flex-col gap-1">
      <dt className="text-sm font-bold text-ink">{label}</dt>
      <dd className={mono ? 'font-mono text-sm' : 'text-sm text-ink-2'}>{value}</dd>
    </div>
  )
}
