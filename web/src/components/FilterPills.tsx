import { ToggleButton } from 'react-aria-components'

import { cx } from '../lib/text'

export type FilterPillOption<K extends string> = { id: K; label: string; count?: number }

type FilterPillsProps<K extends string> = {
  label: string
  options: ReadonlyArray<FilterPillOption<K>>
  value: K
  onChange: (value: K) => void
  className?: string
}

/**
 * Filter pills per HANDOFF §14.3: toggle buttons with aria-pressed, exactly one on.
 * (A ToggleButtonGroup would turn them into radios.)
 */
export function FilterPills<K extends string>({ label, options, value, onChange, className }: FilterPillsProps<K>) {
  return (
    <div role="group" aria-label={label} className={cx('flex flex-wrap gap-2', className)}>
      {options.map((o) => (
        <ToggleButton
          key={o.id}
          isSelected={value === o.id}
          onChange={() => onChange(o.id)}
          className={({ isSelected }) =>
            cx(
              'inline-flex min-h-(--control-h-md) cursor-pointer items-center gap-2 rounded-full border px-4 text-sm font-bold',
              'transition-colors duration-(--duration-fast)',
              isSelected
                ? 'border-ink bg-ink text-on-primary'
                : 'border-border-strong bg-surface text-ink hover:bg-surface-subtle',
            )
          }
        >
          {o.label}
          {o.count !== undefined && <span className="tabular-nums opacity-80">{o.count}</span>}
        </ToggleButton>
      ))}
    </div>
  )
}
