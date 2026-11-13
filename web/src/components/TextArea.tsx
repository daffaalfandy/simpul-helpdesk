import type { ReactNode } from 'react'
import { FieldError, Label, Text, TextArea as AriaTextArea, TextField, type TextFieldProps } from 'react-aria-components'

import { cx } from '../lib/text'

type TextAreaProps = Omit<TextFieldProps, 'children' | 'className'> & {
  label: ReactNode
  hideLabel?: boolean
  description?: ReactNode
  placeholder?: string
  rows?: number
  size?: 'md' | 'lg'
  /** "internal" turns the field yellow, like an internal note (HANDOFF §14.3 Tabs). */
  tone?: 'default' | 'internal'
  errorMessage?: string
  className?: string
}

const DEFAULT_ROWS = 4

/** Multi-line text input with a label above and an error below. */
export function TextArea({
  label,
  hideLabel,
  description,
  placeholder,
  rows = DEFAULT_ROWS,
  size = 'md',
  tone = 'default',
  errorMessage,
  className,
  ...props
}: TextAreaProps) {
  return (
    <TextField {...props} className={cx('flex flex-col gap-1.5', className)}>
      <Label className={hideLabel ? 'sr-only' : 'text-sm font-bold text-ink'}>{label}</Label>
      <AriaTextArea
        rows={rows}
        placeholder={placeholder}
        className={cx(
          'w-full rounded-lg border px-3.5 py-2.5 font-sans leading-relaxed text-ink placeholder:text-muted data-invalid:border-danger',
          tone === 'internal' ? 'border-note-internal-border bg-note-internal-bg' : 'border-border-strong bg-surface',
          size === 'lg' ? 'text-md' : 'text-base',
        )}
      />
      {description && (
        <Text slot="description" className="text-xs text-muted">
          {description}
        </Text>
      )}
      <FieldError className="text-xs font-semibold text-danger">{errorMessage}</FieldError>
    </TextField>
  )
}
