import type { ReactNode } from 'react'
import {
  FieldError,
  Input,
  Label,
  Text,
  TextField,
  type TextFieldProps,
  type ValidationResult,
} from 'react-aria-components'

import { cx } from '../lib/text'

type TextInputProps = Omit<TextFieldProps, 'children' | 'className'> & {
  label: ReactNode
  /** Extra content on the label row, e.g. a "Forgot password?" link. */
  labelAction?: ReactNode
  description?: ReactNode
  placeholder?: string
  size?: 'md' | 'lg'
  errorMessage?: string | ((validation: ValidationResult) => string)
  className?: string
}

/** Text input per HANDOFF §14.3: label above, error below, danger border when invalid. */
export function TextInput({
  label,
  labelAction,
  description,
  placeholder,
  size = 'md',
  errorMessage,
  className,
  ...props
}: TextInputProps) {
  return (
    <TextField {...props} className={cx('flex flex-col gap-1.5', className)}>
      <div className="flex items-baseline justify-between gap-3">
        <Label className="text-sm font-bold text-ink">{label}</Label>
        {labelAction}
      </div>
      <Input
        placeholder={placeholder}
        className={cx(
          'w-full rounded-lg border border-border-strong bg-surface px-3.5 font-sans text-ink',
          'placeholder:text-muted data-invalid:border-danger',
          size === 'lg' ? 'min-h-(--control-h-lg) text-md' : 'min-h-(--control-h-md) text-base',
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
