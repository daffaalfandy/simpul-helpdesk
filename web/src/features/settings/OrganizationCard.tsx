import { useSuspenseQuery } from '@tanstack/react-query'
import { Upload } from 'lucide-react'
import { useId, useMemo, useState } from 'react'
import { FileTrigger } from 'react-aria-components'
import { useTranslation } from 'react-i18next'

import type { Organization, OrganizationInput } from '../../api/client'
import { organizationQueryOptions } from '../../api/queries'
import { Alert } from '../../components/Alert'
import { Button } from '../../components/Button'
import { SearchSelect } from '../../components/SearchSelect'
import { TextInput } from '../../components/TextInput'
import { UnsavedBar } from '../../components/UnsavedBar'
import { formatFileSize } from '../../lib/files'
import { useRemoveLogo, useUpdateOrganization, useUploadLogo } from './queries'
import { ErrorAlert } from './shared'

// Limits, matching the API (api/internal/organization).
const MAX_NAME_LENGTH = 200
const MAX_PREFIX_LENGTH = 10
const PREFIX_PATTERN = /^[A-Za-z][A-Za-z0-9]{0,9}$/
export const MAX_LOGO_BYTES = 512 * 1024
const LOGO_TYPES = ['image/png', 'image/jpeg', 'image/webp']

/** Indonesia's three zones first (Settings mock), each with its local name; then every zone. */
const INDONESIAN_ZONES = [
  { id: 'Asia/Jakarta', key: 'wib' },
  { id: 'Asia/Makassar', key: 'wita' },
  { id: 'Asia/Jayapura', key: 'wit' },
] as const

function draftOf(org: Organization): OrganizationInput {
  return { name: org.name, ticketPrefix: org.ticketPrefix, timezone: org.timezone }
}

/**
 * The organisation (Settings mock's General card): logo, name, App URL (read-only, set on the
 * server), ticket prefix and time zone. Fields stay editable and save from the unsaved bar, like
 * Priorities; the logo saves on its own.
 */
export function OrganizationCard({ onFlash }: { onFlash: (message: string | null) => void }) {
  const { t } = useTranslation()
  const { data: org } = useSuspenseQuery(organizationQueryOptions)
  const [draft, setDraft] = useState(() => draftOf(org))
  const [showErrors, setShowErrors] = useState(false)
  const save = useUpdateOrganization()
  const titleId = useId()

  const zones = useMemo(() => {
    const known = new Set<string>(INDONESIAN_ZONES.map((z) => z.id))
    const rest = Intl.supportedValuesOf('timeZone').filter((z) => !known.has(z))
    if (!known.has(org.timezone) && !rest.includes(org.timezone)) rest.unshift(org.timezone)
    return [
      ...INDONESIAN_ZONES.map((z) => ({ id: z.id, label: t(`settings.general.zones.${z.key}`), searchText: z.id })),
      ...rest.map((z) => ({ id: z, label: z })),
    ]
  }, [org.timezone, t])

  const nameInvalid = draft.name.trim() === ''
  const prefixInvalid = !PREFIX_PATTERN.test(draft.ticketPrefix)
  const dirty = draft.name !== org.name || draft.ticketPrefix !== org.ticketPrefix || draft.timezone !== org.timezone
  const edit = (next: Partial<OrganizationInput>) => {
    onFlash(null)
    setDraft((d) => ({ ...d, ...next }))
  }
  const discard = () => {
    setDraft(draftOf(org))
    setShowErrors(false)
    save.reset()
  }
  const submit = () => {
    setShowErrors(true)
    if (nameInvalid || prefixInvalid) return
    save.mutate(
      { ...draft, name: draft.name.trim(), ticketPrefix: draft.ticketPrefix.toUpperCase() },
      {
        onSuccess: (saved) => {
          setDraft(draftOf(saved))
          setShowErrors(false)
          onFlash(t('settings.general.saved'))
        },
      },
    )
  }

  return (
    <section
      aria-labelledby={titleId}
      className="flex min-w-0 flex-[999_1_560px] flex-col gap-4 rounded-3xl border border-border bg-surface p-5 shadow-card"
    >
      <h3 id={titleId} className="text-lg font-extrabold tracking-title">
        {t('settings.general.title')}
      </h3>
      <LogoField org={org} onFlash={onFlash} />
      <div className="grid grid-cols-[repeat(auto-fit,minmax(var(--org-field-min-w),1fr))] gap-3.5">
        <TextInput
          label={t('settings.general.name')}
          value={draft.name}
          maxLength={MAX_NAME_LENGTH}
          onChange={(name) => edit({ name })}
          isInvalid={showErrors && nameInvalid}
          errorMessage={t('settings.general.nameRequired')}
        />
        <TextInput label={t('settings.general.appUrl')} value={org.appUrl} isReadOnly description={t('settings.general.appUrlHint')} />
        <TextInput
          label={t('settings.general.prefix')}
          value={draft.ticketPrefix}
          maxLength={MAX_PREFIX_LENGTH}
          onChange={(v) => edit({ ticketPrefix: v.toUpperCase() })}
          description={t('settings.general.prefixHint')}
          isInvalid={showErrors && prefixInvalid}
          errorMessage={t('settings.general.prefixInvalid')}
        />
        <SearchSelect
          label={t('settings.general.timezone')}
          options={zones}
          value={draft.timezone}
          onChange={(timezone) => timezone && edit({ timezone })}
          emptyText={t('settings.general.noZone')}
          description={t('settings.general.timezoneHint')}
        />
      </div>
      <ErrorAlert error={save.error} />
      {dirty && <UnsavedBar message={t('settings.general.unsaved')} onDiscard={discard} onSave={submit} isPending={save.isPending} />}
    </section>
  )
}

/** The logo (mock: 64px box, "Upload logo"). It saves as soon as a file is chosen. */
function LogoField({ org, onFlash }: { org: Organization; onFlash: (message: string | null) => void }) {
  const { t } = useTranslation()
  const upload = useUploadLogo()
  const remove = useRemoveLogo()
  const [tooLarge, setTooLarge] = useState(false)

  const choose = (files: FileList | null) => {
    const file = files?.[0]
    if (!file) return
    onFlash(null)
    remove.reset()
    setTooLarge(file.size > MAX_LOGO_BYTES)
    if (file.size > MAX_LOGO_BYTES) return
    upload.mutate(file, { onSuccess: () => onFlash(t('settings.general.logoSaved')) })
  }

  return (
    <div className="flex flex-wrap items-center gap-4">
      {org.logoUrl ? (
        <img
          src={org.logoUrl}
          alt={t('settings.general.logoAlt', { name: org.name })}
          width={64}
          height={64}
          className="size-16 rounded-2xl border border-border object-contain"
        />
      ) : (
        <span className="flex size-16 items-center justify-center rounded-2xl border border-dashed border-border-strong bg-tint text-center text-2xs font-bold text-muted">
          {t('settings.general.noLogo')}
        </span>
      )}
      <div className="flex min-w-0 flex-col gap-2">
        <div className="flex flex-wrap gap-2">
          <FileTrigger acceptedFileTypes={LOGO_TYPES} onSelect={choose}>
            <Button variant="outline" icon={<Upload aria-hidden="true" className="size-(--icon-md)" />} isPending={upload.isPending}>
              {org.logoUrl ? t('settings.general.replaceLogo') : t('settings.general.uploadLogo')}
            </Button>
          </FileTrigger>
          {org.logoUrl && (
            <Button
              variant="ghost"
              isPending={remove.isPending}
              onPress={() => {
                onFlash(null)
                upload.reset()
                remove.mutate(undefined, { onSuccess: () => onFlash(t('settings.general.logoRemoved')) })
              }}
            >
              {t('settings.general.removeLogo')}
            </Button>
          )}
        </div>
        <span className="text-xs text-muted">{t('settings.general.logoHint', { size: formatFileSize(MAX_LOGO_BYTES) })}</span>
        {tooLarge && <Alert tone="error">{t('errors.logo_too_large')}</Alert>}
        <ErrorAlert error={upload.error ?? remove.error} />
      </div>
    </div>
  )
}
