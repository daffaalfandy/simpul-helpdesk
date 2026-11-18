import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useContext } from 'react'
import {
  Button,
  CalendarCell,
  CalendarGrid,
  CalendarGridBody,
  CalendarGridHeader,
  CalendarHeaderCell,
  CalendarMonthPicker,
  CalendarYearPicker,
  RangeCalendarStateContext,
} from 'react-aria-components'
import { useTranslation } from 'react-i18next'

import { cx } from '../lib/text'
import { Dropdown } from './Dropdown'

/*
 * The insides of a <Calendar> or <RangeCalendar>: a header to move between months and a grid of
 * days. React Aria keeps the state; these parts only read it, so both calendars share them.
 */

// On a phone the buttons draw at 36px, keeping a 44px touch area, so the month and year lists fit.
const navButton =
  'inline-flex size-(--control-h-md) shrink-0 items-center justify-center rounded-lg text-ink outline-none ' +
  'max-sm:hit-area max-sm:size-(--control-h-sm) ' +
  'data-hovered:bg-surface-subtle data-pressed:bg-tint data-focus-visible:shadow-(--focus-ring) data-disabled:text-disabled'

/** Previous and next month buttons around month and year lists, so a far-off date is two picks away. */
export function CalendarHeader() {
  const { t } = useTranslation()
  return (
    <div className="flex items-center gap-1">
      <Button slot="previous" aria-label={t('datePicker.previousMonth')} className={navButton}>
        <ChevronLeft aria-hidden="true" className="size-(--icon-lg)" />
      </Button>
      <div className="flex min-w-0 flex-1 gap-1">
        <CalendarMonthPicker>
          {({ items, value, onChange }) => (
            <Dropdown
              label={t('datePicker.month')}
              hideLabel
              className="min-w-0 flex-1"
              options={items.map((m) => ({ id: String(m.id), label: m.formatted }))}
              value={String(value)}
              onChange={(id) => onChange(Number(id))}
            />
          )}
        </CalendarMonthPicker>
        <CalendarYearPicker>
          {({ items, value, onChange }) => (
            <Dropdown
              label={t('datePicker.year')}
              hideLabel
              className="min-w-0 flex-1"
              options={items.map((y) => ({ id: String(y.id), label: y.formatted }))}
              value={String(value)}
              onChange={(id) => onChange(Number(id))}
            />
          )}
        </CalendarYearPicker>
      </div>
      <Button slot="next" aria-label={t('datePicker.nextMonth')} className={navButton}>
        <ChevronRight aria-hidden="true" className="size-(--icon-lg)" />
      </Button>
    </div>
  )
}

const dayCell =
  'flex h-(--control-h-md) w-full items-center justify-center text-base font-semibold tabular-nums text-ink outline-none ' +
  'transition-colors duration-(--duration-fast) ' +
  // Days of the months around this one stay blank, so each month reads as one block.
  'data-outside-month:invisible ' +
  'not-data-selected:data-hovered:bg-surface-subtle not-data-selected:data-pressed:bg-primary-tint-2 ' +
  'data-today:font-extrabold data-today:ring-1 data-today:ring-primary data-today:ring-inset ' +
  'data-focus-visible:relative data-focus-visible:z-10 data-focus-visible:shadow-(--focus-ring) ' +
  'data-disabled:cursor-not-allowed data-disabled:text-disabled'

/** One chosen day: a filled square, red when the field is invalid. */
const singleCell =
  'rounded-lg data-selected:bg-primary data-selected:text-on-primary data-selected:data-invalid:bg-danger'

/** A range: filled ends joined by a tinted band. */
const rangeCell =
  'not-data-selected:rounded-lg data-selected:bg-primary-tint data-selected:text-primary-hover ' +
  'data-selection-start:rounded-s-lg data-selection-start:bg-primary data-selection-start:text-on-primary ' +
  'data-selection-end:rounded-e-lg data-selection-end:bg-primary data-selection-end:text-on-primary'

/** The visible month: weekday names, then a 44px cell per day. */
export function CalendarMonthGrid() {
  const isRange = useContext(RangeCalendarStateContext) !== null
  return (
    <CalendarGrid weekdayStyle="short" className="w-full table-fixed border-collapse">
      <CalendarGridHeader>
        {(day) => <CalendarHeaderCell className="pb-1 text-xs font-bold text-muted">{day}</CalendarHeaderCell>}
      </CalendarGridHeader>
      <CalendarGridBody>
        {(date) => <CalendarCell date={date} className={cx(dayCell, isRange ? rangeCell : singleCell)} />}
      </CalendarGridBody>
    </CalendarGrid>
  )
}
