import type { ReactNode } from 'react'
import { FieldError, Label, TextArea as AriaTextArea, TextField, type TextFieldProps } from 'react-aria-components'

import { cx } from '../lib/text'

type TextAreaProps = Omit<TextFieldProps, 'children' | 'className'> & {
  label: ReactNode
  errorMessage?: string
  className?: string
}

/** Multi-line text input with a label above and an error below. */
export function TextArea({ label, errorMessage, className, ...props }: TextAreaProps) {
  return (
    <TextField {...props} className={cx('flex flex-col gap-1.5', className)}>
      <Label className="text-sm font-bold text-ink">{label}</Label>
      <AriaTextArea className="min-h-24 w-full rounded-lg border border-border-strong bg-surface px-3.5 py-2.5 font-sans text-base text-ink data-invalid:border-danger" />
      <FieldError className="text-xs font-semibold text-danger">{errorMessage}</FieldError>
    </TextField>
  )
}
