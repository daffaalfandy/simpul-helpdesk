import { useSuspenseQuery } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert } from '../../components/Alert'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { FormDialog } from '../../components/Dialog'
import { OrgLogo } from '../../components/Logo'
import { RowHeader, Table, Td, Th } from '../../components/Table'
import { organizationsQueryOptions, useCreateOrganization } from './queries'
import { ErrorAlert, SectionHeader } from './shared'
import { cleanOrganizationDraft, isOrganizationDraftValid, newOrganizationDraft, OrganizationFields } from './OrganizationFields'

/**
 * Settings › Organizations (administrators): the install's vendor organizations, each with its
 * own clients, projects, tickets and settings (owner, 17 Nov). There's no mock: it reuses the
 * Clients page's card and table.
 */
export function OrganizationsPage() {
  const { t } = useTranslation()
  const { data: organizations } = useSuspenseQuery(organizationsQueryOptions)
  const [adding, setAdding] = useState(false)
  const [flash, setFlash] = useState<string | null>(null)
  const addButton = (
    <Button
      icon={<Plus aria-hidden="true" className="size-(--icon-md)" />}
      onPress={() => {
        setFlash(null)
        setAdding(true)
      }}
    >
      {t('settings.organizations.add')}
    </Button>
  )
  return (
    <>
      <SectionHeader section="organizations" />
      {flash && <Alert tone="success">{flash}</Alert>}
      <Card as="h3" title={t('settings.organizations.cardTitle')} actions={addButton}>
        <Table caption={t('settings.organizations.cardTitle')}>
          <thead>
            <tr>
              <Th>{t('settings.organizations.cols.name')}</Th>
              <Th>{t('settings.organizations.cols.prefix')}</Th>
              <Th>{t('settings.organizations.cols.timezone')}</Th>
              <Th>{t('settings.organizations.cols.members')}</Th>
              <Th>{t('settings.organizations.cols.clients')}</Th>
            </tr>
          </thead>
          <tbody>
            {organizations.map((o) => (
              <tr key={o.id}>
                <RowHeader>
                  <span className="flex items-center gap-2.5">
                    <OrgLogo org={o} size="sm" />
                    {o.name}
                  </span>
                </RowHeader>
                <Td className="tabular-nums">{o.ticketPrefix}</Td>
                <Td>{o.timezone}</Td>
                <Td className="tabular-nums">{o.memberCount}</Td>
                <Td className="tabular-nums">{o.clientCount}</Td>
              </tr>
            ))}
          </tbody>
        </Table>
      </Card>
      {adding && (
        <CreateOrganizationDialog
          onClose={() => setAdding(false)}
          onCreated={(name) => {
            setAdding(false)
            setFlash(t('settings.organizations.created', { name }))
          }}
        />
      )}
    </>
  )
}

function CreateOrganizationDialog({ onClose, onCreated }: { onClose: () => void; onCreated: (name: string) => void }) {
  const { t } = useTranslation()
  const create = useCreateOrganization()
  const [draft, setDraft] = useState(newOrganizationDraft)
  const [tried, setTried] = useState(false)
  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => !open && onClose()}
      title={t('settings.organizations.dialogTitle')}
      subtitle={t('settings.organizations.dialogSubtitle')}
      submitLabel={t('settings.organizations.add')}
      isPending={create.isPending}
      error={<ErrorAlert error={create.error} />}
      onSubmit={() => {
        setTried(true)
        if (!isOrganizationDraftValid(draft)) return
        create.mutate(cleanOrganizationDraft(draft), { onSuccess: (o) => onCreated(o.name) })
      }}
    >
      <OrganizationFields draft={draft} onChange={setDraft} showErrors={tried} />
    </FormDialog>
  )
}
