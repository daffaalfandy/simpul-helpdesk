import { useTranslation } from 'react-i18next'
import { ToggleButton } from 'react-aria-components'

import { Dropdown } from '../../components/Dropdown'
import { cx } from '../../lib/text'
import { monthChoices, monthLong, monthShort, type MonthKey } from './months'

type MonthSwitcherProps = {
  value: MonthKey
  current: MonthKey
  first: MonthKey | null
  onChange: (month: MonthKey) => void
}

/**
 * The mock's month buttons (the latest three, one pressed), plus a list of every older month
 * since the first ticket when there are any.
 */
export function MonthSwitcher({ value, current, first, onChange }: MonthSwitcherProps) {
  const { t } = useTranslation()
  const { recent, earlier } = monthChoices(current, first)
  return (
    <div className="flex flex-wrap items-center gap-2">
      <div role="group" aria-label={t('reports.month.label')} className="flex gap-0.5 rounded-lg border border-border bg-surface p-1">
        {recent.map((m) => (
          <ToggleButton
            key={m}
            isSelected={value === m}
            onChange={() => onChange(m)}
            className={({ isSelected }) =>
              cx(
                'hit-area min-h-(--control-h-sm) cursor-pointer rounded-md px-3.5 text-sm font-bold',
                'transition-colors duration-(--duration-fast) ease-standard',
                isSelected ? 'bg-ink text-on-primary' : 'bg-transparent text-ink-2 hover:bg-surface-subtle',
              )
            }
          >
            {monthShort(m)}
          </ToggleButton>
        ))}
      </div>
      {earlier.length > 0 && (
        <Dropdown
          label={t('reports.month.earlier')}
          hideLabel
          placeholder={t('reports.month.earlier')}
          options={earlier.map((m) => ({ id: m, label: monthLong(m) }))}
          value={earlier.includes(value) ? value : null}
          onChange={onChange}
          className="w-(--month-picker-w)"
        />
      )}
    </div>
  )
}
