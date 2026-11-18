import { CalendarDays } from 'lucide-react'
import type { ReactNode } from 'react'
import {
  DatePicker as AriaDatePicker,
  Button,
  Calendar,
  DateInput,
  DateSegment,
  Dialog,
  FieldError,
  Group,
  Label,
  Popover,
  Text,
  type DatePickerProps as AriaDatePickerProps,
  type DateValue,
} from 'react-aria-components'
import { useTranslation } from 'react-i18next'

import { POPOVER_OFFSET_PX } from '../lib/constants'
import { cx } from '../lib/text'
import { CalendarHeader, CalendarMonthGrid } from './CalendarParts'
import { dateSegment, fieldBox, fieldDescription, fieldError, fieldLabel, fieldStack } from './fieldStyles'

type DatePickerProps<T extends DateValue> = Omit<
  AriaDatePickerProps<T>,
  // A min or max brings React Aria's own error text, which isn't translated; check ranges in the form.
  'children' | 'className' | 'minValue' | 'maxValue'
> & {
  /** Also names the calendar for screen readers, so it is text. */
  label: string
  hideLabel?: boolean
  description?: ReactNode
  errorMessage?: string
  className?: string
}

/**
 * Date field: type the day, month and year (in the en-GB order the app's I18nProvider sets), or
 * open the calendar beside them. Picking a day closes the calendar and focus returns to its button;
 * Esc closes it without a change.
 */
export function DatePicker<T extends DateValue>({
  label,
  hideLabel,
  description,
  errorMessage,
  className,
  ...props
}: DatePickerProps<T>) {
  const { t } = useTranslation()
  return (
    <AriaDatePicker {...props} className={cx('group/date', fieldStack, className)}>
      <Label className={hideLabel ? 'sr-only' : fieldLabel('md')}>{label}</Label>
      <Group className={cx(fieldBox, 'gap-2 group-data-[open]/date:border-primary')}>
        <DateInput className="flex min-w-0 flex-1">{(s) => <DateSegment segment={s} className={dateSegment} />}</DateInput>
        <Button
          aria-label={t('datePicker.open')}
          className={cx(
            'hit-area -me-2 inline-flex size-(--control-h-sm) shrink-0 items-center justify-center rounded-md text-muted outline-none',
            'data-hovered:bg-surface-subtle data-hovered:text-ink data-pressed:bg-tint data-pressed:text-ink',
            'data-focus-visible:shadow-(--focus-ring) data-disabled:text-disabled',
          )}
        >
          <CalendarDays aria-hidden="true" className="size-(--icon-lg)" />
        </Button>
      </Group>
      {description && (
        <Text slot="description" className={fieldDescription}>
          {description}
        </Text>
      )}
      <FieldError className={fieldError}>{errorMessage}</FieldError>
      <Popover
        placement="bottom start"
        offset={POPOVER_OFFSET_PX}
        className={cx(
          'w-(--calendar-w) max-w-[calc(100vw-2*var(--space-4))] rounded-2xl border border-border-strong bg-surface',
          'shadow-popover outline-none data-entering:animate-enter',
        )}
      >
        <Dialog className="p-4 outline-none max-sm:p-2">
          <Calendar aria-label={label} className="flex flex-col gap-3">
            <CalendarHeader />
            <CalendarMonthGrid />
          </Calendar>
        </Dialog>
      </Popover>
    </AriaDatePicker>
  )
}
