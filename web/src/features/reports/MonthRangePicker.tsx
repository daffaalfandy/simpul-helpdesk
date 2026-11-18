import { useTranslation } from 'react-i18next'

import { Dropdown } from '../../components/Dropdown'
import { reportMonthLabel } from './format'
import { rangeOptions, type MonthKey, type MonthRange } from './months'

type MonthRangePickerProps = {
  value: MonthRange
  current: MonthKey
  /** The earliest month the API serves. */
  oldest: MonthKey
  /** The first ticket's month; null without tickets. */
  first: MonthKey | null
  onChange: (range: MonthRange) => void
}

/**
 * The report's months (owner, 18 Nov): a "From" and a "To" list, newest first, the current month
 * marked "(to date)". Each list offers only months that keep the range in order, so it can't be
 * turned around and needs no error message.
 */
export function MonthRangePicker({ value, current, oldest, first, onChange }: MonthRangePickerProps) {
  const { t } = useTranslation()
  const months = rangeOptions(current, oldest, first, value)
  const option = (m: MonthKey) => ({ id: m, label: reportMonthLabel(t, m, m === current) })
  return (
    <fieldset className="flex flex-wrap items-end gap-2 max-sm:w-full">
      <legend className="sr-only">{t('reports.range.label')}</legend>
      <Dropdown
        label={t('reports.range.from')}
        options={months.from.map(option)}
        value={value.from}
        onChange={(from) => onChange({ from, to: value.to })}
        className="w-(--month-picker-w) max-sm:w-full"
      />
      <Dropdown
        label={t('reports.range.to')}
        options={months.to.map(option)}
        value={value.to}
        onChange={(to) => onChange({ from: value.from, to })}
        className="w-(--month-picker-w) max-sm:w-full"
      />
    </fieldset>
  )
}
