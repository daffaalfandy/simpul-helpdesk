import type { ReactNode } from 'react'
import { FieldError, Input, Label, NumberField, Text, type NumberFieldProps } from 'react-aria-components'

import { cx } from '../lib/text'
import { fieldBox, fieldDescription, fieldError, fieldLabel, fieldStack } from './fieldStyles'

type Common = { label: ReactNode; hideLabel?: boolean; errorMessage?: string; className?: string }

export function NumberInput({
  label,
  hideLabel,
  description,
  errorMessage,
  className,
  ...props
}: Common & { description?: ReactNode } & Omit<NumberFieldProps, 'children' | 'className'>) {
  return (
    <NumberField {...props} className={cx(fieldStack, className)}>
      <Label className={hideLabel ? 'sr-only' : fieldLabel('md')}>{label}</Label>
      <Input className={cx(fieldBox, 'tabular-nums')} />
      {description && (
        <Text slot="description" className={fieldDescription}>
          {description}
        </Text>
      )}
      <FieldError className={fieldError}>{errorMessage}</FieldError>
    </NumberField>
  )
}
