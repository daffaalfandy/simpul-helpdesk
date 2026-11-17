import { useSuspenseQuery } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { OrganizationInput } from '../../api/client'
import { Alert } from '../../components/Alert'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { FormDialog } from '../../components/Dialog'
import { OrgLogo } from '../../components/Logo'
import { SearchSelect } from '../../components/SearchSelect'
import { RowHeader, Table, Td, Th } from '../../components/Table'
import { TextInput } from '../../components/TextInput'
import { organizationsQueryOptions, useCreateOrganization } from './queries'
import { ErrorAlert, SectionHeader } from './shared'
import { DEFAULT_TIME_ZONE, useTimeZoneOptions } from './timeZones'

// Limits, matching the API (api/internal/organization).
const MAX_NAME_LENGTH = 200
const MAX_PREFIX_LENGTH = 10
const PREFIX_PATTERN = /^[A-Za-z][A-Za-z0-9]{0,9}$/

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
  const [draft, setDraft] = useState<OrganizationInput>({ name: '', ticketPrefix: '', timezone: DEFAULT_TIME_ZONE })
  const [tried, setTried] = useState(false)
  const zones = useTimeZoneOptions(draft.timezone)
  const nameInvalid = draft.name.trim() === ''
  const prefixInvalid = !PREFIX_PATTERN.test(draft.ticketPrefix)
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
        if (nameInvalid || prefixInvalid) return
        const body = { ...draft, name: draft.name.trim(), ticketPrefix: draft.ticketPrefix.toUpperCase() }
        create.mutate(body, { onSuccess: (o) => onCreated(o.name) })
      }}
    >
      <TextInput
        label={t('settings.general.name')}
        value={draft.name}
        maxLength={MAX_NAME_LENGTH}
        onChange={(name) => setDraft((d) => ({ ...d, name }))}
        isInvalid={tried && nameInvalid}
        errorMessage={t('settings.general.nameRequired')}
      />
      <TextInput
        label={t('settings.general.prefix')}
        value={draft.ticketPrefix}
        maxLength={MAX_PREFIX_LENGTH}
        onChange={(v) => setDraft((d) => ({ ...d, ticketPrefix: v.toUpperCase() }))}
        description={t('settings.organizations.prefixHint')}
        isInvalid={tried && prefixInvalid}
        errorMessage={t('settings.general.prefixInvalid')}
      />
      <SearchSelect
        label={t('settings.general.timezone')}
        options={zones}
        value={draft.timezone}
        onChange={(timezone) => timezone && setDraft((d) => ({ ...d, timezone }))}
        emptyText={t('settings.general.noZone')}
        description={t('settings.general.timezoneHint')}
      />
    </FormDialog>
  )
}
