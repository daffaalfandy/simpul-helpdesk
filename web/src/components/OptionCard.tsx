import type { ReactNode } from 'react'
import { FieldError, Label, Radio, RadioGroup, Text } from 'react-aria-components'

import { cx } from '../lib/text'

type OptionCardGroupProps<K extends string> = {
  label: ReactNode
  description?: ReactNode
  value: K | null
  onChange: (value: K) => void
  isRequired?: boolean
  isInvalid?: boolean
  errorMessage?: string
  /** "cards" lay out in a wrapping grid; "chips" sit in a row (compact dialogs). */
  layout?: 'cards' | 'chips'
  children: ReactNode
  className?: string
}

/**
 * Radio cards per HANDOFF §14.3 (priority and type pickers). Native radios give
 * role="radio" with the checked state, and arrow keys move between cards.
 */
export function OptionCardGroup<K extends string>({
  label,
  description,
  value,
  onChange,
  isRequired,
  isInvalid,
  errorMessage,
  layout = 'cards',
  children,
  className,
}: OptionCardGroupProps<K>) {
  return (
    <RadioGroup
      value={value}
      onChange={(v) => onChange(v as K)}
      isRequired={isRequired}
      isInvalid={isInvalid}
      className={cx('flex flex-col gap-2', className)}
    >
      <Label className="text-sm font-bold text-ink">{label}</Label>
      <div
        className={
          layout === 'chips'
            ? 'flex flex-wrap gap-2'
            : 'grid grid-cols-[repeat(auto-fit,minmax(var(--option-card-min-w),1fr))] gap-3'
        }
      >
        {children}
      </div>
      {description && (
        <Text slot="description" className="text-sm text-muted">
          {description}
        </Text>
      )}
      <FieldError className="text-xs font-semibold text-danger">{errorMessage}</FieldError>
    </RadioGroup>
  )
}

type OptionCardProps<K extends string> = { value: K; variant?: 'card' | 'chip'; children: ReactNode }

export function OptionCard<K extends string>({ value, variant = 'card', children }: OptionCardProps<K>) {
  return (
    <Radio
      value={value}
      className={({ isSelected, isFocusVisible, isInvalid }) =>
        cx(
          'cursor-pointer border-2 text-ink transition-colors duration-(--duration-fast)',
          variant === 'chip'
            ? 'inline-flex min-h-(--control-h-md) items-center rounded-full px-4 text-sm font-bold'
            : 'flex h-full flex-col gap-2 rounded-xl p-4',
          isSelected ? 'border-primary bg-primary-tint' : 'border-border bg-surface hover:border-border-strong',
          isInvalid && !isSelected && 'border-danger',
          isFocusVisible && 'shadow-(--focus-ring)',
        )
      }
    >
      {children}
    </Radio>
  )
}
