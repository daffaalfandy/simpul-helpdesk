import { useTranslation } from 'react-i18next'

import type { OrganizationInput } from '../../api/client'
import { SearchSelect } from '../../components/SearchSelect'
import { TextInput } from '../../components/TextInput'
import { DEFAULT_TIME_ZONE, useTimeZoneOptions } from './timeZones'

// Limits, matching the API (api/internal/organization).
const MAX_NAME_LENGTH = 200
const MAX_PREFIX_LENGTH = 10
const PREFIX_PATTERN = /^[A-Za-z][A-Za-z0-9]{0,9}$/

export function newOrganizationDraft(): OrganizationInput {
  return { name: '', ticketPrefix: '', timezone: DEFAULT_TIME_ZONE }
}

function problems(draft: OrganizationInput) {
  return { name: draft.name.trim() === '', prefix: !PREFIX_PATTERN.test(draft.ticketPrefix) }
}

export function isOrganizationDraftValid(draft: OrganizationInput): boolean {
  const p = problems(draft)
  return !p.name && !p.prefix
}

/** The draft as the API takes it. */
export function cleanOrganizationDraft(draft: OrganizationInput): OrganizationInput {
  return { ...draft, name: draft.name.trim(), ticketPrefix: draft.ticketPrefix.toUpperCase() }
}

type OrganizationFieldsProps = {
  draft: OrganizationInput
  onChange: (next: OrganizationInput) => void
  /** Show what's wrong once the user tried to submit. */
  showErrors: boolean
}

/** A new organization's name, ticket prefix and time zone (Settings › Organizations, first run). */
export function OrganizationFields({ draft, onChange, showErrors }: OrganizationFieldsProps) {
  const { t } = useTranslation()
  const zones = useTimeZoneOptions(draft.timezone)
  const wrong = problems(draft)
  return (
    <>
      <TextInput
        label={t('settings.general.name')}
        value={draft.name}
        maxLength={MAX_NAME_LENGTH}
        autoComplete="organization"
        onChange={(name) => onChange({ ...draft, name })}
        isInvalid={showErrors && wrong.name}
        errorMessage={t('settings.general.nameRequired')}
      />
      <TextInput
        label={t('settings.general.prefix')}
        value={draft.ticketPrefix}
        maxLength={MAX_PREFIX_LENGTH}
        onChange={(v) => onChange({ ...draft, ticketPrefix: v.toUpperCase() })}
        description={t('settings.organizations.prefixHint')}
        isInvalid={showErrors && wrong.prefix}
        errorMessage={t('settings.general.prefixInvalid')}
      />
      <SearchSelect
        label={t('settings.general.timezone')}
        options={zones}
        value={draft.timezone}
        onChange={(timezone) => timezone && onChange({ ...draft, timezone })}
        emptyText={t('settings.general.noZone')}
        description={t('settings.general.timezoneHint')}
      />
    </>
  )
}
