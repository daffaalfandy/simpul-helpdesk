import { Check } from 'lucide-react'
import type { ReactNode } from 'react'
import {
  Checkbox as AriaCheckbox,
  CheckboxGroup as AriaCheckboxGroup,
  Label,
  Text,
  type CheckboxGroupProps as AriaCheckboxGroupProps,
  type CheckboxProps as AriaCheckboxProps,
} from 'react-aria-components'

import { cx } from '../lib/text'

type CheckboxProps = Omit<AriaCheckboxProps, 'children' | 'className'> & {
  children: ReactNode
  description?: ReactNode
  className?: string
}

/** Checkbox per HANDOFF §14.3: the whole row is the click target. */
export function Checkbox({ children, description, className, ...props }: CheckboxProps) {
  return (
    <AriaCheckbox
      {...props}
      className={cx(
        'group flex min-h-(--control-h-sm) cursor-pointer items-start gap-2.5 py-1.5 text-base text-ink',
        'data-disabled:cursor-not-allowed data-disabled:text-muted',
        className,
      )}
    >
      <span
        aria-hidden="true"
        className={cx(
          'mt-0.5 inline-flex size-5 shrink-0 items-center justify-center rounded-xs border-2 border-border-strong bg-surface',
          'group-data-selected:border-primary group-data-selected:bg-primary group-data-disabled:bg-disabled',
          'group-data-focus-visible:shadow-(--focus-ring)',
        )}
      >
        <Check className="hidden size-(--icon-sm) text-on-primary group-data-selected:block" strokeWidth={3} />
      </span>
      <span className="flex flex-col">
        <span className="font-semibold">{children}</span>
        {description && <span className="text-xs text-muted">{description}</span>}
      </span>
    </AriaCheckbox>
  )
}

type CheckboxGroupProps = Omit<AriaCheckboxGroupProps, 'children' | 'className'> & {
  label: ReactNode
  description?: ReactNode
  children: ReactNode
  /** Lay options out in a row that wraps (e.g. weekdays). */
  inline?: boolean
  className?: string
}

export function CheckboxGroup({ label, description, children, inline, className, ...props }: CheckboxGroupProps) {
  return (
    <AriaCheckboxGroup {...props} className={cx('flex flex-col gap-1.5', className)}>
      <Label className="text-sm font-bold text-ink">{label}</Label>
      {description && (
        <Text slot="description" className="text-xs text-muted">
          {description}
        </Text>
      )}
      <div className={inline ? 'flex flex-wrap gap-x-5 gap-y-1' : 'flex flex-col'}>{children}</div>
    </AriaCheckboxGroup>
  )
}
