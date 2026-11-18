import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { Client, Requester } from '../../api/client'
import { Checkbox, CheckboxGroup } from '../../components/Checkbox'
import { ConfirmDialog, FormDialog } from '../../components/Dialog'
import { useSetRequesterProjects } from './queries'
import { ErrorAlert } from './shared'

type RequesterProjectsDialogProps = {
  client: Client
  requester: Requester
  onClose: () => void
  /** Called with the confirmation to show once the change is saved. */
  onSaved: (message: string) => void
}

/**
 * Which of the client's projects a requester reports on and sees tickets of (owner, 18 Nov).
 * Taking away their last project asks first: they would keep only the tickets they reported.
 */
export function RequesterProjectsDialog({ client, requester, onClose, onSaved }: RequesterProjectsDialogProps) {
  const { t } = useTranslation()
  const save = useSetRequesterProjects(client.id)
  const [projectIds, setProjectIds] = useState<string[]>(requester.projectIds)
  const [confirming, setConfirming] = useState(false)

  const persist = () =>
    save.mutate(
      { userId: requester.id, projectIds },
      { onSuccess: () => onSaved(t('settings.clients.requesters.projectsSaved', { name: requester.name })) },
    )
  const submit = () => {
    if (projectIds.length === 0 && requester.projectIds.length > 0) {
      setConfirming(true)
      return
    }
    persist()
  }

  return (
    <>
      <FormDialog
        isOpen
        onOpenChange={(open) => !open && onClose()}
        title={t('settings.clients.requesters.projectsTitle', { name: requester.name })}
        subtitle={t('settings.clients.requesters.projectsSubtitle', { name: requester.name })}
        submitLabel={t('settings.clients.requesters.projectsSave')}
        isPending={save.isPending && !confirming}
        onSubmit={submit}
        error={<ErrorAlert error={confirming ? null : save.error} />}
      >
        <CheckboxGroup
          label={t('settings.clients.requesters.projects')}
          description={t('settings.clients.requesters.projectsHint')}
          value={projectIds}
          onChange={setProjectIds}
        >
          {client.projects.map((p) => (
            <Checkbox key={p.id} value={p.id}>
              {p.name}
            </Checkbox>
          ))}
        </CheckboxGroup>
      </FormDialog>
      {/* Outside the form: a portal's submit would bubble to the projects form through React. */}
      {confirming && (
        <ConfirmDialog
          isOpen
          onOpenChange={(open) => {
            if (open) return
            setConfirming(false)
            save.reset()
          }}
          tone="danger"
          title={t('settings.clients.requesters.removeAllTitle', { name: requester.name })}
          confirmLabel={t('settings.clients.requesters.removeAllConfirm')}
          isPending={save.isPending}
          error={<ErrorAlert error={save.error} />}
          onConfirm={persist}
        >
          {t('settings.clients.requesters.removeAllBody')}
        </ConfirmDialog>
      )}
    </>
  )
}
