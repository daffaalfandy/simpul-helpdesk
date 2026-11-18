import { parseDate } from '@internationalized/date'
import { CalendarDays, ChevronDown } from 'lucide-react'
import { useId, useState } from 'react'
import { Button as AriaButton, Dialog, DialogTrigger, Heading, Popover, RangeCalendar } from 'react-aria-components'
import { useTranslation } from 'react-i18next'

import { POPOVER_OFFSET_PX } from '../lib/constants'
import { currentMonthRange, isSameRange, lastMonthRange, type DateRange } from '../lib/dateRange'
import { formatDateRange } from '../lib/dates'
import { cx } from '../lib/text'
import { Button } from './Button'
import { CalendarHeader, CalendarMonthGrid } from './CalendarParts'
import { pillClasses } from './FilterPills'

const PRESETS = ['thisMonth', 'lastMonth'] as const

type DateRangeFilterProps = {
  value: DateRange
  /** The organization's time zone, where "this month" starts. */
  timeZone: string
  /** What the range limits on this page. */
  description: string
  onChange: (range: DateRange) => void
}

/**
 * The days tickets were reported on (owner, 18 Nov): a filter pill naming the range, opening a
 * calendar where the second day picked, or a preset, saves the range and closes it. Esc closes
 * without a change and focus returns to the pill. The pill looks on while the range isn't this
 * month, the default.
 */
export function DateRangeFilter({ value, timeZone, description, onChange }: DateRangeFilterProps) {
  const { t } = useTranslation()
  const [isOpen, setOpen] = useState(false)
  const descriptionId = useId()
  const thisMonth = currentMonthRange(timeZone)
  const presets: Record<(typeof PRESETS)[number], DateRange> = { thisMonth, lastMonth: lastMonthRange(timeZone) }
  const pick = (range: DateRange) => {
    onChange(range)
    setOpen(false)
  }

  return (
    <DialogTrigger isOpen={isOpen} onOpenChange={setOpen}>
      <AriaButton className={pillClasses(!isSameRange(value, thisMonth))}>
        <CalendarDays aria-hidden="true" className="size-(--icon-md) shrink-0" />
        {t('dateRange.trigger', { range: formatDateRange(value.from, value.to) })}
        <ChevronDown aria-hidden="true" className="size-(--icon-md) shrink-0" />
      </AriaButton>
      <Popover
        placement="bottom start"
        offset={POPOVER_OFFSET_PX}
        // On a short screen React Aria caps the height; the panel then scrolls instead of spilling.
        className={cx(
          'w-(--calendar-w) max-w-[calc(100vw-2*var(--space-4))] overflow-y-auto rounded-2xl border border-border-strong bg-surface',
          'shadow-popover outline-none data-entering:animate-enter',
        )}
      >
        <Dialog aria-describedby={descriptionId} className="flex flex-col gap-3 p-4 outline-none max-sm:p-2">
          <div className="flex flex-col gap-1">
            <Heading slot="title" className="text-base font-extrabold">
              {t('dateRange.title')}
            </Heading>
            <p id={descriptionId} className="text-sm text-muted">
              {description}
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            {PRESETS.map((id) => (
              <Button key={id} variant="outline" size="sm" onPress={() => pick(presets[id])}>
                {t(`dateRange.${id}`)}
              </Button>
            ))}
          </div>
          <RangeCalendar
            aria-label={t('dateRange.title')}
            autoFocus
            value={{ start: parseDate(value.from), end: parseDate(value.to) }}
            onChange={(range) => pick({ from: range.start.toString(), to: range.end.toString() })}
            className="flex flex-col gap-3"
          >
            <CalendarHeader />
            <CalendarMonthGrid />
          </RangeCalendar>
        </Dialog>
      </Popover>
    </DialogTrigger>
  )
}
