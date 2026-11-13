import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from './Button'

type UnsavedBarProps = {
  /** What is unsaved, e.g. "You have unsaved changes to priority names." */
  message: ReactNode
  onDiscard: () => void
  onSave: () => void
  saveLabel?: ReactNode
  isPending?: boolean
  isSaveDisabled?: boolean
}

/** Navy bar shown below an edited form until it is saved or discarded (Settings › Priorities). */
export function UnsavedBar({ message, onDiscard, onSave, saveLabel, isPending, isSaveDisabled }: UnsavedBarProps) {
  const { t } = useTranslation()
  return (
    <div
      role="region"
      aria-label={t('common.unsavedChanges')}
      className="flex animate-enter flex-wrap items-center justify-between gap-3 rounded-2xl bg-ink px-4.5 py-3.5 text-on-primary"
    >
      <p className="text-base font-bold">{message}</p>
      <div className="flex flex-wrap gap-3">
        <Button variant="outlineOnDark" onPress={onDiscard} isDisabled={isPending}>
          {t('common.discard')}
        </Button>
        <Button onPress={onSave} isPending={isPending} isDisabled={isSaveDisabled}>
          {saveLabel ?? t('common.save')}
        </Button>
      </div>
    </div>
  )
}
