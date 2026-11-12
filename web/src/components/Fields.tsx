import type { ReactNode } from 'react'
import {
  DateField,
  DateInput as AriaDateInput,
  DateSegment,
  FieldError,
  Group,
  Input,
  Label,
  NumberField,
  TimeField,
  type DateFieldProps,
  type DateValue,
  type NumberFieldProps,
  type TimeFieldProps,
  type TimeValue,
} from 'react-aria-components'

import { cx } from '../lib/text'

const field = 'flex flex-col gap-1.5'
const labelClass = 'text-sm font-bold text-ink'
const box =
  'flex min-h-(--control-h-md) w-full items-center rounded-lg border border-border-strong bg-surface px-3.5 ' +
  'font-sans text-base text-ink data-invalid:border-danger data-focus-within:border-primary'
const segment =
  'rounded-xs px-0.5 tabular-nums outline-none data-placeholder:text-muted data-focused:bg-primary data-focused:text-on-primary'
const errorClass = 'text-xs font-semibold text-danger'

type Common = { label: ReactNode; hideLabel?: boolean; errorMessage?: string; className?: string }

export function NumberInput({
  label,
  hideLabel,
  errorMessage,
  className,
  ...props
}: Common & Omit<NumberFieldProps, 'children' | 'className'>) {
  return (
    <NumberField {...props} className={cx(field, className)}>
      <Label className={hideLabel ? 'sr-only' : labelClass}>{label}</Label>
      <Input className={cx(box, 'tabular-nums')} />
      <FieldError className={errorClass}>{errorMessage}</FieldError>
    </NumberField>
  )
}

/** Date input with typed segments. The order follows the app's date locale. */
export function DateInput<T extends DateValue>({
  label,
  hideLabel,
  errorMessage,
  className,
  ...props
}: Common & Omit<DateFieldProps<T>, 'children' | 'className'>) {
  return (
    <DateField {...props} className={cx(field, className)}>
      <Label className={hideLabel ? 'sr-only' : labelClass}>{label}</Label>
      <Group className={box}>
        <AriaDateInput className="flex">{(s) => <DateSegment segment={s} className={segment} />}</AriaDateInput>
      </Group>
      <FieldError className={errorClass}>{errorMessage}</FieldError>
    </DateField>
  )
}

/** 24-hour time input. */
export function TimeInput<T extends TimeValue>({
  label,
  hideLabel,
  errorMessage,
  className,
  ...props
}: Common & Omit<TimeFieldProps<T>, 'children' | 'className' | 'hourCycle'>) {
  return (
    <TimeField {...props} hourCycle={24} className={cx(field, className)}>
      <Label className={hideLabel ? 'sr-only' : labelClass}>{label}</Label>
      <Group className={box}>
        <AriaDateInput className="flex">{(s) => <DateSegment segment={s} className={segment} />}</AriaDateInput>
      </Group>
      <FieldError className={errorClass}>{errorMessage}</FieldError>
    </TimeField>
  )
}
