import { Pencil } from 'lucide-react'
import { useEffect, useId, useRef, type FormEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from './Button'

const FOCUSABLE = 'input, button, [role="button"], textarea, [tabindex]:not([tabindex="-1"])'

type EditableCardProps = {
  title: ReactNode
  description?: ReactNode
  /** Header button that switches to edit mode, e.g. "Edit SLA". */
  editLabel: ReactNode
  /** Save button in edit mode, e.g. "Save SLA". */
  saveLabel: ReactNode
  /** Users who may only read never see the Edit button. */
  canEdit: boolean
  isEditing: boolean
  onEdit: () => void
  onCancel: () => void
  onSave: () => void
  isSaveDisabled?: boolean
  isPending?: boolean
  /** The read-only body, or the form fields while editing. */
  children: ReactNode
}

/**
 * Card that reads compactly and turns into a form on "Edit" (HANDOFF §14.3, first used by the
 * project SLA card). The parent owns the draft; give the card a `key` per record so switching
 * records discards unsaved changes.
 */
export function EditableCard({
  title,
  description,
  editLabel,
  saveLabel,
  canEdit,
  isEditing,
  onEdit,
  onCancel,
  onSave,
  isSaveDisabled,
  isPending,
  children,
}: EditableCardProps) {
  const { t } = useTranslation()
  const titleId = useId()
  const formId = useId()
  const bodyRef = useRef<HTMLDivElement>(null)
  const editRef = useRef<HTMLButtonElement>(null)
  const wasEditing = useRef(isEditing)

  // The pressed button disappears when the mode changes, so move focus somewhere useful:
  // the first field when editing starts, back to "Edit" when it ends.
  useEffect(() => {
    if (isEditing && !wasEditing.current) bodyRef.current?.querySelector<HTMLElement>(FOCUSABLE)?.focus()
    if (!isEditing && wasEditing.current) editRef.current?.focus()
    wasEditing.current = isEditing
  }, [isEditing])

  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (!isSaveDisabled) onSave()
  }

  return (
    <section
      aria-labelledby={titleId}
      className="flex flex-col gap-3.5 rounded-3xl border border-border bg-surface px-5.5 py-5 shadow-card"
    >
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex max-w-3xl min-w-0 flex-1 flex-col gap-1">
          <h2 id={titleId} className="text-lg font-extrabold">
            {title}
          </h2>
          {description && <p className="text-sm text-muted">{description}</p>}
        </div>
        {canEdit && (
          <div className="flex shrink-0 flex-wrap gap-2">
            {isEditing ? (
              <>
                <Button variant="outline" onPress={onCancel} isDisabled={isPending}>
                  {t('common.cancel')}
                </Button>
                <Button type="submit" form={formId} isPending={isPending} isDisabled={isSaveDisabled}>
                  {saveLabel}
                </Button>
              </>
            ) : (
              <Button ref={editRef} variant="outline" onPress={onEdit} icon={<Pencil aria-hidden="true" className="size-(--icon-md)" />}>
                {editLabel}
              </Button>
            )}
          </div>
        )}
      </div>
      <div key={isEditing ? 'edit' : 'read'} ref={bodyRef} className="animate-fade-in">
        {isEditing ? (
          <form id={formId} onSubmit={submit} noValidate className="flex flex-col gap-3.5">
            {children}
          </form>
        ) : (
          <div className="flex flex-col gap-3.5">{children}</div>
        )}
      </div>
    </section>
  )
}
