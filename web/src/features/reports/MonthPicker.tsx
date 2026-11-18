import { useTranslation } from 'react-i18next'

import { Dropdown } from '../../components/Dropdown'
import { reportMonthLabel } from './format'
import { monthOptions, type MonthKey } from './months'

type MonthPickerProps = {
  value: MonthKey
  current: MonthKey
  /** The earliest month the API serves. */
  oldest: MonthKey
  /** The first ticket's month; null without tickets. */
  first: MonthKey | null
  onChange: (month: MonthKey) => void
}

/** The report's month: the current one (to date) and each earlier month with tickets, newest first. */
export function MonthPicker({ value, current, oldest, first, onChange }: MonthPickerProps) {
  const { t } = useTranslation()
  const options = monthOptions(current, oldest, first).map((m) => ({ id: m, label: reportMonthLabel(t, m, m === current) }))
  return (
    <Dropdown
      label={t('reports.month.label')}
      hideLabel
      options={options}
      value={value}
      onChange={onChange}
      className="w-(--month-picker-w)"
    />
  )
}
